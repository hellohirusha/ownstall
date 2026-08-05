package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testWebhookSecret = "whsec_test_secret_not_used_anywhere_real"

// stripeSignature builds the Stripe-Signature header for a payload the
// way Stripe does: HMAC-SHA256 over "<timestamp>.<payload>".
func stripeSignature(t *testing.T, payload []byte, secret string, ts time.Time) string {
	t.Helper()

	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := fmt.Fprintf(mac, "%d.%s", ts.Unix(), payload); err != nil {
		t.Fatalf("build signature: %v", err)
	}
	return fmt.Sprintf("t=%d,v1=%s", ts.Unix(), hex.EncodeToString(mac.Sum(nil)))
}

func webhookRequest(payload []byte, signature string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", strings.NewReader(string(payload)))
	if signature != "" {
		req.Header.Set("Stripe-Signature", signature)
	}
	return req
}

// An unsigned or wrongly signed webhook is an attempt to mark an order
// paid without paying. It must be rejected before any database work.
func TestHandleStripeWebhook_RejectsBadSignature(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", testWebhookSecret)

	payload := []byte(`{"id":"evt_1","type":"checkout.session.completed",` +
		`"data":{"object":{"id":"cs_1","metadata":{"order_id":"order-1"}}}}`)

	cases := []struct {
		name      string
		signature string
	}{
		{"no signature header", ""},
		{"empty signature", "  "},
		{"garbage signature", "t=123,v1=deadbeef"},
		{"malformed header", "not-a-signature"},
		{
			"signed with the wrong secret",
			stripeSignature(t, payload, "whsec_an_attackers_guess", time.Now()),
		},
		{
			// Stripe's own tolerance check: an old signature is a replay.
			"valid signature but far outside the tolerance window",
			stripeSignature(t, payload, testWebhookSecret, time.Now().Add(-24*time.Hour)),
		},
	}

	// A nil DB is deliberate: if any of these reach the database, the
	// test panics rather than silently passing.
	h := &CheckoutHandler{}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.HandleStripeWebhook(rec, webhookRequest(payload, tc.signature))

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if body := rec.Body.String(); !strings.Contains(body, "invalid signature") {
				t.Errorf("body = %q, want it to report an invalid signature", body)
			}
		})
	}
}

// A tampered payload must fail even though the signature itself is
// well-formed: the HMAC covers the body.
func TestHandleStripeWebhook_RejectsTamperedPayload(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", testWebhookSecret)

	original := []byte(`{"id":"evt_1","type":"checkout.session.completed",` +
		`"data":{"object":{"id":"cs_1","amount_total":499}}}`)
	signature := stripeSignature(t, original, testWebhookSecret, time.Now())

	tampered := []byte(`{"id":"evt_1","type":"checkout.session.completed",` +
		`"data":{"object":{"id":"cs_1","amount_total":1}}}`)

	rec := httptest.NewRecorder()
	(&CheckoutHandler{}).HandleStripeWebhook(rec, webhookRequest(tampered, signature))

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// Stripe retries any non-200, so events we do not handle must still be
// acknowledged rather than left to pile up in their retry queue.
func TestHandleStripeWebhook_AcknowledgesUnhandledEvents(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", testWebhookSecret)

	payload := []byte(`{"id":"evt_2","type":"customer.subscription.updated",` +
		`"data":{"object":{"id":"sub_1"}}}`)
	signature := stripeSignature(t, payload, testWebhookSecret, time.Now())

	rec := httptest.NewRecorder()
	(&CheckoutHandler{}).HandleStripeWebhook(rec, webhookRequest(payload, signature))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); !strings.Contains(body, "received") {
		t.Errorf("body = %q, want an acknowledgement", body)
	}
}

// A completed session with no order_id in its metadata cannot be
// matched to an order. It is acknowledged and dropped rather than
// retried forever.
func TestHandleStripeWebhook_CompletedSessionWithoutOrderID(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", testWebhookSecret)

	payload := []byte(`{"id":"evt_3","type":"checkout.session.completed",` +
		`"data":{"object":{"id":"cs_legacy","metadata":{}}}}`)
	signature := stripeSignature(t, payload, testWebhookSecret, time.Now())

	rec := httptest.NewRecorder()
	(&CheckoutHandler{}).HandleStripeWebhook(rec, webhookRequest(payload, signature))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// An expired session carrying no order_id must not reach the database
// either — the handler has nothing to update.
func TestHandleStripeWebhook_ExpiredSessionWithoutOrderID(t *testing.T) {
	t.Setenv("STRIPE_WEBHOOK_SECRET", testWebhookSecret)

	payload := []byte(`{"id":"evt_4","type":"checkout.session.expired",` +
		`"data":{"object":{"id":"cs_expired","metadata":{}}}}`)
	signature := stripeSignature(t, payload, testWebhookSecret, time.Now())

	rec := httptest.NewRecorder()
	(&CheckoutHandler{}).HandleStripeWebhook(rec, webhookRequest(payload, signature))

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
