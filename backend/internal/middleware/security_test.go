package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func noopHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestSecurityHeaders_AlwaysSet(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	rec := httptest.NewRecorder()

	SecurityHeaders(noopHandler()).ServeHTTP(rec, req)

	want := map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		// Authenticated per-tenant JSON must never sit in a shared cache.
		"Cache-Control": "no-store",
	}
	for header, expected := range want {
		if got := rec.Header().Get(header); got != expected {
			t.Errorf("%s = %q, want %q", header, got, expected)
		}
	}

	permissions := rec.Header().Get("Permissions-Policy")
	for _, feature := range []string{"camera=()", "geolocation=()", "microphone=()", "payment=()"} {
		if !strings.Contains(permissions, feature) {
			t.Errorf("Permissions-Policy %q missing %q", permissions, feature)
		}
	}
}

func TestSecurityHeaders_HSTSOnlyOverTLS(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(*http.Request)
		wantHSTS bool
	}{
		{
			// Sent over plaintext it is ignored by browsers anyway, and
			// would break local http development if one leaked through.
			name:     "plain http",
			setup:    func(*http.Request) {},
			wantHSTS: false,
		},
		{
			name:     "direct TLS connection",
			setup:    func(r *http.Request) { r.TLS = &tls.ConnectionState{} },
			wantHSTS: true,
		},
		{
			// Railway terminates TLS at the edge, so the origin sees http
			// and only this header proves the client leg was encrypted.
			name:     "behind a TLS-terminating proxy",
			setup:    func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "https") },
			wantHSTS: true,
		},
		{
			name:     "proxy reports http",
			setup:    func(r *http.Request) { r.Header.Set("X-Forwarded-Proto", "http") },
			wantHSTS: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			tc.setup(req)
			rec := httptest.NewRecorder()

			SecurityHeaders(noopHandler()).ServeHTTP(rec, req)

			hsts := rec.Header().Get("Strict-Transport-Security")
			if tc.wantHSTS && hsts == "" {
				t.Error("Strict-Transport-Security missing, want it set")
			}
			if !tc.wantHSTS && hsts != "" {
				t.Errorf("Strict-Transport-Security = %q, want it unset", hsts)
			}
		})
	}
}

func TestRequestSanitizer_OversizedAuthHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("A", maxAuthHeaderBytes))
	rec := httptest.NewRecorder()

	RequestSanitizer(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestHeaderFieldsTooLarge {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusRequestHeaderFieldsTooLarge)
	}
	if called {
		t.Error("downstream handler ran on an oversized header")
	}
}

func TestRequestSanitizer_AllowsNormalRequest(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/products?limit=10", nil)
	req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 200))
	rec := httptest.NewRecorder()

	RequestSanitizer(next).ServeHTTP(rec, req)

	if !called {
		t.Error("downstream handler did not run on a valid request")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// A NUL byte is never legitimate in a path or query and is a classic
// way to confuse downstream string handling.
func TestRequestSanitizer_RejectsNullBytes(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*http.Request)
	}{
		{"in path", func(r *http.Request) { r.URL.Path = "/api/products\x00.json" }},
		{"in query", func(r *http.Request) { r.URL.RawQuery = "id=1\x00" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
			})

			req := httptest.NewRequest(http.MethodGet, "/api/products", nil)
			tc.mutate(req)
			rec := httptest.NewRecorder()

			RequestSanitizer(next).ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
			if called {
				t.Error("downstream handler ran on a request containing a NUL byte")
			}
		})
	}
}
