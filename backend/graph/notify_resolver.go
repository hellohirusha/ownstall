package graph

import (
	"context"
	"fmt"

	"github.com/hellohirusha/ownstall/graph/model"
)

// campaignColumns is the shared SELECT list for email campaigns.
// delivered/opened/clicked are live aggregates from email_logs (the
// tracking endpoints update logs, not the campaign counters), while
// sent_count comes from the campaign row (the worker increments it).
const campaignColumns = `
    c.id, c.name, c.subject, c.template_id, c.status, c.recipient_type,
    c.recipient_count, c.sent_count,
    (SELECT COUNT(*) FROM email_logs l
     WHERE l.campaign_id = c.id AND l.status IN ('delivered','opened','clicked')),
    (SELECT COUNT(*) FROM email_logs l
     WHERE l.campaign_id = c.id AND l.status IN ('opened','clicked')),
    (SELECT COUNT(*) FROM email_logs l
     WHERE l.campaign_id = c.id AND l.status = 'clicked'),
    c.scheduled_at, c.started_at, c.completed_at, c.created_at`

// scanCampaign maps one campaign row (selected with campaignColumns)
// onto the generated GraphQL type.
func scanCampaign(row interface{ Scan(dest ...any) error }) (*model.EmailCampaign, error) {
	var c model.EmailCampaign
	var id string
	var templateID *string
	if err := row.Scan(
		&id, &c.Name, &c.Subject, &templateID, &c.Status, &c.RecipientType,
		&c.RecipientCount, &c.SentCount,
		&c.DeliveredCount, &c.OpenedCount, &c.ClickedCount,
		&c.ScheduledAt, &c.StartedAt, &c.CompletedAt, &c.CreatedAt,
	); err != nil {
		return nil, err
	}
	c.ID = parseUUID(id)
	if templateID != nil {
		tid := parseUUID(*templateID)
		c.TemplateID = &tid
	}
	return &c, nil
}

// fetchCampaign loads a single tenant-owned campaign by id.
func (r *Resolver) fetchCampaign(ctx context.Context, tenantID, id string) (*model.EmailCampaign, error) {
	row := r.DB.QueryRow(ctx, `
        SELECT `+campaignColumns+`
        FROM email_campaigns c
        WHERE c.id = $1 AND c.tenant_id = $2
    `, id, tenantID)

	campaign, err := scanCampaign(row)
	if err != nil {
		return nil, fmt.Errorf("campaign not found: %w", err)
	}
	return campaign, nil
}
