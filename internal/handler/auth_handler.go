package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/middleware"
	"jobsity-chat/internal/security"
	"jobsity-chat/internal/service"
)

type AuthHandler struct {
	authService  *service.AuthService
	sessionRepo  domain.SessionRepository
	tokenMgr     *security.TokenManager
	isProduction bool
}

func NewAuthHandler(authService *service.AuthService, sessionRepo domain.SessionRepository, tokenMgr *security.TokenManager) *AuthHandler {
	env := os.Getenv("ENVIRONMENT")
	isProduction := env == "production" || env == "prod"

	return &AuthHandler{
		authService:  authService,
		sessionRepo:  sessionRepo,
		tokenMgr:     tokenMgr,
		isProduction: isProduction,
	}
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Success      bool             `json:"success"`
	User         RegisterResponse `json:"user"`
	SessionToken string           `json:"session_token"` // Token for WebSocket authentication
	CSRFToken    string           `json:"csrf_token"`    // Token for CSRF protection
}

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
		case errors.Is(err, domain.ErrUsernameExists), errors.Is(err, domain.ErrEmailExists):
			status = http.StatusConflict
			message = "User already exists"
			slog.Info("registration failed: user exists",
				slog.String("username", req.Username),
				slog.String("email", req.Email))
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
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode register response", slog.String("error", err.Error()))
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

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

		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"`+message+`"}`, status)
		return
	}

	// Generate CSRF token for the session
	csrfToken, err := h.tokenMgr.Generate()
	if err != nil {
		slog.Error("failed to generate CSRF token", slog.String("error", err.Error()))
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	// Store CSRF token in session
	if err := h.sessionRepo.UpdateCSRFToken(r.Context(), csrfToken, session.Token); err != nil {
		slog.Error("failed to store CSRF token", slog.String("error", err.Error()))
		w.Header().Set("Content-Type", "application/json")
		http.Error(w, `{"error":"Internal server error"}`, http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.Token,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   h.isProduction,
		SameSite: http.SameSiteLaxMode,
	})

	resp := LoginResponse{
		Success: true,
		User: RegisterResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
		},
		SessionToken: session.Token,
		CSRFToken:    csrfToken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
		return
	}

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

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	session, ok := middleware.GetSession(r.Context())
	if !ok {
		http.Error(w, `{"error":"Session not found"}`, http.StatusUnauthorized)
		return
	}

	if err := h.authService.Logout(r.Context(), session.Token); err != nil {
		http.Error(w, `{"error":"Failed to logout"}`, http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.isProduction,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		slog.Error("failed to encode logout response", slog.String("error", err.Error()))
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
