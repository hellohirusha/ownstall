package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type TicketService struct {
	DB *pgxpool.Pool
}

// setTenantContext sets the RLS context variable so Postgres
// enforces tenant isolation on every query
func (s *TicketService) setTenantContext(ctx context.Context, tenantID string) error {
	_, err := s.DB.Exec(ctx,
		"SELECT set_config('app.current_tenant_id', $1, true)",
		tenantID,
	)
	return err
}

// ListTickets returns tickets for a tenant, filtered by status and priority.
// Ordered by: urgent first, then by SLA breach risk, then by creation time.
func (s *TicketService) ListTickets(ctx context.Context, tenantID string, opts ListTicketsOptions) ([]*Ticket, error) {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	query := `
        SELECT
            t.id,
            t.number,
            t.subject,
            t.status,
            t.priority,
            t.customer_email,
            t.customer_name,
            t.assignee_id,
            t.source,
            t.first_response_at,
            t.sla_first_response_at,
            t.resolved_at,
            t.ai_draft_confidence,
            t.created_at,
            t.updated_at,
            -- Latest message preview (for inbox list)
            (SELECT SUBSTRING(body, 1, 120)
             FROM ticket_messages tm
             WHERE tm.ticket_id = t.id AND tm.is_internal = false
             ORDER BY tm.created_at DESC LIMIT 1) as latest_message,
            -- Unread count (messages from customer after last staff reply)
            (SELECT COUNT(*)
             FROM ticket_messages tm
             WHERE tm.ticket_id = t.id
               AND tm.author_type = 'customer'
               AND tm.created_at > COALESCE(
                   (SELECT MAX(created_at) FROM ticket_messages
                    WHERE ticket_id = t.id AND author_type = 'staff'),
                   '1970-01-01'
               )
            ) as unread_count,
            -- Assignee name
            COALESCE(u.first_name || ' ' || u.last_name, '') as assignee_name
        FROM tickets t
        LEFT JOIN users u ON u.id = t.assignee_id
        WHERE t.tenant_id = $1
    `

	args := []interface{}{tenantID}
	argNum := 2

	if opts.Status != "" {
		query += fmt.Sprintf(" AND t.status = $%d", argNum)
		args = append(args, opts.Status)
		argNum++
	}

	if opts.Priority != "" {
		query += fmt.Sprintf(" AND t.priority = $%d", argNum)
		args = append(args, opts.Priority)
		argNum++
	}

	if opts.AssigneeID != "" {
		query += fmt.Sprintf(" AND t.assignee_id = $%d", argNum)
		args = append(args, opts.AssigneeID)
		argNum++
	}

	if opts.Search != "" {
		query += fmt.Sprintf(" AND (t.subject ILIKE $%d OR t.customer_email ILIKE $%d)", argNum, argNum)
		args = append(args, "%"+opts.Search+"%")
		argNum++
	}

	// Sort: urgent/high priority first, then SLA at risk, then newest
	query += `
        ORDER BY
            CASE t.priority
                WHEN 'urgent' THEN 1
                WHEN 'high'   THEN 2
                WHEN 'normal' THEN 3
                WHEN 'low'    THEN 4
            END,
            CASE WHEN t.sla_first_response_at < NOW() THEN 0 ELSE 1 END,
            t.created_at DESC
    `

	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argNum)
		args = append(args, opts.Limit)
	}

	rows, err := s.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tickets: %w", err)
	}
	defer rows.Close()

	var tickets []*Ticket
	for rows.Next() {
		var t Ticket
		var latestMessage *string
		var unreadCount int
		var assigneeName string

		err := rows.Scan(
			&t.ID, &t.Number, &t.Subject, &t.Status, &t.Priority,
			&t.CustomerEmail, &t.CustomerName, &t.AssigneeID, &t.Source,
			&t.FirstResponseAt, &t.SLAFirstResponseAt, &t.ResolvedAt,
			&t.AIDraftConfidence,
			&t.CreatedAt, &t.UpdatedAt,
			&latestMessage, &unreadCount, &assigneeName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ticket: %w", err)
		}

		if latestMessage != nil {
			t.LatestMessage = *latestMessage
		}
		t.UnreadCount = unreadCount
		t.AssigneeName = assigneeName

		// Calculate SLA status
		t.SLAStatus = calculateSLAStatus(t.SLAFirstResponseAt, t.FirstResponseAt)

		tickets = append(tickets, &t)
	}

	return tickets, nil
}

