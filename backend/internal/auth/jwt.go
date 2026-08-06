package auth

import (
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Scope names the audience a token was minted for. Ownstall has three kinds
// of principal and they must never be interchangeable: a store owner's token
// must not open the operator console, and a shopper's token must not reach a
// tenant dashboard.
//
// Tokens issued before scopes existed carry no scope claim and decode as "".
// ScopeOf treats that as a tenant token, which is what those tokens were —
// and since it is never equal to ScopeAdmin or ScopeBuyer, an old token still
// cannot reach the new surfaces.
const (
	ScopeTenant = "tenant"
	ScopeBuyer  = "buyer"
	ScopeAdmin  = "admin"
)

// Claims is embedded in every JWT token
type Claims struct {
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Type     string `json:"type"`
	Scope    string `json:"scope,omitempty"`
	jwt.RegisteredClaims
}

// ScopeOf returns the token's audience, defaulting legacy tokens to tenant.
func (c *Claims) ScopeOf() string {
	if c.Scope == "" {
		return ScopeTenant
	}
	return c.Scope
}

// GenerateAccessToken creates a short-lived tenant access token (15 minutes)
func GenerateAccessToken(userID, tenantID, email, role string) (string, error) {
	return GenerateScopedAccessToken(userID, tenantID, email, role, ScopeTenant)
}

// GenerateScopedAccessToken creates a short-lived access token (15 minutes)
// bound to one audience. Buyers and platform admins pass an empty tenantID —
// neither belongs to a tenant.
func GenerateScopedAccessToken(subjectID, tenantID, email, role, scope string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET is not set")
	}

	claims := Claims{
		UserID:   subjectID,
		TenantID: tenantID,
		Email:    email,
		Role:     role,
		Type:     string(AccessToken),
		Scope:    scope,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ownstall",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// GenerateRefreshToken creates a long-lived tenant refresh token (7 days)
func GenerateRefreshToken(userID, tenantID string) (string, error) {
	return GenerateScopedRefreshToken(userID, tenantID, ScopeTenant)
}

// GenerateScopedRefreshToken creates a long-lived refresh token (7 days)
// bound to one audience.
func GenerateScopedRefreshToken(subjectID, tenantID, scope string) (string, error) {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		return "", fmt.Errorf("JWT_SECRET is not set")
	}

	claims := Claims{
		UserID:   subjectID,
		TenantID: tenantID,
		Type:     string(RefreshToken),
		Scope:    scope,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ownstall",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateToken parses and validates a JWT token string
func ValidateToken(tokenString string) (*Claims, error) {
	secret := os.Getenv("JWT_SECRET")

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
