package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/hellohirusha/ownstall/internal/auth"
)

// AdminAuthHandler serves the platform operator console's auth endpoints.
type AdminAuthHandler struct {
	DB *pgxpool.Pool
}

type adminPayload struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name,omitempty"`
		Role  string `json:"role"`
	} `json:"user"`
}

// EnsureBootstrapAdmin creates the first operator account from the environment
// so a fresh deployment has someone who can approve the first stall. It only
// ever inserts when the table is empty — it will not reset or overwrite an
// existing operator's password.
func EnsureBootstrapAdmin(ctx context.Context, db *pgxpool.Pool) error {
	email := strings.TrimSpace(strings.ToLower(os.Getenv("PLATFORM_ADMIN_EMAIL")))
	password := os.Getenv("PLATFORM_ADMIN_PASSWORD")

	if email == "" || password == "" {
		return nil
	}

	var existing int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM platform_admins`).Scan(&existing); err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}

	if len(password) < 12 {
		return fmt.Errorf("PLATFORM_ADMIN_PASSWORD must be at least 12 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	_, err = db.Exec(ctx,
		`INSERT INTO platform_admins (email, password_hash, name) VALUES ($1, $2, $3)`,
		email, string(hash), os.Getenv("PLATFORM_ADMIN_NAME"),
	)
	if err != nil {
		return err
	}

	log.Printf("Created bootstrap platform admin: %s", email)
	return nil
}

func (h *AdminAuthHandler) issueAdminTokens(
	ctx context.Context, adminID, email string,
) (string, string, error) {
	accessToken, err := auth.GenerateScopedAccessToken(
		adminID, "", email, "platform_admin", auth.ScopeAdmin,
	)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := auth.GenerateScopedRefreshToken(adminID, "", auth.ScopeAdmin)
	if err != nil {
		return "", "", err
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(refreshToken)))
	_, err = h.DB.Exec(ctx,
		`INSERT INTO refresh_tokens (admin_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		adminID, tokenHash, time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

// Login authenticates a platform operator. POST /api/admin/login
//
// There is no admin signup endpoint by design: operator accounts are created
// by an existing operator or by EnsureBootstrapAdmin, never self-served.
func (h *AdminAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	var admin struct {
		ID           string
		PasswordHash string
		Name         *string
		IsActive     bool
	}
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, password_hash, name, is_active FROM platform_admins WHERE email = $1`,
		email,
	).Scan(&admin.ID, &admin.PasswordHash, &admin.Name, &admin.IsActive)
	if err != nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	if !admin.IsActive {
		http.Error(w, `{"error":"account is deactivated"}`, http.StatusForbidden)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	_, _ = h.DB.Exec(r.Context(), `UPDATE platform_admins SET last_login_at = NOW() WHERE id = $1`, admin.ID)

	accessToken, refreshToken, err := h.issueAdminTokens(r.Context(), admin.ID, email)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	resp := adminPayload{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: 900}
	resp.User.ID = admin.ID
	resp.User.Email = email
	resp.User.Role = "platform_admin"
	if admin.Name != nil {
		resp.User.Name = *admin.Name
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// Refresh exchanges an operator refresh token for a new access token.
// POST /api/admin/refresh
func (h *AdminAuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	claims, err := auth.ValidateToken(req.RefreshToken)
	if err != nil || claims.Type != "refresh" || claims.ScopeOf() != auth.ScopeAdmin {
		http.Error(w, `{"error":"invalid or expired refresh token"}`, http.StatusUnauthorized)
		return
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.RefreshToken)))
	var stored string
	err = h.DB.QueryRow(r.Context(),
		`SELECT admin_id FROM refresh_tokens
		 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		tokenHash,
	).Scan(&stored)
	if err != nil || stored != claims.UserID {
		http.Error(w, `{"error":"refresh token not found or revoked"}`, http.StatusUnauthorized)
		return
	}

	var email string
	var isActive bool
	err = h.DB.QueryRow(r.Context(),
		`SELECT email, is_active FROM platform_admins WHERE id = $1`, claims.UserID,
	).Scan(&email, &isActive)
	if err != nil || !isActive {
		http.Error(w, `{"error":"account not found"}`, http.StatusUnauthorized)
		return
	}

	accessToken, err := auth.GenerateScopedAccessToken(
		claims.UserID, "", email, "platform_admin", auth.ScopeAdmin,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"access_token": accessToken,
		"expires_in":   900,
	})
}

// Logout revokes an operator refresh token. POST /api/admin/logout
func (h *AdminAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.RefreshToken)))
	_, _ = h.DB.Exec(r.Context(),
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND admin_id IS NOT NULL`,
		tokenHash,
	)

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"message":"logged out"}`))
}
