package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/pkg/ai"
)

// AutoReplyService drafts support replies with an LLM and, when the
// model is confident enough, sends them without human review.
type AutoReplyService struct {
	DB            *pgxpool.Pool
	AI            *ai.Client
	TicketService *TicketService
}

// AutoReplyConfig controls how aggressively drafting and auto-sending run.
type AutoReplyConfig struct {
	// AutoSendAboveThreshold sends a draft unreviewed at or above this
	// confidence. Zero disables auto-sending entirely (draft only).
	AutoSendAboveThreshold float64
	// MaxDraftsPerDay caps spend and blast radius if the model starts
	// producing nonsense.
	MaxDraftsPerDay int
	// BatchSize is how many tickets one scheduler pass will draft.
	BatchSize int
}

// DefaultAutoReplyConfig is deliberately conservative: 90% confidence
// before anything reaches a customer unreviewed.
var DefaultAutoReplyConfig = AutoReplyConfig{
	AutoSendAboveThreshold: 0.90,
	MaxDraftsPerDay:        200,
	BatchSize:              5,
}

// GenerateDraft writes an AI draft onto a ticket and, if it clears the
// auto-send threshold and needs no escalation, posts it and resolves
// the ticket.
func (s *AutoReplyService) GenerateDraft(
	ctx context.Context, tenantID, ticketID string, config AutoReplyConfig,
) error {
	if !s.AI.Enabled() {
		return ai.ErrDisabled
	}

	ticket, err := s.TicketService.GetTicketWithMessages(ctx, tenantID, ticketID)
	if err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}
	if len(ticket.Messages) == 0 {
		return fmt.Errorf("ticket %s has no messages to reply to", ticketID)
	}

	systemPrompt, err := s.buildSystemPrompt(ctx, tenantID)
	if err != nil {
		return err
	}

	var conversation strings.Builder
	for _, msg := range ticket.Messages {
		if msg.IsInternal {
			continue // internal notes are staff-only and must not leak into a reply
		}
		speaker := "Customer"
		if msg.AuthorType != "customer" {
			speaker = "Support"
		}
		fmt.Fprintf(&conversation, "%s: %s\n\n", speaker, msg.Body)
	}

	userPrompt := fmt.Sprintf(`Customer email: %s
Subject: %s

Conversation so far:
%s
Write a reply to the customer's most recent message, then rate your own
confidence that it is accurate and complete.

Set should_escalate to true if the reply needs order-specific data, a
refund decision, or anything the knowledge base does not cover.

Respond with JSON only:
{"reply": "...", "confidence": 0.87, "reasoning": "brief", "should_escalate": false}`,
		ticket.CustomerEmail, ticket.Subject, conversation.String())

	response, err := s.AI.CompleteJSON(ctx, ai.Request{
		Feature:     "auto_reply",
		TenantID:    tenantID,
		ReferenceID: ticketID,
		System:      systemPrompt,
		User:        userPrompt,
		Temperature: 0.3,
		MaxTokens:   600,
	})
	if err != nil {
		return fmt.Errorf("draft generation failed: %w", err)
	}

	var draft struct {
		Reply          string  `json:"reply"`
		Confidence     float64 `json:"confidence"`
		Reasoning      string  `json:"reasoning"`
		ShouldEscalate bool    `json:"should_escalate"`
	}
	if err := json.Unmarshal([]byte(response), &draft); err != nil {
		return fmt.Errorf("failed to parse draft: %w", err)
	}
	if strings.TrimSpace(draft.Reply) == "" {
		return fmt.Errorf("draft for ticket %s was empty", ticketID)
	}

	draft.Confidence = clamp01(draft.Confidence)

	if _, err := s.DB.Exec(ctx, `
        UPDATE tickets
        SET ai_draft_body = $1, ai_draft_confidence = $2, ai_draft_generated_at = NOW()
        WHERE id = $3 AND tenant_id = $4
    `, draft.Reply, draft.Confidence, ticketID, tenantID); err != nil {
		return fmt.Errorf("failed to store draft: %w", err)
	}

	fmt.Printf("auto_reply: drafted ticket %s (confidence %.2f, escalate=%v)\n",
		ticketID, draft.Confidence, draft.ShouldEscalate)

	shouldSend := !draft.ShouldEscalate &&
		config.AutoSendAboveThreshold > 0 &&
		draft.Confidence >= config.AutoSendAboveThreshold

	if !shouldSend {
		return nil
	}

	if err := s.sendAIReply(ctx, tenantID, ticketID, draft.Reply, draft.Confidence); err != nil {
		return fmt.Errorf("auto-send failed: %w", err)
	}

	// High confidence and no escalation means the question is answered;
	// a customer reply reopens the ticket through the inbound handler.
	if err := s.TicketService.UpdateTicketStatus(ctx, tenantID, ticketID, "", "resolved"); err != nil {
		fmt.Printf("auto_reply: failed to resolve ticket %s: %v\n", ticketID, err)
	}

	fmt.Printf("auto_reply: auto-sent reply on ticket %s (confidence %.2f)\n",
		ticketID, draft.Confidence)

	return nil
}

