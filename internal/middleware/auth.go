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

func Auth(sessionRepo domain.SessionRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("session_id")
			if err != nil {
				http.Error(w, `{"error":"Not authenticated"}`, http.StatusUnauthorized)
				return
			}

			session, err := sessionRepo.GetByToken(r.Context(), cookie.Value)
			if err != nil {
				http.Error(w, `{"error":"Invalid or expired session"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, session.UserID)
			ctx = context.WithValue(ctx, SessionKey, session)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetUserID(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

func GetSession(ctx context.Context) (*domain.Session, bool) {
	session, ok := ctx.Value(SessionKey).(*domain.Session)
	return session, ok
}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

func WithSession(ctx context.Context, session *domain.Session) context.Context {
	return context.WithValue(ctx, SessionKey, session)
}
