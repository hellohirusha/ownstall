package handlers

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/hellohirusha/ownstall/internal/auth"
	"github.com/hellohirusha/ownstall/internal/services"
)

type AuthHandler struct {
	DB    *pgxpool.Pool
	Email *services.EmailService
}

type SignupRequest struct {
	StoreName string `json:"store_name" validate:"required,min=2,max=50"`
	Subdomain string `json:"subdomain" validate:"required,min=2,max=30,alphanum"`
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
}

type AuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	User         struct {
		ID       string `json:"id"`
		Email    string `json:"email"`
		TenantID string `json:"tenant_id"`
		Role     string `json:"role"`
	} `json:"user"`
}

func (h *AuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req SignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		http.Error(w, `{"error":"failed to process password"}`, http.StatusInternalServerError)
		return
	}

	tx, err := h.DB.Begin(r.Context())
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	defer func() { _ = tx.Rollback(r.Context()) }()

	var tenantID string
	err = tx.QueryRow(r.Context(),
		`INSERT INTO tenants (name, subdomain) VALUES ($1, $2) RETURNING id`,
		req.StoreName, req.Subdomain,
	).Scan(&tenantID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			http.Error(w, `{"error":"subdomain already taken"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to create tenant"}`, http.StatusInternalServerError)
		return
	}

	var userID string
	err = tx.QueryRow(r.Context(),
		`INSERT INTO users (tenant_id, email, password_hash, role)
		 VALUES ($1, $2, $3, 'owner') RETURNING id`,
		tenantID, req.Email, string(passwordHash),
	).Scan(&userID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			http.Error(w, `{"error":"email already registered"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to create user"}`, http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		http.Error(w, `{"error":"transaction failed"}`, http.StatusInternalServerError)
		return
	}

	if err := h.Email.SeedSystemTemplates(r.Context(), tenantID); err != nil {
		fmt.Println("Warning: failed to seed email templates:", err)
	}

	accessToken, err := auth.GenerateAccessToken(userID, tenantID, req.Email, "owner")
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	refreshToken, err := auth.GenerateRefreshToken(userID, tenantID)
	if err != nil {
		http.Error(w, `{"error":"failed to generate refresh token"}`, http.StatusInternalServerError)
		return
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(refreshToken)))
	_, err = h.DB.Exec(r.Context(),
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		fmt.Println("Warning: failed to store refresh token:", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	resp := AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
	}
	resp.User.ID = userID
	resp.User.Email = req.Email
	resp.User.TenantID = tenantID
	resp.User.Role = "owner"
	_ = json.NewEncoder(w).Encode(resp)
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	var user struct {
		ID           string
		TenantID     string
		PasswordHash string
		Role         string
		IsActive     bool
	}

	err := h.DB.QueryRow(r.Context(),
		`SELECT id, tenant_id, password_hash, role, is_active
		 FROM users WHERE email = $1`,
		req.Email,
	).Scan(&user.ID, &user.TenantID, &user.PasswordHash, &user.Role, &user.IsActive)
	if err != nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	if !user.IsActive {
		http.Error(w, `{"error":"account is deactivated"}`, http.StatusForbidden)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	_, _ = h.DB.Exec(r.Context(), `UPDATE users SET last_login_at = NOW() WHERE id = $1`, user.ID)

	accessToken, _ := auth.GenerateAccessToken(user.ID, user.TenantID, req.Email, user.Role)
	refreshToken, _ := auth.GenerateRefreshToken(user.ID, user.TenantID)

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(refreshToken)))
	_, _ = h.DB.Exec(r.Context(),
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		user.ID, tokenHash, time.Now().Add(7*24*time.Hour),
	)

	w.Header().Set("Content-Type", "application/json")
	resp := AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
	}
	resp.User.ID = user.ID
	resp.User.Email = req.Email
	resp.User.TenantID = user.TenantID
	resp.User.Role = user.Role
	_ = json.NewEncoder(w).Encode(resp)
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	claims, err := auth.ValidateToken(req.RefreshToken)
	if err != nil || claims.Type != "refresh" {
		http.Error(w, `{"error":"invalid or expired refresh token"}`, http.StatusUnauthorized)
		return
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.RefreshToken)))
	var expiresAt time.Time
	err = h.DB.QueryRow(r.Context(),
		`SELECT expires_at FROM refresh_tokens
		 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > NOW()`,
		tokenHash,
	).Scan(&expiresAt)
	if err != nil {
		http.Error(w, `{"error":"refresh token not found or revoked"}`, http.StatusUnauthorized)
		return
	}

	var user struct {
		Email    string
		TenantID string
		Role     string
	}
	err = h.DB.QueryRow(r.Context(),
		`SELECT email, tenant_id, role FROM users WHERE id = $1`,
		claims.UserID,
	).Scan(&user.Email, &user.TenantID, &user.Role)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusUnauthorized)
		return
	}

	accessToken, err := auth.GenerateAccessToken(claims.UserID, user.TenantID, user.Email, user.Role)
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

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.RefreshToken)))
	_, _ = h.DB.Exec(r.Context(),
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1`,
		tokenHash,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"message":"logged out"}`))
}