// GetTicketWithMessages returns a ticket and all its messages
func (s *TicketService) GetTicketWithMessages(ctx context.Context, tenantID, ticketID string) (*Ticket, error) {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	var t Ticket
	err := s.DB.QueryRow(ctx, `
        SELECT id, number, subject, status, priority, customer_email, customer_name,
               assignee_id, source, first_response_at, sla_first_response_at,
               resolved_at, ai_draft_body, ai_draft_confidence,
               created_at, updated_at
        FROM tickets
        WHERE id = $1 AND tenant_id = $2
    `, ticketID, tenantID).Scan(
		&t.ID, &t.Number, &t.Subject, &t.Status, &t.Priority,
		&t.CustomerEmail, &t.CustomerName, &t.AssigneeID, &t.Source,
		&t.FirstResponseAt, &t.SLAFirstResponseAt, &t.ResolvedAt,
		&t.AIDraftBody, &t.AIDraftConfidence,
		&t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("ticket not found: %w", err)
	}

	// Load messages
	msgRows, err := s.DB.Query(ctx, `
        SELECT id, author_type, author_id, author_email, author_name,
               body, is_internal, created_at
        FROM ticket_messages
        WHERE ticket_id = $1
        ORDER BY created_at ASC
    `, ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to query ticket messages: %w", err)
	}
	defer msgRows.Close()

	for msgRows.Next() {
		var m Message
		if err := msgRows.Scan(
			&m.ID, &m.AuthorType, &m.AuthorID, &m.AuthorEmail, &m.AuthorName,
			&m.Body, &m.IsInternal, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan ticket message: %w", err)
		}
		t.Messages = append(t.Messages, m)
	}

	t.SLAStatus = calculateSLAStatus(t.SLAFirstResponseAt, t.FirstResponseAt)
	return &t, nil
}

// CreateTicket creates a new ticket from a web form or email
type CreateTicketInput struct {
	TenantID      string
	Subject       string
	Body          string
	CustomerEmail string
	CustomerName  string
	Source        string
	OrderID       string
	Priority      string
}

