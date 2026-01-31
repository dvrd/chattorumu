package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/testutil"
)

func TestAuth_ValidSession(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	session := testutil.NewTestSession(
		testutil.WithToken("valid-token"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := Auth(sessionRepo)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	testutil.AssertStatusCode(t, w, http.StatusOK)
	testutil.AssertTrue(t, nextHandlerCalled, "next handler should be called")
}

func TestAuth_NoCookie(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
	})

	middleware := Auth(sessionRepo)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
	testutil.AssertFalse(t, nextHandlerCalled, "next handler should not be called")
	testutil.AssertContains(t, w.Body.String(), "Not authenticated")
}

func TestAuth_InvalidSession(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	// No sessions in repo - any token will be invalid

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
	})

	middleware := Auth(sessionRepo)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "invalid-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
	testutil.AssertFalse(t, nextHandlerCalled, "next handler should not be called")
	testutil.AssertContains(t, w.Body.String(), "Invalid or expired session")
}

func TestAuth_ExpiredSession(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	// Create an expired session
	expiredSession := testutil.NewTestSession(
		testutil.WithToken("expired-token"),
		testutil.WithSessionUserID("user-123"),
		testutil.WithExpired(),
	)
	sessionRepo.Sessions[expiredSession.Token] = expiredSession

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
	})

	middleware := Auth(sessionRepo)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "expired-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
	testutil.AssertFalse(t, nextHandlerCalled, "next handler should not be called")
	testutil.AssertContains(t, w.Body.String(), "Invalid or expired session")
}

func TestAuth_ContextInjection(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	session := testutil.NewTestSession(
		testutil.WithToken("valid-token"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	var capturedUserID string
	var capturedSession *domain.Session
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedUserID, _ = GetUserID(r.Context())
		capturedSession, _ = GetSession(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := Auth(sessionRepo)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	testutil.AssertEqual(t, capturedUserID, "user-123")
	testutil.AssertNotNil(t, capturedSession)
	testutil.AssertEqual(t, capturedSession.Token, "valid-token")
	testutil.AssertEqual(t, capturedSession.UserID, "user-123")
}

func TestAuth_RepositoryError(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	sessionRepo.GetByTokenFunc = func(ctx context.Context, token string) (*domain.Session, error) {
		return nil, domain.ErrSessionNotFound
	}

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
	})

	middleware := Auth(sessionRepo)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "some-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
	testutil.AssertFalse(t, nextHandlerCalled, "next handler should not be called")
}

func TestGetUserID_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDKey, "user-456")

	userID, ok := GetUserID(ctx)

	testutil.AssertTrue(t, ok, "should find user ID in context")
	testutil.AssertEqual(t, userID, "user-456")
}

func TestGetUserID_Missing(t *testing.T) {
	ctx := context.Background()

	userID, ok := GetUserID(ctx)

	testutil.AssertFalse(t, ok, "should not find user ID in context")
	testutil.AssertEqual(t, userID, "")
}

func TestGetUserID_WrongType(t *testing.T) {
	// Set wrong type in context
	ctx := context.WithValue(context.Background(), UserIDKey, 12345)

	userID, ok := GetUserID(ctx)

	testutil.AssertFalse(t, ok, "should return false for wrong type")
	testutil.AssertEqual(t, userID, "")
}

func TestGetSession_Present(t *testing.T) {
	session := &domain.Session{
		ID:        "session-1",
		UserID:    "user-123",
		Token:     "token-abc",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	ctx := context.WithValue(context.Background(), SessionKey, session)

	gotSession, ok := GetSession(ctx)

	testutil.AssertTrue(t, ok, "should find session in context")
	testutil.AssertNotNil(t, gotSession)
	testutil.AssertEqual(t, gotSession.ID, "session-1")
	testutil.AssertEqual(t, gotSession.UserID, "user-123")
}

func TestGetSession_Missing(t *testing.T) {
	ctx := context.Background()

	gotSession, ok := GetSession(ctx)

	testutil.AssertFalse(t, ok, "should not find session in context")
	testutil.AssertNil(t, gotSession)
}

func TestGetSession_WrongType(t *testing.T) {
	// Set wrong type in context
	ctx := context.WithValue(context.Background(), SessionKey, "not-a-session")

	gotSession, ok := GetSession(ctx)

	testutil.AssertFalse(t, ok, "should return false for wrong type")
	testutil.AssertNil(t, gotSession)
}

func TestWithUserID(t *testing.T) {
	ctx := context.Background()

	newCtx := WithUserID(ctx, "user-789")

	userID, ok := GetUserID(newCtx)
	testutil.AssertTrue(t, ok, "should find user ID in new context")
	testutil.AssertEqual(t, userID, "user-789")

	// Original context should not be modified
	_, okOrig := GetUserID(ctx)
	testutil.AssertFalse(t, okOrig, "original context should not have user ID")
}

func TestWithSession(t *testing.T) {
	ctx := context.Background()
	session := &domain.Session{
		ID:     "session-2",
		UserID: "user-456",
		Token:  "token-xyz",
	}

	newCtx := WithSession(ctx, session)

	gotSession, ok := GetSession(newCtx)
	testutil.AssertTrue(t, ok, "should find session in new context")
	testutil.AssertEqual(t, gotSession.ID, "session-2")

	// Original context should not be modified
	_, okOrig := GetSession(ctx)
	testutil.AssertFalse(t, okOrig, "original context should not have session")
}

func TestAuth_EmptyCookieValue(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
	})

	middleware := Auth(sessionRepo)
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: ""})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
	testutil.AssertFalse(t, nextHandlerCalled, "next handler should not be called")
}

func TestAuth_MultipleMiddleware(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	session := testutil.NewTestSession(
		testutil.WithToken("valid-token"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	// Test that auth middleware can be chained with other middleware
	callOrder := make([]string, 0)

	loggingMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callOrder = append(callOrder, "logging-before")
			next.ServeHTTP(w, r)
			callOrder = append(callOrder, "logging-after")
		})
	}

	finalHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callOrder = append(callOrder, "handler")
		w.WriteHeader(http.StatusOK)
	})

	handler := loggingMiddleware(Auth(sessionRepo)(finalHandler))

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	testutil.AssertStatusCode(t, w, http.StatusOK)
	testutil.AssertLen(t, callOrder, 3)
	testutil.AssertEqual(t, callOrder[0], "logging-before")
	testutil.AssertEqual(t, callOrder[1], "handler")
	testutil.AssertEqual(t, callOrder[2], "logging-after")
}
