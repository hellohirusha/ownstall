package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hellohirusha/ownstall/internal/auth"
)

const testSecret = "test-secret-not-used-anywhere-real"

// okHandler records what the middleware put in the context and reports 200.
func okHandler(seen *map[string]string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = map[string]string{
			"userID":   GetUserID(r.Context()),
			"tenantID": GetTenantID(r.Context()),
		}
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuthRequired_RejectsUnauthenticated(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	refresh, err := auth.GenerateRefreshToken("user-1", "tenant-1")
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	cases := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"bare token without scheme", "sometoken"},
		{"wrong scheme", "Basic dXNlcjpwYXNz"},
		{"lowercase bearer", "bearer sometoken"},
		{"bearer with garbage token", "Bearer not-a-jwt"},
		{"empty bearer token", "Bearer "},
		// A refresh token is a valid signed JWT. Accepting it as an
		// access token would extend a 15-minute credential to 7 days.
		{"refresh token used as access", "Bearer " + refresh},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()

			AuthRequired(next).ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
			}
			if called {
				t.Error("downstream handler ran on a rejected request")
			}
		})
	}
}

func TestAuthRequired_AcceptsAccessTokenAndPopulatesContext(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	token, err := auth.GenerateAccessToken("user-1", "tenant-1", "owner@example.com", "owner")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	var seen map[string]string
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	AuthRequired(okHandler(&seen)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if seen["userID"] != "user-1" {
		t.Errorf("context userID = %q, want %q", seen["userID"], "user-1")
	}
	if seen["tenantID"] != "tenant-1" {
		t.Errorf("context tenantID = %q, want %q", seen["tenantID"], "tenant-1")
	}
}

func TestAuthOptional(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	access, err := auth.GenerateAccessToken("user-1", "tenant-1", "owner@example.com", "owner")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	refresh, err := auth.GenerateRefreshToken("user-1", "tenant-1")
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	cases := []struct {
		name         string
		header       string
		wantTenantID string
	}{
		// Anonymous storefront traffic must still reach the resolver.
		{"no header passes through anonymously", "", ""},
		{"invalid token passes through anonymously", "Bearer garbage", ""},
		{"refresh token does not authenticate", "Bearer " + refresh, ""},
		{"valid access token populates tenant", "Bearer " + access, "tenant-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seen map[string]string
			req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader("{}"))
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()

			AuthOptional(okHandler(&seen)).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (AuthOptional must never reject)",
					rec.Code, http.StatusOK)
			}
			if seen["tenantID"] != tc.wantTenantID {
				t.Errorf("context tenantID = %q, want %q", seen["tenantID"], tc.wantTenantID)
			}
		})
	}
}

func TestGetters_MissingValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	if got := GetUserID(req.Context()); got != "" {
		t.Errorf("GetUserID on bare context = %q, want empty", got)
	}
	if got := GetTenantID(req.Context()); got != "" {
		t.Errorf("GetTenantID on bare context = %q, want empty", got)
	}
}
