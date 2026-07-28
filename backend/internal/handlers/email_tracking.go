package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// EmailTrackingHandler receives engagement signals:
// pixel loads from mail clients and delivery events from Resend.
type EmailTrackingHandler struct {
	DB *pgxpool.Pool
}

// trackingPixel is a 1x1 transparent GIF — the smallest valid image
// a mail client will happily render.
var trackingPixel = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, // "GIF89a"
	0x01, 0x00, 0x01, 0x00, 0x80, 0x00, 0x00, // 1x1, global color table
	0x00, 0x00, 0x00, 0xff, 0xff, 0xff, // black, white
	0x21, 0xf9, 0x04, 0x01, 0x00, 0x00, 0x00, 0x00, // transparency extension
	0x2c, 0x00, 0x00, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, // image descriptor
	0x02, 0x02, 0x44, 0x01, 0x00, // image data
	0x3b, // trailer
}

// HandleOpen serves the tracking pixel embedded in every email.
// GET /webhooks/email/open?log_id=<uuid>
func (h *EmailTrackingHandler) HandleOpen(w http.ResponseWriter, r *http.Request) {
	logID := r.URL.Query().Get("log_id")
	if logID != "" {
		// Only move the status forward — never downgrade clicked back to opened
		if _, err := h.DB.Exec(r.Context(), `
            UPDATE email_logs
            SET status = 'opened', opened_at = COALESCE(opened_at, NOW())
            WHERE id = $1 AND status IN ('queued', 'sent', 'delivered')
        `, logID); err != nil {
			fmt.Printf("ERROR: failed to record open for log %s: %v\n", logID, err)
		}
	}

	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(trackingPixel) // client may already be gone — nothing to do
}

// resendWebhookEvent is the shape Resend POSTs for email.* events
type resendWebhookEvent struct {
	Type string `json:"type"`
	Data struct {
		EmailID string `json:"email_id"`
		Click   struct {
			Link string `json:"link"`
		} `json:"click"`
	} `json:"data"`
}

// HandleResendWebhook processes delivery events from Resend.
// POST /webhooks/resend
func (h *EmailTrackingHandler) HandleResendWebhook(w http.ResponseWriter, r *http.Request) {
	var event resendWebhookEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, `{"error":"invalid payload"}`, http.StatusBadRequest)
		return
	}

	// Always ACK with 200 below — Resend retries non-2xx responses,
	// and a permanently failing event would retry forever.
	if event.Data.EmailID == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	ctx := r.Context()

	switch event.Type {
	case "email.delivered":
		if _, err := h.DB.Exec(ctx, `
            UPDATE email_logs
            SET status = 'delivered'
            WHERE resend_message_id = $1 AND status IN ('queued', 'sent')
        `, event.Data.EmailID); err != nil {
			fmt.Printf("ERROR: resend webhook (delivered) for %s: %v\n", event.Data.EmailID, err)
		}

	case "email.opened":
		if _, err := h.DB.Exec(ctx, `
            UPDATE email_logs
            SET status = 'opened', opened_at = COALESCE(opened_at, NOW())
            WHERE resend_message_id = $1 AND status IN ('queued', 'sent', 'delivered')
        `, event.Data.EmailID); err != nil {
			fmt.Printf("ERROR: resend webhook (opened) for %s: %v\n", event.Data.EmailID, err)
		}

	case "email.clicked":
		if _, err := h.DB.Exec(ctx, `
            UPDATE email_logs
            SET status = 'clicked',
                clicked_at = COALESCE(clicked_at, NOW()),
                clicked_url = $2
            WHERE resend_message_id = $1
        `, event.Data.EmailID, event.Data.Click.Link); err != nil {
			fmt.Printf("ERROR: resend webhook (clicked) for %s: %v\n", event.Data.EmailID, err)
		}

	case "email.bounced":
		if _, err := h.DB.Exec(ctx, `
            UPDATE email_logs
            SET status = 'bounced', bounced_at = NOW()
            WHERE resend_message_id = $1
        `, event.Data.EmailID); err != nil {
			fmt.Printf("ERROR: resend webhook (bounced) for %s: %v\n", event.Data.EmailID, err)
		}
		h.suppress(ctx, event.Data.EmailID, "bounce")

	case "email.complained":
		if _, err := h.DB.Exec(ctx, `
            UPDATE email_logs
            SET status = 'complained', complained_at = NOW()
            WHERE resend_message_id = $1
        `, event.Data.EmailID); err != nil {
			fmt.Printf("ERROR: resend webhook (complained) for %s: %v\n", event.Data.EmailID, err)
		}
		h.suppress(ctx, event.Data.EmailID, "complaint")

	default:
		// email.sent, email.delivery_delayed, etc. — nothing to record
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"received":true}`))
}

// suppress adds the recipient of the given Resend message to the tenant's
// suppression list, so the worker never emails them again.
func (h *EmailTrackingHandler) suppress(ctx context.Context, resendID, reason string) {
	if _, err := h.DB.Exec(ctx, `
        INSERT INTO email_suppressions (tenant_id, email, reason)
        SELECT tenant_id, to_email, $2
        FROM email_logs
        WHERE resend_message_id = $1
        ON CONFLICT (tenant_id, email) DO NOTHING
    `, resendID, reason); err != nil {
		fmt.Printf("ERROR: failed to suppress recipient of %s (%s): %v\n", resendID, reason, err)
	}
}
