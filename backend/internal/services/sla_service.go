package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SLAService struct {
	DB *pgxpool.Pool
}

// slaBreach is one ticket nearing or past its first-response deadline
type slaBreach struct {
	ID          string
	Number      int
	Subject     string
	Customer    string
	Priority    string
	SLADeadline time.Time
	IsBreached  bool
	StoreName   string
	Assignee    string
}

// slackClient bounds the webhook call so a slow Slack cannot stall the checker
var slackClient = &http.Client{Timeout: 10 * time.Second}

// CheckSLABreaches runs every 5 minutes.
// Finds tickets nearing or past SLA deadline and sends alerts.
// Tickets alerted in the last hour are skipped (last_alerted_at),
// so the 5-minute cadence doesn't spam the channel.
func (s *SLAService) CheckSLABreaches(ctx context.Context) error {
	now := time.Now()
	alertWindow := now.Add(15 * time.Minute) // Alert 15 minutes before breach

	// Find tickets nearing SLA or already breached.
	// Slack caps a section at 10 fields, so cap the batch to match.
	rows, err := s.DB.Query(ctx, `
        SELECT
            t.id,
            t.number,
            t.subject,
            t.customer_email,
            t.priority,
            t.sla_first_response_at,
            ten.name as store_name,
            COALESCE(u.first_name || ' ' || u.last_name, 'Unassigned') as assignee_name
        FROM tickets t
        JOIN tenants ten ON ten.id = t.tenant_id
        LEFT JOIN users u ON u.id = t.assignee_id
        WHERE t.status = 'open'
          AND t.first_response_at IS NULL
          AND t.sla_first_response_at IS NOT NULL
          AND t.sla_first_response_at <= $1
          AND (t.last_alerted_at IS NULL OR t.last_alerted_at < NOW() - INTERVAL '1 hour')
        ORDER BY t.sla_first_response_at ASC
        LIMIT 10
    `, alertWindow)
	if err != nil {
		return err
	}
	defer rows.Close()

	var breaches []slaBreach
	for rows.Next() {
		var b slaBreach
		if err := rows.Scan(&b.ID, &b.Number, &b.Subject, &b.Customer,
			&b.Priority, &b.SLADeadline, &b.StoreName, &b.Assignee); err != nil {
			return fmt.Errorf("failed to scan SLA row: %w", err)
		}
		b.IsBreached = b.SLADeadline.Before(now)
		breaches = append(breaches, b)
	}

	if len(breaches) == 0 {
		return nil
	}

	// Mark as alerted up front — a Slack failure will retry within the hour anyway
	ids := make([]string, len(breaches))
	for i, b := range breaches {
		ids[i] = b.ID
	}
	if _, err := s.DB.Exec(ctx,
		"UPDATE tickets SET last_alerted_at = NOW() WHERE id = ANY($1)", ids,
	); err != nil {
		return fmt.Errorf("failed to record alert time: %w", err)
	}

	// Build Slack alert message
	slackURL := os.Getenv("SLACK_WEBHOOK_URL")
	if slackURL == "" {
		// No Slack configured — just log
		for _, b := range breaches {
			fmt.Printf("SLA %s: Ticket #%d (%s) - %s\n",
				map[bool]string{true: "BREACH", false: "AT RISK"}[b.IsBreached],
				b.Number, b.Priority, b.Subject,
			)
		}
		return nil
	}

	// Format breaches for Slack
	var fields []map[string]string
	for _, b := range breaches {
		emoji := "⚠️"
		status := "At risk"
		if b.IsBreached {
			emoji = "🚨"
			status = "BREACHED"
		}

		fields = append(fields, map[string]string{
			"type": "mrkdwn",
			"text": fmt.Sprintf("%s *#%d* (%s) — %s\n_%s_ → %s", emoji, b.Number, status, b.Subject, b.Customer, b.Assignee),
		})
	}

	slackPayload := map[string]interface{}{
		"blocks": []map[string]interface{}{
			{
				"type": "section",
				"text": map[string]string{
					"type": "mrkdwn",
					"text": fmt.Sprintf("*SLA Alert* — %d ticket(s) need attention", len(breaches)),
				},
			},
			{
				"type":   "section",
				"fields": fields,
			},
		},
	}

	payloadBytes, err := json.Marshal(slackPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal slack payload: %w", err)
	}

	resp, err := slackClient.Post(slackURL, "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("slack webhook failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack webhook returned %d", resp.StatusCode)
	}

	return nil
}
