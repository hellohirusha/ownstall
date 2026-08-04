package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/hellohirusha/ownstall/internal/auth"
)

type contextKey string

const (
	ContextKeyUserID   contextKey = "userID"
	ContextKeyTenantID contextKey = "tenantID"
	ContextKeyEmail    contextKey = "email"
	ContextKeyRole     contextKey = "role"
)

// AuthRequired middleware validates JWT and injects user info into context
func AuthRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"authorization header required"}`, http.StatusUnauthorized)
			return
		}

		// Expect "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error":"invalid authorization format"}`, http.StatusUnauthorized)
			return
		}

		// Validate JWT
		claims, err := auth.ValidateToken(parts[1])
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		// Ensure it's an access token (not a refresh token used as access)
		if claims.Type != "access" {
			http.Error(w, `{"error":"invalid token type"}`, http.StatusUnauthorized)
			return
		}

		// Inject claims into request context
		ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextKeyTenantID, claims.TenantID)
		ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
		ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthOptional injects user/tenant info into context if a valid access token
// is present, but does not reject requests without one. Use it for endpoints
// that serve both public and authenticated traffic (e.g. the GraphQL endpoint).
func AuthOptional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			if claims, err := auth.ValidateToken(parts[1]); err == nil && claims.Type == "access" {
				ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
				ctx = context.WithValue(ctx, ContextKeyTenantID, claims.TenantID)
				ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
				ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)
				r = r.WithContext(ctx)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// GetUserID extracts user ID from request context
func GetUserID(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyUserID).(string); ok {
		return v
	}
	return ""
}

// GetTenantID extracts tenant ID from request context
func GetTenantID(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyTenantID).(string); ok {
		return v
	}
	return ""
}
