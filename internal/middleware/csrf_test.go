package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jobsity-chat/internal/domain"
)

// mockSessionRepository implements domain.SessionRepository for testing
type mockSessionRepository struct{}

func (m *mockSessionRepository) Create(ctx context.Context, session *domain.Session) error {
	return nil
}

func (m *mockSessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	return nil, domain.ErrSessionNotFound
}

func (m *mockSessionRepository) GetByCSRFToken(ctx context.Context, csrfToken string) (*domain.Session, error) {
	return nil, domain.ErrSessionNotFound
}

func (m *mockSessionRepository) UpdateCSRFToken(ctx context.Context, csrfToken, sessionToken string) error {
	return nil
}

func (m *mockSessionRepository) Delete(ctx context.Context, token string) error {
	return nil
}

func (m *mockSessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	return 0, nil
}

func TestCSRF_SkipsSafeMethod(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		method string
		name   string
	}{
		{http.MethodGet, "GET"},
		{http.MethodHead, "HEAD"},
		{http.MethodOptions, "OPTIONS"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/v1/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Should not require CSRF token for safe methods
			if w.Code != http.StatusOK {
				t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
			}
		})
	}
}

func TestCSRF_ExemptsHealthEndpoint(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// POST to /health should bypass CSRF (no session needed)
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}
}

func TestCSRF_ExemptsWebsocketEndpoint(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// POST to /ws/* should bypass CSRF (websocket uses different auth)
	req := httptest.NewRequest(http.MethodPost, "/ws/chat/room-123", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}
}

func TestCSRF_RejectsNonAuthenticatedRequest(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// POST without session context should fail
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestCSRF_RejectsMissingToken(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create authenticated request without CSRF token
	session := &domain.Session{
		ID:        "session-123",
		UserID:    "user-123",
		Token:     "token-123",
		CSRFToken: "csrf-abc",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	ctx := WithSession(context.Background(), session)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestCSRF_RejectsInvalidToken(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create authenticated request with invalid CSRF token
	session := &domain.Session{
		ID:        "session-123",
		UserID:    "user-123",
		Token:     "token-123",
		CSRFToken: "csrf-correct",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	ctx := WithSession(context.Background(), session)

	body := "csrf_token=csrf-wrong"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestCSRF_AcceptsValidTokenInFormData(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create authenticated request with valid CSRF token in form data
	session := &domain.Session{
		ID:        "session-123",
		UserID:    "user-123",
		Token:     "token-123",
		CSRFToken: "csrf-abc123",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	ctx := WithSession(context.Background(), session)

	body := "csrf_token=csrf-abc123&name=test"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}
}

func TestCSRF_AcceptsValidTokenInHeader(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create authenticated request with valid CSRF token in header
	session := &domain.Session{
		ID:        "session-123",
		UserID:    "user-123",
		Token:     "token-123",
		CSRFToken: "csrf-xyz789",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	ctx := WithSession(context.Background(), session)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", nil)
	req.Header.Set("X-CSRF-Token", "csrf-xyz789")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}
}

func TestCSRF_AcceptsAlternateHeaderName(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create authenticated request with valid CSRF token using alternate header
	session := &domain.Session{
		ID:        "session-123",
		UserID:    "user-123",
		Token:     "token-123",
		CSRFToken: "csrf-alt456",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	ctx := WithSession(context.Background(), session)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", nil)
	req.Header.Set("X-XSRF-Token", "csrf-alt456")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected %d, got %d", http.StatusOK, w.Code)
	}
}

func TestCSRF_ValidatesDELETERequests(t *testing.T) {
	sessionRepo := &mockSessionRepository{}
	middleware := CSRF(sessionRepo)

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// DELETE without CSRF token should fail
	session := &domain.Session{
		ID:        "session-123",
		UserID:    "user-123",
		Token:     "token-123",
		CSRFToken: "csrf-token",
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	ctx := WithSession(context.Background(), session)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/chatrooms/123", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected %d, got %d", http.StatusForbidden, w.Code)
	}
}
