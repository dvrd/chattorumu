package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/middleware"
	"jobsity-chat/internal/service"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *service.AuthService
	isProduction bool
}

// NewAuthHandler creates a new authentication handler
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	// Detect production environment
	env := os.Getenv("ENVIRONMENT")
	isProduction := env == "production" || env == "prod"

	return &AuthHandler{
		authService: authService,
		isProduction: isProduction,
	}
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterResponse represents registration response
type RegisterResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

// LoginRequest represents login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Success      bool             `json:"success"`
	User         RegisterResponse `json:"user"`
	SessionToken string           `json:"session_token"` // Token for WebSocket authentication
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	user, err := h.authService.Register(r.Context(), req.Username, req.Email, req.Password)
	if err != nil {
		var status int
		var message string

		switch {
		case errors.Is(err, domain.ErrInvalidInput):
			status = http.StatusBadRequest
			message = "Invalid input"
		case errors.Is(err, domain.ErrUsernameExists):
			status = http.StatusConflict
			message = "Username already exists"
		case errors.Is(err, domain.ErrEmailExists):
			status = http.StatusConflict
			message = "Email already exists"
		default:
			status = http.StatusInternalServerError
			message = "Internal server error"
			slog.Error("register error", slog.String("error", err.Error()))
		}

		http.Error(w, `{"error":"`+message+`"}`, status)
		return
	}

	resp := RegisterResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	session, user, err := h.authService.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		var status int
		var message string

		if errors.Is(err, domain.ErrInvalidCredentials) {
			status = http.StatusUnauthorized
			message = "Invalid credentials"
		} else {
			status = http.StatusInternalServerError
			message = "Internal server error"
			slog.Error("login error", slog.String("error", err.Error()))
		}

		http.Error(w, `{"error":"`+message+`"}`, status)
		return
	}

	// Set session cookie with environment-aware security settings
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.Token,
		Path:     "/",
		MaxAge:   86400, // 24 hours
		HttpOnly: true,
		Secure:   h.isProduction, // Auto-enable Secure flag in production
		SameSite: http.SameSiteLaxMode, // Lax allows WebSocket upgrades
	})

	resp := LoginResponse{
		Success: true,
		User: RegisterResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
		SessionToken: session.Token, // Include token for WebSocket connections
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Me returns current authenticated user info
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context (set by auth middleware)
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Get user from database
	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"User not found"}`, http.StatusNotFound)
		return
	}

	resp := RegisterResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	// Get session from context
	session, ok := middleware.GetSession(r.Context())
	if !ok {
		http.Error(w, `{"error":"Session not found"}`, http.StatusUnauthorized)
		return
	}

	// Delete session
	if err := h.authService.Logout(r.Context(), session.Token); err != nil {
		http.Error(w, `{"error":"Failed to logout"}`, http.StatusInternalServerError)
		return
	}

	// Clear cookie with environment-aware security settings
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.isProduction, // Match security settings
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
