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
	ContextKeyScope    contextKey = "scope"
	ContextKeyBuyerID  contextKey = "buyerID"
	ContextKeyAdminID  contextKey = "adminID"
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

		// A buyer or operator token must not open a tenant endpoint.
		if claims.ScopeOf() != auth.ScopeTenant {
			http.Error(w, `{"error":"wrong token audience"}`, http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r.WithContext(claimsContext(r.Context(), claims)))
	})
}

// claimsContext injects a validated token's claims under the keys that match
// its audience, so a handler reading GetBuyerID can never be handed a tenant
// user id by accident.
func claimsContext(ctx context.Context, claims *auth.Claims) context.Context {
	scope := claims.ScopeOf()

	ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
	ctx = context.WithValue(ctx, ContextKeyScope, scope)

	switch scope {
	case auth.ScopeBuyer:
		ctx = context.WithValue(ctx, ContextKeyBuyerID, claims.UserID)
	case auth.ScopeAdmin:
		ctx = context.WithValue(ctx, ContextKeyAdminID, claims.UserID)
	default:
		ctx = context.WithValue(ctx, ContextKeyUserID, claims.UserID)
		ctx = context.WithValue(ctx, ContextKeyTenantID, claims.TenantID)
		ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)
	}

	return ctx
}

// scopedAuthRequired builds a middleware that admits only access tokens minted
// for the given audience.
func scopedAuthRequired(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			parts := strings.SplitN(r.Header.Get("Authorization"), " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error":"authorization header required"}`, http.StatusUnauthorized)
				return
			}

			claims, err := auth.ValidateToken(parts[1])
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			if claims.Type != "access" {
				http.Error(w, `{"error":"invalid token type"}`, http.StatusUnauthorized)
				return
			}

			if claims.ScopeOf() != scope {
				http.Error(w, `{"error":"wrong token audience"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r.WithContext(claimsContext(r.Context(), claims)))
		})
	}
}

// BuyerRequired admits only buyer-scoped access tokens.
var BuyerRequired = scopedAuthRequired(auth.ScopeBuyer)

// PlatformAdminRequired admits only operator-scoped access tokens.
var PlatformAdminRequired = scopedAuthRequired(auth.ScopeAdmin)

// AuthOptional injects user/tenant info into context if a valid access token
// is present, but does not reject requests without one. Use it for endpoints
// that serve both public and authenticated traffic (e.g. the GraphQL endpoint).
func AuthOptional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			// Any audience is welcome here — GraphQL serves all three — but
			// the claims still land under keys matching the token's scope.
			if claims, err := auth.ValidateToken(parts[1]); err == nil && claims.Type == "access" {
				r = r.WithContext(claimsContext(r.Context(), claims))
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

// GetBuyerID returns the signed-in shopper's id, or "" for a guest.
func GetBuyerID(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyBuyerID).(string); ok {
		return v
	}
	return ""
}

// GetAdminID returns the signed-in operator's id, or "" when the caller is not
// a platform admin.
func GetAdminID(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyAdminID).(string); ok {
		return v
	}
	return ""
}

// GetScope returns the audience of the caller's token, or "" when anonymous.
func GetScope(ctx context.Context) string {
	if v, ok := ctx.Value(ContextKeyScope).(string); ok {
		return v
	}
	return ""
}
