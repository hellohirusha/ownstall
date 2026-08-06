package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/hellohirusha/ownstall/internal/auth"
	appMiddleware "github.com/hellohirusha/ownstall/internal/middleware"
)

// BuyerAuthHandler serves the shopper-side account endpoints. Buyers are
// deliberately separate from `users`: a user belongs to exactly one tenant,
// while a buyer shops across every stall and belongs to none.
type BuyerAuthHandler struct {
	DB *pgxpool.Pool
}

type buyerPayload struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	User         struct {
		ID        string `json:"id"`
		Email     string `json:"email"`
		FirstName string `json:"first_name,omitempty"`
		LastName  string `json:"last_name,omitempty"`
	} `json:"user"`
}

// issueBuyerTokens mints the pair and records the refresh token's hash so it
// can be revoked on logout.
func (h *BuyerAuthHandler) issueBuyerTokens(
	ctx context.Context, buyerID, email string,
) (string, string, error) {
	accessToken, err := auth.GenerateScopedAccessToken(
		buyerID, "", email, "buyer", auth.ScopeBuyer,
	)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := auth.GenerateScopedRefreshToken(buyerID, "", auth.ScopeBuyer)
	if err != nil {
		return "", "", err
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(refreshToken)))
	_, err = h.DB.Exec(ctx,
		`INSERT INTO refresh_tokens (buyer_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		buyerID, tokenHash, time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

type BuyerSignupRequest struct {
	Email     string `json:"email"`
	Password  string `json:"password"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// Signup creates a shopper account. POST /api/buyer/signup
func (h *BuyerAuthHandler) Signup(w http.ResponseWriter, r *http.Request) {
	var req BuyerSignupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || len(req.Password) < 8 {
		http.Error(w, `{"error":"email and a password of at least 8 characters are required"}`, http.StatusBadRequest)
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		http.Error(w, `{"error":"failed to process password"}`, http.StatusInternalServerError)
		return
	}

	var buyerID string
	err = h.DB.QueryRow(r.Context(),
		`INSERT INTO buyers (email, password_hash, first_name, last_name)
		 VALUES ($1, $2, NULLIF($3, ''), NULLIF($4, '')) RETURNING id`,
		req.Email, string(passwordHash), req.FirstName, req.LastName,
	).Scan(&buyerID)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			http.Error(w, `{"error":"an account with that email already exists"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"error":"failed to create account"}`, http.StatusInternalServerError)
		return
	}

	// Orders placed as a guest with this address belong to this person; claim
	// them so the account's history is complete from the first sign-in.
	_, err = h.DB.Exec(r.Context(),
		`UPDATE orders SET buyer_id = $1 WHERE buyer_id IS NULL AND lower(customer_email) = $2`,
		buyerID, req.Email,
	)
	if err != nil {
		fmt.Println("Warning: failed to claim guest orders:", err)
	}

	accessToken, refreshToken, err := h.issueBuyerTokens(r.Context(), buyerID, req.Email)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	resp := buyerPayload{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: 900}
	resp.User.ID = buyerID
	resp.User.Email = req.Email
	resp.User.FirstName = req.FirstName
	resp.User.LastName = req.LastName
	_ = json.NewEncoder(w).Encode(resp)
}

// Login authenticates a shopper. POST /api/buyer/login
func (h *BuyerAuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	var buyer struct {
		ID           string
		PasswordHash string
		FirstName    *string
		LastName     *string
		IsActive     bool
	}
	err := h.DB.QueryRow(r.Context(),
		`SELECT id, password_hash, first_name, last_name, is_active
		 FROM buyers WHERE email = $1`,
		email,
	).Scan(&buyer.ID, &buyer.PasswordHash, &buyer.FirstName, &buyer.LastName, &buyer.IsActive)
	if err != nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	if !buyer.IsActive {
		http.Error(w, `{"error":"account is deactivated"}`, http.StatusForbidden)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(buyer.PasswordHash), []byte(req.Password)); err != nil {
		http.Error(w, `{"error":"invalid email or password"}`, http.StatusUnauthorized)
		return
	}

	_, _ = h.DB.Exec(r.Context(), `UPDATE buyers SET last_login_at = NOW() WHERE id = $1`, buyer.ID)

	accessToken, refreshToken, err := h.issueBuyerTokens(r.Context(), buyer.ID, email)
	if err != nil {
		http.Error(w, `{"error":"failed to generate token"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	resp := buyerPayload{AccessToken: accessToken, RefreshToken: refreshToken, ExpiresIn: 900}
	resp.User.ID = buyer.ID
	resp.User.Email = email
	if buyer.FirstName != nil {
		resp.User.FirstName = *buyer.FirstName
	}
	if buyer.LastName != nil {
		resp.User.LastName = *buyer.LastName
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// Refresh exchanges a buyer refresh token for a new access token.
// POST /api/buyer/refresh
func (h *BuyerAuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	claims, err := auth.ValidateToken(req.RefreshToken)
	if err != nil || claims.Type != "refresh" || claims.ScopeOf() != auth.ScopeBuyer {
		http.Error(w, `{"error":"invalid or expired refresh token"}`, http.StatusUnauthorized)
		return
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.RefreshToken)))
	var stored string
	err = h.DB.QueryRow(r.Context(),
		`SELECT buyer_id FROM refresh_tokens
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
		`SELECT email, is_active FROM buyers WHERE id = $1`, claims.UserID,
	).Scan(&email, &isActive)
	if err != nil || !isActive {
		http.Error(w, `{"error":"account not found"}`, http.StatusUnauthorized)
		return
	}

	accessToken, err := auth.GenerateScopedAccessToken(
		claims.UserID, "", email, "buyer", auth.ScopeBuyer,
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

// Logout revokes a buyer refresh token. POST /api/buyer/logout
func (h *BuyerAuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	tokenHash := fmt.Sprintf("%x", sha256.Sum256([]byte(req.RefreshToken)))
	_, _ = h.DB.Exec(r.Context(),
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND buyer_id IS NOT NULL`,
		tokenHash,
	)

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"message":"logged out"}`))
}

// Me returns the signed-in shopper's profile. GET /api/buyer/me
func (h *BuyerAuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	buyerID := appMiddleware.GetBuyerID(r.Context())
	if buyerID == "" {
		http.Error(w, `{"error":"not signed in"}`, http.StatusUnauthorized)
		return
	}

	var email string
	var firstName, lastName *string
	var createdAt time.Time
	err := h.DB.QueryRow(r.Context(),
		`SELECT email, first_name, last_name, created_at FROM buyers WHERE id = $1`,
		buyerID,
	).Scan(&email, &firstName, &lastName, &createdAt)
	if err != nil {
		http.Error(w, `{"error":"account not found"}`, http.StatusNotFound)
		return
	}

	out := map[string]interface{}{
		"id":         buyerID,
		"email":      email,
		"created_at": createdAt,
	}
	if firstName != nil {
		out["first_name"] = *firstName
	}
	if lastName != nil {
		out["last_name"] = *lastName
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}
