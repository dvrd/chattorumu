package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)

// Session represents a user session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	CSRFToken string    `json:"csrf_token"` // CSRF protection token
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionRepository defines the interface for session data access
type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	GetByToken(ctx context.Context, token string) (*Session, error)
	GetByCSRFToken(ctx context.Context, csrfToken string) (*Session, error)
	UpdateCSRFToken(ctx context.Context, csrfToken, sessionToken string) error
	Delete(ctx context.Context, token string) error
	DeleteExpired(ctx context.Context) (int64, error)
}