func (s *TicketService) CreateTicket(ctx context.Context, input CreateTicketInput) (*Ticket, error) {
	if input.Priority == "" {
		input.Priority = "normal"
	}

	// Calculate SLA deadlines (1hr for first response, 24hr for resolution)
	slaFirstResponse := time.Now().Add(1 * time.Hour)
	slaResolution := time.Now().Add(24 * time.Hour)

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// Rollback is a no-op after a successful commit
	defer func() { _ = tx.Rollback(ctx) }()

	// Create ticket
	var ticketID string
	err = tx.QueryRow(ctx, `
        INSERT INTO tickets
            (tenant_id, subject, status, priority, customer_email, customer_name,
             source, order_id, sla_first_response_at, sla_resolution_at)
        VALUES ($1,$2,'open',$3,$4,$5,$6,$7,$8,$9)
        RETURNING id
    `, input.TenantID, input.Subject, input.Priority,
		input.CustomerEmail, input.CustomerName, input.Source,
		nullableString(input.OrderID), slaFirstResponse, slaResolution,
	).Scan(&ticketID)
	if err != nil {
		return nil, fmt.Errorf("failed to create ticket: %w", err)
	}

	// Create the opening message
	_, err = tx.Exec(ctx, `
        INSERT INTO ticket_messages
            (ticket_id, author_type, author_email, author_name, body)
        VALUES ($1, 'customer', $2, $3, $4)
    `, ticketID, input.CustomerEmail, input.CustomerName, input.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create opening message: %w", err)
	}

	// Create ticket_created event
	eventMeta, _ := json.Marshal(map[string]string{
		"subject": input.Subject,
		"source":  input.Source,
	})
	_, err = tx.Exec(ctx, `
        INSERT INTO ticket_events (ticket_id, event_type, metadata)
        VALUES ($1, 'created', $2)
    `, ticketID, string(eventMeta))
	if err != nil {
		return nil, fmt.Errorf("failed to record created event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return s.GetTicketWithMessages(ctx, input.TenantID, ticketID)
}

// ReplyToTicket adds a staff reply or internal note
type ReplyInput struct {
	TenantID   string
	TicketID   string
	AuthorID   string
	Body       string
	IsInternal bool
}

func (s *TicketService) ReplyToTicket(ctx context.Context, input ReplyInput) (*Message, error) {
	if err := s.setTenantContext(ctx, input.TenantID); err != nil {
		return nil, fmt.Errorf("failed to set tenant context: %w", err)
	}

	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	// Rollback is a no-op after a successful commit
	defer func() { _ = tx.Rollback(ctx) }()

	// Get author name
	var authorName, authorEmail string
	err = tx.QueryRow(ctx, `
        SELECT COALESCE(first_name || ' ' || last_name, ''), email
        FROM users WHERE id = $1
    `, input.AuthorID).Scan(&authorName, &authorEmail)
	if err != nil {
		return nil, fmt.Errorf("author %s not found: %w", input.AuthorID, err)
	}

	// Insert message
	var messageID string
	err = tx.QueryRow(ctx, `
        INSERT INTO ticket_messages
            (ticket_id, author_type, author_id, author_email, author_name, body, is_internal)
        VALUES ($1, 'staff', $2, $3, $4, $5, $6)
        RETURNING id
    `, input.TicketID, input.AuthorID, authorEmail, authorName, input.Body, input.IsInternal).Scan(&messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to insert reply: %w", err)
	}

	// If this is the FIRST staff reply, record first_response_at for SLA
	if !input.IsInternal {
		_, err = tx.Exec(ctx, `
            UPDATE tickets
            SET first_response_at = NOW(),
                status = 'pending'
            WHERE id = $1 AND first_response_at IS NULL
        `, input.TicketID)
		if err != nil {
			return nil, fmt.Errorf("failed to record first response: %w", err)
		}
	}

	// Record event
	_, err = tx.Exec(ctx, `
        INSERT INTO ticket_events (ticket_id, user_id, event_type, metadata)
        VALUES ($1, $2, 'replied', $3)
    `, input.TicketID, input.AuthorID,
		fmt.Sprintf(`{"is_internal":%v}`, input.IsInternal))
	if err != nil {
		return nil, fmt.Errorf("failed to record reply event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	return &Message{
		ID:          messageID,
		AuthorType:  "staff",
		AuthorID:    &input.AuthorID,
		AuthorEmail: &authorEmail,
		AuthorName:  &authorName,
		Body:        input.Body,
		IsInternal:  input.IsInternal,
		CreatedAt:   time.Now(),
	}, nil
}

// UpdateTicketStatus changes status and records the event
func (s *TicketService) UpdateTicketStatus(ctx context.Context, tenantID, ticketID, userID, newStatus string) error {
	if err := s.setTenantContext(ctx, tenantID); err != nil {
		return fmt.Errorf("failed to set tenant context: %w", err)
	}

	var resolvedAt *time.Time
	if newStatus == "resolved" || newStatus == "closed" {
		now := time.Now()
		resolvedAt = &now
	}

	_, err := s.DB.Exec(ctx, `
        UPDATE tickets
        SET status = $1, resolved_at = COALESCE($2, resolved_at)
        WHERE id = $3 AND tenant_id = $4
    `, newStatus, resolvedAt, ticketID, tenantID)
	if err != nil {
		return err
	}

	// An empty userID means no human performed the change (the AI
	// auto-reply resolving a ticket). user_id is nullable, so record
	// NULL rather than letting an invalid UUID fail the insert.
	var actor any
	if userID != "" {
		actor = userID
	}

	// Event is best-effort — the status change itself already succeeded
	_, _ = s.DB.Exec(ctx, `
        INSERT INTO ticket_events (ticket_id, user_id, event_type, metadata)
        VALUES ($1, $2, 'status_changed', $3)
    `, ticketID, actor, fmt.Sprintf(`{"new_status":"%s"}`, newStatus))

	return nil
}

// ─────────────────────────────────────────────────────────────
// HELPER TYPES
// ─────────────────────────────────────────────────────────────

type Ticket struct {
	ID                 string
	Number             int
	Subject            string
	Status             string
	Priority           string
	CustomerEmail      string
	CustomerName       *string
	AssigneeID         *string
	AssigneeName       string
	Source             string
	FirstResponseAt    *time.Time
	SLAFirstResponseAt *time.Time
	ResolvedAt         *time.Time
	SLAStatus          string
	AIDraftBody        *string
	AIDraftConfidence  *float64
	LatestMessage      string
	UnreadCount        int
	Messages           []Message
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type Message struct {
	ID          string
	AuthorType  string
	AuthorID    *string
	AuthorEmail *string
	AuthorName  *string
	Body        string
	IsInternal  bool
	CreatedAt   time.Time
}

type ListTicketsOptions struct {
	Status     string
	Priority   string
	AssigneeID string
	Search     string
	Limit      int
}

func calculateSLAStatus(deadline *time.Time, firstResponseAt *time.Time) string {
	if firstResponseAt != nil {
		return "met"
	}
	if deadline == nil {
		return "none"
	}
	now := time.Now()
	if now.After(*deadline) {
		return "breached"
	}
	if now.After(deadline.Add(-15 * time.Minute)) {
		return "at_risk"
	}
	return "ok"
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
