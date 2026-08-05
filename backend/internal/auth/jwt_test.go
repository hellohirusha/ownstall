package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testSecret = "test-secret-not-used-anywhere-real"

func TestGenerateAccessToken_RoundTrip(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	token, err := GenerateAccessToken("user-1", "tenant-1", "owner@example.com", "owner")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-1")
	}
	if claims.TenantID != "tenant-1" {
		t.Errorf("TenantID = %q, want %q", claims.TenantID, "tenant-1")
	}
	if claims.Email != "owner@example.com" {
		t.Errorf("Email = %q, want %q", claims.Email, "owner@example.com")
	}
	if claims.Role != "owner" {
		t.Errorf("Role = %q, want %q", claims.Role, "owner")
	}
	if claims.Type != string(AccessToken) {
		t.Errorf("Type = %q, want %q", claims.Type, AccessToken)
	}
	if claims.Issuer != "ownstall" {
		t.Errorf("Issuer = %q, want %q", claims.Issuer, "ownstall")
	}
}

// The access token lifetime is the blast radius of a leaked token, so
// it is asserted rather than left to drift.
func TestGenerateAccessToken_ExpiresIn15Minutes(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	token, err := GenerateAccessToken("user-1", "tenant-1", "owner@example.com", "owner")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 15*time.Minute || ttl < 14*time.Minute {
		t.Errorf("access token TTL = %v, want ~15m", ttl)
	}
}

func TestGenerateRefreshToken_RoundTrip(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	token, err := GenerateRefreshToken("user-1", "tenant-1")
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims.Type != string(RefreshToken) {
		t.Errorf("Type = %q, want %q", claims.Type, RefreshToken)
	}
	// A refresh token carries no email or role: it is only ever
	// exchanged for an access token, and the values would be stale by
	// the time it is used.
	if claims.Email != "" || claims.Role != "" {
		t.Errorf("refresh token carries Email=%q Role=%q, want both empty",
			claims.Email, claims.Role)
	}

	ttl := time.Until(claims.ExpiresAt.Time)
	if ttl > 7*24*time.Hour || ttl < 7*24*time.Hour-time.Minute {
		t.Errorf("refresh token TTL = %v, want ~7 days", ttl)
	}
}

func TestGenerateToken_NoSecretConfigured(t *testing.T) {
	t.Setenv("JWT_SECRET", "")

	if _, err := GenerateAccessToken("u", "t", "e", "r"); err == nil {
		t.Error("GenerateAccessToken with no JWT_SECRET: got nil error, want failure")
	}
	if _, err := GenerateRefreshToken("u", "t"); err == nil {
		t.Error("GenerateRefreshToken with no JWT_SECRET: got nil error, want failure")
	}
}

func TestValidateToken_WrongSecret(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)
	token, err := GenerateAccessToken("user-1", "tenant-1", "owner@example.com", "owner")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	// Simulate the token arriving at a service holding a different key.
	t.Setenv("JWT_SECRET", "a-completely-different-secret")

	if _, err := ValidateToken(token); err == nil {
		t.Error("ValidateToken with mismatched secret: got nil error, want failure")
	}
}

func TestValidateToken_Expired(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	claims := Claims{
		UserID:   "user-1",
		TenantID: "tenant-1",
		Type:     string(AccessToken),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Hour)),
			Issuer:    "ownstall",
		},
	}
	expired, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign expired token: %v", err)
	}

	if _, err := ValidateToken(expired); err == nil {
		t.Error("ValidateToken on expired token: got nil error, want failure")
	}
}

// A token whose header says alg=none must not be accepted. This is the
// classic JWT downgrade: strip the signature, claim the algorithm is
// "none", and the claims are attacker-controlled.
func TestValidateToken_RejectsAlgNone(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	claims := Claims{
		UserID:   "attacker",
		TenantID: "someone-elses-tenant",
		Role:     "owner",
		Type:     string(AccessToken),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    "ownstall",
		},
	}
	unsigned, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).
		SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign alg=none token: %v", err)
	}

	if _, err := ValidateToken(unsigned); err == nil {
		t.Error("ValidateToken accepted an alg=none token")
	}
}

func TestValidateToken_Malformed(t *testing.T) {
	t.Setenv("JWT_SECRET", testSecret)

	cases := map[string]string{
		"empty":            "",
		"not a jwt":        "definitely-not-a-token",
		"two segments":     "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0",
		"garbage payload":  "eyJhbGciOiJIUzI1NiJ9.!!!!.signature",
		"tampered payload": "",
	}

	// Build the tampered case from a real token so only the payload differs.
	real, err := GenerateAccessToken("user-1", "tenant-1", "owner@example.com", "owner")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	parts := strings.Split(real, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT segments, got %d", len(parts))
	}
	// Swap in a payload claiming a different tenant, keeping the old signature.
	forged := Claims{
		UserID:   "user-1",
		TenantID: "victim-tenant",
		Type:     string(AccessToken),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	forgedToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, forged).
		SignedString([]byte("some-other-key"))
	if err != nil {
		t.Fatalf("sign forged token: %v", err)
	}
	forgedParts := strings.Split(forgedToken, ".")
	cases["tampered payload"] = parts[0] + "." + forgedParts[1] + "." + parts[2]

	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ValidateToken(token); err == nil {
				t.Errorf("ValidateToken(%q): got nil error, want failure", token)
			}
		})
	}
}
