package services

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/creator-os/pkg/queue"
)

// CampaignService dispatches scheduled email campaigns.
// Recipients come from the tenant's users table (staff accounts) —
// fine for the demo; a real deployment would target customers.
type CampaignService struct {
	DB    *pgxpool.Pool
	Queue *queue.Client
}

// ProcessDueCampaigns finds campaigns whose scheduled_at has passed and
// dispatches them. Called by the scheduler ticker in main.go.
func (s *CampaignService) ProcessDueCampaigns(ctx context.Context) error {
	if s.Queue == nil {
		return nil // Redis was down at boot — campaigns stay scheduled
	}

	rows, err := s.DB.Query(ctx, `
        SELECT id, tenant_id, template_id, name
        FROM email_campaigns
        WHERE status = 'scheduled'
          AND (scheduled_at IS NULL OR scheduled_at <= NOW())
    `)
	if err != nil {
		return fmt.Errorf("failed to query due campaigns: %w", err)
	}
	defer rows.Close()

	type dueCampaign struct {
		ID         string
		TenantID   string
		TemplateID *string
		Name       string
	}
	var due []dueCampaign
	for rows.Next() {
		var c dueCampaign
		if err := rows.Scan(&c.ID, &c.TenantID, &c.TemplateID, &c.Name); err != nil {
			return fmt.Errorf("failed to scan campaign row: %w", err)
		}
		due = append(due, c)
	}
	rows.Close()

	for _, c := range due {
		if err := s.dispatchCampaign(ctx, c.ID, c.TenantID, c.TemplateID); err != nil {
			fmt.Printf("ERROR: campaign %s (%s) dispatch failed: %v\n", c.ID, c.Name, err)
			if _, uerr := s.DB.Exec(ctx,
				"UPDATE email_campaigns SET status = 'failed' WHERE id = $1", c.ID,
			); uerr != nil {
				fmt.Printf("ERROR: failed to mark campaign %s failed: %v\n", c.ID, uerr)
			}
		}
	}

	return nil
}

// dispatchCampaign enqueues one email job per recipient and finalizes the
// campaign row. The worker increments sent_count as each email goes out.
func (s *CampaignService) dispatchCampaign(ctx context.Context, campaignID, tenantID string, templateID *string) error {
	if templateID == nil {
		return fmt.Errorf("campaign has no template")
	}

	// Claim the campaign so a second scheduler tick can't double-send it
	tag, err := s.DB.Exec(ctx, `
        UPDATE email_campaigns
        SET status = 'sending', started_at = NOW()
        WHERE id = $1 AND status = 'scheduled'
    `, campaignID)
	if err != nil {
		return fmt.Errorf("failed to claim campaign: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil // already claimed elsewhere
	}

	// Store name for template branding
	var storeName string
	_ = s.DB.QueryRow(ctx, "SELECT name FROM tenants WHERE id = $1", tenantID).Scan(&storeName)

	// Recipients: the tenant's active users
	rows, err := s.DB.Query(ctx, `
        SELECT email, COALESCE(first_name, '')
        FROM users
        WHERE tenant_id = $1 AND is_active = true
    `, tenantID)
	if err != nil {
		return fmt.Errorf("failed to load recipients: %w", err)
	}
	defer rows.Close()

	type recipient struct {
		Email string
		Name  string
	}
	var recipients []recipient
	for rows.Next() {
		var rec recipient
		if err := rows.Scan(&rec.Email, &rec.Name); err != nil {
			return fmt.Errorf("failed to scan recipient: %w", err)
		}
		recipients = append(recipients, rec)
	}
	rows.Close()

	// Campaign subject snapshot for the log rows
	var subject string
	_ = s.DB.QueryRow(ctx,
		"SELECT subject FROM email_campaigns WHERE id = $1", campaignID,
	).Scan(&subject)

	queued := 0
	for _, rec := range recipients {
		// One log row per recipient, linked to the campaign
		var logID string
		if err := s.DB.QueryRow(ctx, `
            INSERT INTO email_logs
                (tenant_id, campaign_id, template_id, to_email, to_name, subject, status)
            VALUES ($1, $2, $3, $4, $5, $6, 'queued')
            RETURNING id
        `, tenantID, campaignID, *templateID, rec.Email, rec.Name, subject).Scan(&logID); err != nil {
			fmt.Printf("WARNING: failed to create campaign log for %s: %v\n", rec.Email, err)
		}

		if err := s.Queue.Publish(ctx, queue.QueueEmail, queue.EmailJobPayload{
			TenantID:   tenantID,
			TemplateID: *templateID,
			ToEmail:    rec.Email,
			ToName:     rec.Name,
			CampaignID: campaignID,
			LogID:      logID,
			Variables: map[string]string{
				"CustomerName": rec.Name,
				"StoreName":    storeName,
			},
		}); err != nil {
			fmt.Printf("ERROR: failed to queue campaign email to %s: %v\n", rec.Email, err)
			continue
		}
		queued++
	}

	// Finalize: sent from the scheduler's perspective (worker delivers)
	if _, err := s.DB.Exec(ctx, `
        UPDATE email_campaigns
        SET status = 'sent', recipient_count = $2, completed_at = NOW()
        WHERE id = $1
    `, campaignID, queued); err != nil {
		return fmt.Errorf("failed to finalize campaign: %w", err)
	}

	fmt.Printf("Campaign %s dispatched: %d email(s) queued\n", campaignID, queued)
	return nil
}
