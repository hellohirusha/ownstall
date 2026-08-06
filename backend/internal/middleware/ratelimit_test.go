package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsExempt(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// Stripe and Resend retry on a 429, and dropping a payment
		// webhook is worse than serving a burst. They are authenticated
		// by signature instead.
		{"/webhooks/stripe", true},
		{"/webhooks/resend", true},
		{"/webhooks/", true},
		{"/health", true},
		{"/health/ready", true},

		{"/query", false},
		{"/auth/login", false},
		{"/api/checkout/session", false},
		{"/", false},
		// Prefix matching must not be fooled by a lookalike path.
		{"/api/webhooks/stripe", false},
		{"/healthz-public", true}, // documents the prefix match, not an exact match
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := isExempt(tc.path); got != tc.want {
				t.Errorf("isExempt(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestNewRateLimiter_NilRedis(t *testing.T) {
	if rl := NewRateLimiter(nil, 100, time.Minute); rl != nil {
		t.Error("NewRateLimiter(nil, ...) returned a limiter, want nil")
	}
}

// Losing Redis must not take the API offline: a nil limiter's
// middleware is a pass-through.
func TestNilRateLimiter_IsPassThrough(t *testing.T) {
	var rl *RateLimiter

	for name, wrap := range map[string]func(http.Handler) http.Handler{
		"RateLimit":       rl.RateLimit,
		"TenantRateLimit": rl.TenantRateLimit,
	} {
		t.Run(name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/query", nil)
			rec := httptest.NewRecorder()

			wrap(next).ServeHTTP(rec, req)

			if !called {
				t.Error("nil limiter blocked the request, want pass-through")
			}
			if rec.Code != http.StatusOK {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
			}
		})
	}
}

// An unauthenticated request has no tenant to key on, so the tenant
// limiter must let it past to the per-IP limiter rather than erroring.
func TestTenantRateLimit_NoTenantInContext(t *testing.T) {
	rl := &RateLimiter{limit: 1, window: time.Minute}

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/query", nil)
	rec := httptest.NewRecorder()

	rl.TenantRateLimit(next).ServeHTTP(rec, req)

	if !called {
		t.Error("request without a tenant was blocked, want pass-through")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestContextKeysAreDistinct(t *testing.T) {
	ctx := context.WithValue(context.Background(), ContextKeyUserID, "user-1")
	ctx = context.WithValue(ctx, ContextKeyTenantID, "tenant-1")

	if got := GetUserID(ctx); got != "user-1" {
		t.Errorf("GetUserID = %q, want %q", got, "user-1")
	}
	if got := GetTenantID(ctx); got != "tenant-1" {
		t.Errorf("GetTenantID = %q, want %q", got, "tenant-1")
	}
}
