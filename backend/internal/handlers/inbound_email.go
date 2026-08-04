package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/hellohirusha/ownstall/internal/services"
)

// InboundEmailHandler turns customer emails forwarded by Resend
// inbound routing into support tickets.
type InboundEmailHandler struct {
	DB            *pgxpool.Pool
	TicketService *services.TicketService
}

// HandleInboundEmail processes emails forwarded by Resend inbound routing
// POST /webhooks/email/inbound
func (h *InboundEmailHandler) HandleInboundEmail(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		From    string `json:"from"`
		To      string `json:"to"`
		Subject string `json:"subject"`
		Text    string `json:"text"`
		HTML    string `json:"html"`
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Extract customer name and email from "From" header
	// Format: "John Doe <john@example.com>" or just "john@example.com"
	customerName, customerEmail := parseEmailAddress(payload.From)

	// Determine which tenant this email is for based on the "To" address
	// e.g. support@myteststore.ownstall.app → subdomain = myteststore
	tenantID := h.lookupTenantByEmailAddress(r.Context(), payload.To)
	if tenantID == "" {
		// No tenant found — silently discard
		w.WriteHeader(http.StatusOK)
		return
	}

	// Pull threading headers: Message-ID identifies this email,
	// In-Reply-To points at the message the customer replied to
	var messageID, inReplyTo string
	for _, hdr := range payload.Headers {
		switch strings.ToLower(hdr.Name) {
		case "message-id":
			messageID = hdr.Value
		case "in-reply-to":
			inReplyTo = hdr.Value
		}
	}

	if inReplyTo != "" {
		// Check if we have a message with this email ID
		var existingTicketID string
		err := h.DB.QueryRow(r.Context(), `
            SELECT ticket_id FROM ticket_messages
            WHERE email_message_id = $1
            LIMIT 1
        `, inReplyTo).Scan(&existingTicketID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			fmt.Printf("ERROR: inbound email thread lookup failed: %v\n", err)
			http.Error(w, "lookup failed", http.StatusInternalServerError)
			return
		}

		if existingTicketID != "" {
			// Add as a reply to existing ticket
			if _, err := h.DB.Exec(r.Context(), `
                INSERT INTO ticket_messages
                    (ticket_id, author_type, author_email, author_name, body,
                     email_message_id, email_in_reply_to)
                VALUES ($1, 'customer', $2, $3, $4, $5, $6)
            `, existingTicketID, customerEmail, customerName,
				cleanEmailBody(payload.Text), nullableHeader(messageID), inReplyTo); err != nil {
				fmt.Printf("ERROR: inbound email reply insert failed: %v\n", err)
				http.Error(w, "insert failed", http.StatusInternalServerError)
				return
			}

			// Reopen ticket if it was resolved
			if _, err := h.DB.Exec(r.Context(), `
                UPDATE tickets
                SET status = 'open', updated_at = NOW()
                WHERE id = $1 AND status IN ('resolved', 'closed')
            `, existingTicketID); err != nil {
				fmt.Printf("ERROR: inbound email reopen failed: %v\n", err)
			}

			w.WriteHeader(http.StatusOK)
			return
		}
	}

	// Create a new ticket
	body := payload.Text
	if body == "" {
		body = "No text content (HTML email)"
	}

	ticket, err := h.TicketService.CreateTicket(r.Context(), services.CreateTicketInput{
		TenantID:      tenantID,
		Subject:       payload.Subject,
		Body:          cleanEmailBody(body),
		CustomerEmail: customerEmail,
		CustomerName:  customerName,
		Source:        "email",
	})
	if err != nil {
		fmt.Printf("ERROR: inbound email ticket creation failed: %v\n", err)
		http.Error(w, "ticket creation failed", http.StatusInternalServerError)
		return
	}

	// Record this email's Message-ID on the opening message so future
	// customer replies (In-Reply-To) thread onto this ticket
	if messageID != "" && len(ticket.Messages) > 0 {
		if _, err := h.DB.Exec(r.Context(), `
            UPDATE ticket_messages SET email_message_id = $1 WHERE id = $2
        `, messageID, ticket.Messages[0].ID); err != nil {
			fmt.Printf("ERROR: inbound email message-id update failed: %v\n", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func parseEmailAddress(raw string) (name, email string) {
	raw = strings.TrimSpace(raw)
	if idx := strings.Index(raw, "<"); idx >= 0 {
		name = strings.TrimSpace(raw[:idx])
		email = strings.Trim(raw[idx:], "<> ")
	} else {
		email = raw
	}
	return
}

// lookupTenantByEmailAddress maps an inbound "To" address to a tenant.
// support@myteststore.ownstall.app → subdomain = myteststore
func (h *InboundEmailHandler) lookupTenantByEmailAddress(ctx context.Context, to string) string {
	_, address := parseEmailAddress(to)
	parts := strings.Split(address, "@")
	if len(parts) < 2 {
		return ""
	}
	subdomain := strings.Split(parts[1], ".")[0]

	var tenantID string
	err := h.DB.QueryRow(ctx,
		"SELECT id FROM tenants WHERE subdomain = $1 AND is_active = true",
		subdomain,
	).Scan(&tenantID)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			fmt.Printf("ERROR: inbound email tenant lookup failed: %v\n", err)
		}
		return ""
	}
	return tenantID
}

func cleanEmailBody(text string) string {
	// Remove quoted reply sections (lines starting with ">")
	lines := strings.Split(text, "\n")
	var clean []string
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), ">") {
			clean = append(clean, line)
		}
	}
	return strings.TrimSpace(strings.Join(clean, "\n"))
}

// nullableHeader keeps the email_message_id column NULL (not '')
// when the header is missing, so the unique-ish lookups stay clean
func nullableHeader(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
