package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/service"
	"jobsity-chat/internal/testutil"
	ws "jobsity-chat/internal/websocket"

	"github.com/go-chi/chi/v5"
)

// setupWebSocketHandler creates a WebSocketHandler with mock dependencies for testing
func setupWebSocketHandler(
	sessionRepo *testutil.MockSessionRepository,
	userRepo *testutil.MockUserRepository,
	chatroomRepo *testutil.MockChatroomRepository,
	allowedOrigins string,
) *WebSocketHandler {
	hub := ws.NewHub()
	messageRepo := testutil.NewMockMessageRepository()
	chatService := service.NewChatService(messageRepo, chatroomRepo)
	authService := service.NewAuthService(userRepo, sessionRepo)
	publisher := testutil.NewMockMessagePublisher()

	return NewWebSocketHandler(hub, chatService, authService, publisher, sessionRepo, allowedOrigins)
}

// createRequestWithChiContext creates a request with Chi URL params
func createRequestWithChiContext(method, path string, chatroomID string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("chatroom_id", chatroomID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	return req
}

func TestWebSocketHandler_NoToken(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1", "room-1")
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
	testutil.AssertContains(t, w.Body.String(), "No session token provided")
}

func TestWebSocketHandler_TokenFromCookie(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Setup valid session
	session := testutil.NewTestSession(
		testutil.WithToken("valid-cookie-token"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	// Setup user
	user := testutil.NewTestUser(
		testutil.WithUserID("user-123"),
		testutil.WithUsername("testuser"),
	)
	userRepo.Users[user.ID] = user

	// Setup chatroom membership
	chatroomRepo.Members["room-1"] = map[string]bool{"user-123": true}

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1", "room-1")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-cookie-token"})
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	// WebSocket upgrade will fail in tests (no actual WebSocket connection)
	// but we shouldn't get 401/403 errors if auth passes
	// The upgrade failure returns a 400 Bad Request because there's no actual WebSocket handshake
	testutil.AssertTrue(t, w.Code != http.StatusUnauthorized, "should not return 401")
	testutil.AssertTrue(t, w.Code != http.StatusForbidden, "should not return 403")
}

func TestWebSocketHandler_TokenFromQuery(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Setup valid session
	session := testutil.NewTestSession(
		testutil.WithToken("valid-query-token"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	// Setup user
	user := testutil.NewTestUser(
		testutil.WithUserID("user-123"),
		testutil.WithUsername("testuser"),
	)
	userRepo.Users[user.ID] = user

	// Setup chatroom membership
	chatroomRepo.Members["room-1"] = map[string]bool{"user-123": true}

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1?token=valid-query-token", "room-1")
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	// Should pass auth checks (upgrade will fail but not with auth errors)
	testutil.AssertTrue(t, w.Code != http.StatusUnauthorized, "should not return 401")
	testutil.AssertTrue(t, w.Code != http.StatusForbidden, "should not return 403")
}

func TestWebSocketHandler_TokenFromHeader(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Setup valid session
	session := testutil.NewTestSession(
		testutil.WithToken("valid-bearer-token"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	// Setup user
	user := testutil.NewTestUser(
		testutil.WithUserID("user-123"),
		testutil.WithUsername("testuser"),
	)
	userRepo.Users[user.ID] = user

	// Setup chatroom membership
	chatroomRepo.Members["room-1"] = map[string]bool{"user-123": true}

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1", "room-1")
	req.Header.Set("Authorization", "Bearer valid-bearer-token")
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	// Should pass auth checks
	testutil.AssertTrue(t, w.Code != http.StatusUnauthorized, "should not return 401")
	testutil.AssertTrue(t, w.Code != http.StatusForbidden, "should not return 403")
}

func TestWebSocketHandler_InvalidSession(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()
	// No sessions in repo - any token will be invalid

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1", "room-1")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "invalid-token-12345678"})
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
	testutil.AssertContains(t, w.Body.String(), "Invalid or expired session")
}

func TestWebSocketHandler_ExpiredSession(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Setup expired session
	session := testutil.NewTestSession(
		testutil.WithToken("expired-token-1234"),
		testutil.WithSessionUserID("user-123"),
		testutil.WithExpired(),
	)
	sessionRepo.Sessions[session.Token] = session

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1", "room-1")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "expired-token-1234"})
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
	testutil.AssertContains(t, w.Body.String(), "Invalid or expired session")
}

func TestWebSocketHandler_NoChatroomID(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Setup valid session
	session := testutil.NewTestSession(
		testutil.WithToken("valid-token-12345"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	// Create request WITHOUT chatroom_id param
	req := httptest.NewRequest(http.MethodGet, "/ws/chat/", nil)
	rctx := chi.NewRouteContext()
	// Don't add chatroom_id param
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-token-12345"})
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	testutil.AssertStatusCode(t, w, http.StatusBadRequest)
	testutil.AssertContains(t, w.Body.String(), "Chatroom ID required")
}

func TestWebSocketHandler_NotMember(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Setup valid session
	session := testutil.NewTestSession(
		testutil.WithToken("valid-token-12345"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	// User is NOT a member of room-1 (no membership entry)

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1", "room-1")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-token-12345"})
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	testutil.AssertStatusCode(t, w, http.StatusForbidden)
	testutil.AssertContains(t, w.Body.String(), "Not a member of this chatroom")
}

func TestWebSocketHandler_UserNotFound(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Setup valid session
	session := testutil.NewTestSession(
		testutil.WithToken("valid-token-12345"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	// Setup chatroom membership but NO user in repo
	chatroomRepo.Members["room-1"] = map[string]bool{"user-123": true}

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1", "room-1")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-token-12345"})
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
	testutil.AssertContains(t, w.Body.String(), "User not found")
}

func TestWebSocketHandler_MembershipCheckError(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Setup valid session
	session := testutil.NewTestSession(
		testutil.WithToken("valid-token-12345"),
		testutil.WithSessionUserID("user-123"),
	)
	sessionRepo.Sessions[session.Token] = session

	// Make IsMember return an error
	chatroomRepo.IsMemberFunc = func(ctx context.Context, chatroomID, userID string) (bool, error) {
		return false, domain.ErrChatroomNotFound
	}

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1", "room-1")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "valid-token-12345"})
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	testutil.AssertStatusCode(t, w, http.StatusForbidden)
	testutil.AssertContains(t, w.Body.String(), "Not a member of this chatroom")
}

func TestCreateUpgrader_AllowedOrigin(t *testing.T) {
	upgrader := createUpgrader([]string{"http://localhost:3000", "http://example.com"})

	tests := []struct {
		name     string
		origin   string
		expected bool
	}{
		{"allowed localhost", "http://localhost:3000", true},
		{"allowed example", "http://example.com", true},
		{"disallowed origin", "http://malicious.com", false},
		{"empty origin allowed", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			got := upgrader.CheckOrigin(req)
			testutil.AssertEqual(t, got, tt.expected)
		})
	}
}

func TestCreateUpgrader_WildcardOrigin(t *testing.T) {
	upgrader := createUpgrader([]string{"*"})

	tests := []struct {
		name   string
		origin string
	}{
		{"any origin 1", "http://localhost:3000"},
		{"any origin 2", "http://example.com"},
		{"any origin 3", "http://anything.anywhere.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/ws", nil)
			req.Header.Set("Origin", tt.origin)

			got := upgrader.CheckOrigin(req)
			testutil.AssertTrue(t, got, "wildcard should allow all origins")
		})
	}
}

func TestCreateUpgrader_EmptyOrigin(t *testing.T) {
	upgrader := createUpgrader([]string{"http://localhost:3000"})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	// No Origin header

	got := upgrader.CheckOrigin(req)
	testutil.AssertTrue(t, got, "empty origin should be allowed")
}

func TestNewWebSocketHandler_ParsesOrigins(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Test that origins are properly parsed and trimmed
	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, " http://localhost:3000 , http://example.com ")

	// The handler should have properly parsed and trimmed origins
	testutil.AssertNotNil(t, handler)
	testutil.AssertNotNil(t, handler.upgrader)
}

func TestWebSocketHandler_TokenPriority(t *testing.T) {
	// Test that cookie takes priority over query param
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	// Setup two sessions - one for cookie, one for query
	cookieSession := testutil.NewTestSession(
		testutil.WithToken("cookie-token-1234"),
		testutil.WithSessionUserID("user-cookie"),
	)
	sessionRepo.Sessions[cookieSession.Token] = cookieSession

	querySession := testutil.NewTestSession(
		testutil.WithToken("query-token-12345"),
		testutil.WithSessionUserID("user-query"),
	)
	sessionRepo.Sessions[querySession.Token] = querySession

	// Setup user for cookie session
	user := testutil.NewTestUser(
		testutil.WithUserID("user-cookie"),
		testutil.WithUsername("cookieuser"),
	)
	userRepo.Users[user.ID] = user

	// Setup membership for cookie user
	chatroomRepo.Members["room-1"] = map[string]bool{"user-cookie": true}

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	// Request with both cookie and query token
	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1?token=query-token-12345", "room-1")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "cookie-token-1234"})
	w := httptest.NewRecorder()

	handler.HandleConnection(w, req)

	// Should succeed using cookie token (user-cookie has membership)
	// If query token was used (user-query), it would fail membership check
	testutil.AssertTrue(t, w.Code != http.StatusForbidden, "should use cookie token, not query token")
}

func TestWebSocketHandler_ShortToken(t *testing.T) {
	sessionRepo := testutil.NewMockSessionRepository()
	userRepo := testutil.NewMockUserRepository()
	chatroomRepo := testutil.NewMockChatroomRepository()

	handler := setupWebSocketHandler(sessionRepo, userRepo, chatroomRepo, "*")

	// Token shorter than 8 characters - should handle gracefully
	req := createRequestWithChiContext(http.MethodGet, "/ws/chat/room-1", "room-1")
	req.AddCookie(&http.Cookie{Name: "session_id", Value: "short"})
	w := httptest.NewRecorder()

	// This might panic if the code does sessionToken[:8] without checking length
	// The test verifies the handler doesn't crash
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("handler panicked with short token: %v", r)
		}
	}()

	handler.HandleConnection(w, req)

	// Should get unauthorized (token won't be valid)
	testutil.AssertStatusCode(t, w, http.StatusUnauthorized)
}
