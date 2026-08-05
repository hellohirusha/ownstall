package services

import (
	"context"
	"fmt"
)

// CopyGenStats summarises generated product copy for one tenant.
type CopyGenStats struct {
	Generated       int
	AutoPublished   int
	AvgQualityScore float64
}

// Stats reports how much copy has been generated and how much of it
// was good enough to publish unreviewed.
func (s *CopyGeneratorService) Stats(ctx context.Context, tenantID string) (*CopyGenStats, error) {
	var stats CopyGenStats
	err := s.DB.QueryRow(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE ai_description IS NOT NULL),
            COUNT(*) FILTER (WHERE ai_quality_score >= $2),
            COALESCE(AVG(ai_quality_score) FILTER (WHERE ai_quality_score IS NOT NULL), 0)
        FROM products
        WHERE tenant_id = $1
    `, tenantID, autoPublishThreshold).Scan(
		&stats.Generated, &stats.AutoPublished, &stats.AvgQualityScore,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load copy generation stats: %w", err)
	}
	return &stats, nil
}

// AutoReplyStats summarises AI drafting and auto-sending for one tenant.
type AutoReplyStats struct {
	Drafted             int
	AutoSent            int
	AvgConfidence       float64
	TicketsMissingDraft int
	// DeflectionRate is the share of drafted tickets that were closed
	// by the AI reply — the number that says whether this feature is
	// actually removing support work.
	DeflectionRate float64
}

// Stats reports drafting volume, auto-send volume and deflection.
func (s *AutoReplyService) Stats(ctx context.Context, tenantID string) (*AutoReplyStats, error) {
	var stats AutoReplyStats

	err := s.DB.QueryRow(ctx, `
        SELECT
            COUNT(*) FILTER (WHERE ai_draft_body IS NOT NULL),
            COALESCE(AVG(ai_draft_confidence) FILTER (WHERE ai_draft_confidence IS NOT NULL), 0),
            COUNT(*) FILTER (WHERE status = 'open' AND ai_draft_body IS NULL)
        FROM tickets
        WHERE tenant_id = $1
    `, tenantID).Scan(&stats.Drafted, &stats.AvgConfidence, &stats.TicketsMissingDraft)
	if err != nil {
		return nil, fmt.Errorf("failed to load auto-reply stats: %w", err)
	}

	// Auto-sent replies are the messages authored by the assistant
	err = s.DB.QueryRow(ctx, `
        SELECT COUNT(DISTINCT tm.ticket_id)
        FROM ticket_messages tm
        JOIN tickets t ON t.id = tm.ticket_id
        WHERE t.tenant_id = $1 AND tm.author_type = 'ai'
    `, tenantID).Scan(&stats.AutoSent)
	if err != nil {
		return nil, fmt.Errorf("failed to load auto-sent stats: %w", err)
	}

	// Deflected: an AI reply landed and no human ever replied after it
	var deflected int
	err = s.DB.QueryRow(ctx, `
        SELECT COUNT(DISTINCT t.id)
        FROM tickets t
        JOIN ticket_messages ai_msg
          ON ai_msg.ticket_id = t.id AND ai_msg.author_type = 'ai'
        WHERE t.tenant_id = $1
          AND t.status IN ('resolved', 'closed')
          AND NOT EXISTS (
              SELECT 1 FROM ticket_messages staff
              WHERE staff.ticket_id = t.id
                AND staff.author_type = 'staff'
                AND staff.is_internal = false
          )
    `, tenantID).Scan(&deflected)
	if err != nil {
		return nil, fmt.Errorf("failed to load deflection stats: %w", err)
	}

	if stats.Drafted > 0 {
		stats.DeflectionRate = float64(deflected) / float64(stats.Drafted)
	}

	return &stats, nil
}

// RecommendationStats summarises embedding coverage for one tenant.
type RecommendationStats struct {
	Indexed int
	Missing int
}

// Stats reports how much of the active catalogue has a vector.
func (s *RecommendationService) Stats(ctx context.Context, tenantID string) (*RecommendationStats, error) {
	var stats RecommendationStats
	err := s.DB.QueryRow(ctx, `
        SELECT
            COUNT(pe.id),
            COUNT(*) FILTER (WHERE pe.id IS NULL)
        FROM products p
        LEFT JOIN product_embeddings pe ON pe.product_id = p.id
        WHERE p.tenant_id = $1 AND p.status = 'active'
    `, tenantID).Scan(&stats.Indexed, &stats.Missing)
	if err != nil {
		return nil, fmt.Errorf("failed to load recommendation stats: %w", err)
	}
	return &stats, nil
}
