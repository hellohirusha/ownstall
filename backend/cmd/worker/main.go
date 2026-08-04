package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/hellohirusha/ownstall/pkg/database"
	"github.com/hellohirusha/ownstall/pkg/email"
	"github.com/hellohirusha/ownstall/pkg/queue"
)

func main() {
	if os.Getenv("ENVIRONMENT") != "production" {
		if err := godotenv.Load(); err != nil {
			log.Println("No .env file found — using system environment variables")
		}
	}

	// Connect to database
	db, err := database.Connect(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("DB connect failed: %v", err)
	}
	defer db.Close()

	// Connect to Redis queue
	queueClient, err := queue.NewClient()
	if err != nil {
		log.Fatalf("Redis connect failed: %v", err)
	}

	// Initialize email client
	emailClient := email.NewResendClient()

	ctx, cancel := context.WithCancel(context.Background())

	// Graceful shutdown
	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Worker shutting down...")
		cancel()
	}()

	log.Println("Email worker started")

	// Process email jobs from the queue
	queueClient.Subscribe(ctx, queue.QueueEmail, func(ctx context.Context, job *queue.Job) error {
		var payload queue.EmailJobPayload
		if err := json.Unmarshal(job.Payload, &payload); err != nil {
			return fmt.Errorf("invalid job payload: %w", err)
		}

		// 1. Check suppression list — never send to suppressed addresses
		var suppressedCount int
		if err := db.QueryRow(ctx, `
            SELECT COUNT(*) FROM email_suppressions
            WHERE tenant_id = $1 AND email = $2
        `, payload.TenantID, payload.ToEmail).Scan(&suppressedCount); err != nil {
			return fmt.Errorf("suppression check failed: %w", err)
		}

		if suppressedCount > 0 {
			// Update log to show suppressed
			if payload.LogID != "" {
				if _, err := db.Exec(ctx,
					"UPDATE email_logs SET status = 'failed', error_message = 'suppressed' WHERE id = $1",
					payload.LogID,
				); err != nil {
					log.Printf("failed to mark log %s suppressed: %v", payload.LogID, err)
				}
			}
			log.Printf("Suppressed email to %s", payload.ToEmail)
			return nil // Not a failure — intentional skip
		}

		// 2. Load template from database
		var subject, htmlBody string
		err := db.QueryRow(ctx, `
            SELECT subject, html_body FROM email_templates
            WHERE id = $1
        `, payload.TemplateID).Scan(&subject, &htmlBody)
		if err != nil {
			return fmt.Errorf("template %s not found: %w", payload.TemplateID, err)
		}

		// 3. Render template with variables
		renderedSubject, err := email.RenderTemplate(subject, payload.Variables)
		if err != nil {
			return fmt.Errorf("subject render error: %w", err)
		}

		renderedBody, err := email.RenderTemplate(htmlBody, payload.Variables)
		if err != nil {
			return fmt.Errorf("body render error: %w", err)
		}

		// 4. Add tracking pixel to HTML body (for open tracking)
		if payload.LogID != "" {
			trackingPixel := fmt.Sprintf(
				`<img src="%s/webhooks/email/open?log_id=%s" width="1" height="1" style="display:none" />`,
				os.Getenv("API_URL"), payload.LogID,
			)
			renderedBody += trackingPixel
		}

		// 5. Send via Resend
		to := payload.ToEmail
		if payload.ToName != "" {
			to = fmt.Sprintf("%s <%s>", payload.ToName, payload.ToEmail)
		}
		result, err := emailClient.Send(ctx, email.SendEmailRequest{
			To:      []string{to},
			Subject: renderedSubject,
			HTML:    renderedBody,
			Tags: []email.Tag{
				{Name: "tenant_id", Value: payload.TenantID},
				{Name: "template_id", Value: payload.TemplateID},
			},
		})
		if err != nil {
			// Update log with failure
			if payload.LogID != "" {
				if _, execErr := db.Exec(ctx, `
                    UPDATE email_logs
                    SET status = 'failed', error_message = $1
                    WHERE id = $2
                `, err.Error(), payload.LogID); execErr != nil {
					log.Printf("failed to mark log %s failed: %v", payload.LogID, execErr)
				}
			}
			return fmt.Errorf("resend send failed: %w", err)
		}

		// 6. Update log with Resend message ID and sent status
		if payload.LogID != "" {
			if _, err := db.Exec(ctx, `
                UPDATE email_logs
                SET status = 'sent',
                    resend_message_id = $1,
                    sent_at = NOW()
                WHERE id = $2
            `, result.ID, payload.LogID); err != nil {
				log.Printf("email sent but failed to update log %s: %v", payload.LogID, err)
			}
		}

		// 7. Update campaign sent count if applicable
		if payload.CampaignID != "" {
			if _, err := db.Exec(ctx, `
                UPDATE email_campaigns
                SET sent_count = sent_count + 1
                WHERE id = $1
            `, payload.CampaignID); err != nil {
				log.Printf("failed to update campaign %s sent count: %v", payload.CampaignID, err)
			}
		}

		log.Printf("Email sent to %s (resend id: %s)", payload.ToEmail, result.ID)
		return nil
	})
}
