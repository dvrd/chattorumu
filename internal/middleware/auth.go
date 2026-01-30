package middleware

import (
	"context"
	"net/http"

	"jobsity-chat/internal/domain"
)

type contextKey string

const (
	UserIDKey  contextKey = "user_id"
	SessionKey contextKey = "session"
)

// Auth creates an authentication middleware
func Auth(sessionRepo domain.SessionRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get session cookie
			cookie, err := r.Cookie("session_id")
			if err != nil {
				http.Error(w, `{"error":"Not authenticated"}`, http.StatusUnauthorized)
				return
			}

			// Validate session
			session, err := sessionRepo.GetByToken(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"Invalid or expired session"}`, http.StatusUnauthorized)
				return
			}

			// Add user ID and session to context
			ctx := context.WithValue(r.Context(), UserIDKey, session.UserID)
			ctx = context.WithValue(ctx, SessionKey, session)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID retrieves the user ID from context
func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

// GetSession retrieves the session from context
func GetSession(ctx context.Context) (*domain.Session, bool) {
	session, ok := ctx.Value(SessionKey).(*domain.Session)
	return session, ok
}

// WithUserID adds a user ID to the context (for testing)
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithSession adds a session to the context (for testing)
func WithSession(ctx context.Context, session *domain.Session) context.Context {
	return context.WithValue(ctx, SessionKey, session)
}