// buildSystemPrompt grounds the model in the store's own canned
// responses, so answers come from the tenant's stated policies rather
// than the model's assumptions about how a shop works.
func (s *AutoReplyService) buildSystemPrompt(ctx context.Context, tenantID string) (string, error) {
	var storeName string
	if err := s.DB.QueryRow(ctx,
		"SELECT name FROM tenants WHERE id = $1", tenantID,
	).Scan(&storeName); err != nil {
		return "", fmt.Errorf("tenant %s not found: %w", tenantID, err)
	}

	rows, err := s.DB.Query(ctx, `
        SELECT name, body FROM canned_responses
        WHERE tenant_id = $1
        ORDER BY created_at LIMIT 20
    `, tenantID)
	if err != nil {
		return "", fmt.Errorf("failed to load canned responses: %w", err)
	}
	defer rows.Close()

	var knowledge strings.Builder
	for rows.Next() {
		var name, body string
		if err := rows.Scan(&name, &body); err != nil {
			continue
		}
		fmt.Fprintf(&knowledge, "Topic: %s\nAnswer: %s\n\n", name, body)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("failed to read canned responses: %w", err)
	}

	kb := knowledge.String()
	if kb == "" {
		kb = "(no saved answers yet — rely only on what the customer has told you)"
	}

	return fmt.Sprintf(`You are a customer support agent for %s.
Be friendly, professional and concise.

KNOWLEDGE BASE — the only store policies you may state as fact:
%s

Rules:
- Answer the customer's question directly.
- Never invent order details, delivery dates, prices, or policies.
- If the answer is not in the knowledge base or the conversation, say so
  plainly and set should_escalate to true.
- Keep replies under 150 words.
- Close with an offer to help further.`, storeName, kb), nil
}

// sendAIReply posts the reply into the ticket thread.
//
// It writes the message directly rather than calling ReplyToTicket:
// that path resolves a staff author from the users table, and an
// automated reply has no user behind it. author_type 'ai' keeps the
// provenance visible in the thread and in any later audit.
func (s *AutoReplyService) sendAIReply(
	ctx context.Context, tenantID, ticketID, body string, confidence float64,
) error {
	if err := s.TicketService.setTenantContext(ctx, tenantID); err != nil {
		return fmt.Errorf("failed to set tenant context: %w", err)
	}

	// Disclosed to the customer: an unmarked machine reply is the kind
	// of thing that costs more trust than it saves time.
	signed := fmt.Sprintf("%s\n\n— Sent automatically by our AI assistant (%.0f%% confidence). Reply to this message to reach a human.",
		strings.TrimSpace(body), confidence*100)

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	// Rollback is a no-op after a successful commit
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `
        INSERT INTO ticket_messages
            (ticket_id, author_type, author_name, body, is_internal)
        VALUES ($1, 'ai', 'AI Assistant', $2, false)
    `, ticketID, signed); err != nil {
		return fmt.Errorf("failed to insert AI reply: %w", err)
	}

	// An auto-sent reply is a first response for SLA purposes
	if _, err := tx.Exec(ctx, `
        UPDATE tickets
        SET first_response_at = NOW()
        WHERE id = $1 AND first_response_at IS NULL
    `, ticketID); err != nil {
		return fmt.Errorf("failed to record first response: %w", err)
	}

	// user_id stays NULL — no human performed this action
	if _, err := tx.Exec(ctx, `
        INSERT INTO ticket_events (ticket_id, event_type, metadata)
        VALUES ($1, 'replied', $2)
    `, ticketID, fmt.Sprintf(`{"source":"ai","confidence":%.2f}`, confidence)); err != nil {
		return fmt.Errorf("failed to record reply event: %w", err)
	}

	return tx.Commit(ctx)
}

// ProcessNewTickets drafts replies for recent open tickets that do not
// have one yet, and reports how many were drafted.
func (s *AutoReplyService) ProcessNewTickets(ctx context.Context, config AutoReplyConfig) (int, error) {
	if !s.AI.Enabled() {
		return 0, nil
	}

	if config.MaxDraftsPerDay > 0 {
		var today int
		if err := s.DB.QueryRow(ctx, `
            SELECT COUNT(*) FROM ai_logs
            WHERE feature = 'auto_reply' AND created_at >= date_trunc('day', NOW())
        `).Scan(&today); err != nil {
			return 0, fmt.Errorf("failed to check daily draft count: %w", err)
		}
		if today >= config.MaxDraftsPerDay {
			return 0, nil
		}
	}

	batch := config.BatchSize
	if batch <= 0 {
		batch = DefaultAutoReplyConfig.BatchSize
	}

	// Only recent tickets: drafting a reply to a day-old conversation
	// that a human has already handled is worse than useless.
	rows, err := s.DB.Query(ctx, `
        SELECT id, tenant_id FROM tickets
        WHERE status = 'open'
          AND ai_draft_body IS NULL
          AND created_at > NOW() - INTERVAL '1 hour'
        ORDER BY created_at ASC
        LIMIT $1
    `, batch)
	if err != nil {
		return 0, fmt.Errorf("failed to list tickets needing drafts: %w", err)
	}

	type target struct{ ticketID, tenantID string }
	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.ticketID, &t.tenantID); err != nil {
			rows.Close()
			return 0, fmt.Errorf("failed to scan ticket: %w", err)
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("failed to read tickets: %w", err)
	}

	// Sequential, not one goroutine per ticket: the free provider tier
	// allows ~30 requests/minute, and a burst here would 429 the copy
	// generator running alongside it.
	drafted := 0
	for _, t := range targets {
		if err := s.GenerateDraft(ctx, t.tenantID, t.ticketID, config); err != nil {
			fmt.Printf("auto_reply: draft failed for ticket %s: %v\n", t.ticketID, err)
			continue
		}
		drafted++
	}

	return drafted, nil
}

// CountTicketsMissingDraft reports how many open tickets have no AI
// draft — shown on the AI dashboard.
func (s *AutoReplyService) CountTicketsMissingDraft(ctx context.Context, tenantID string) (int, error) {
	var count int
	err := s.DB.QueryRow(ctx, `
        SELECT COUNT(*) FROM tickets
        WHERE tenant_id = $1 AND status = 'open' AND ai_draft_body IS NULL
    `, tenantID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count tickets missing drafts: %w", err)
	}
	return count, nil
}
