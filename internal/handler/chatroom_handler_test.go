package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/middleware"

	"github.com/go-chi/chi/v5"
)

// mockChatService implements service.ChatService interface for testing
type mockChatService struct {
	createChatroomFunc    func(ctx context.Context, name, createdBy string) (*domain.Chatroom, error)
	listChatroomsFunc     func(ctx context.Context) ([]*domain.Chatroom, error)
	joinChatroomFunc      func(ctx context.Context, chatroomID, userID string) error
	isMemberFunc          func(ctx context.Context, chatroomID, userID string) (bool, error)
	getMessagesFunc       func(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error)
	getMessagesBeforeFunc func(ctx context.Context, chatroomID, before string, limit int) ([]*domain.Message, error)
}

func (m *mockChatService) CreateChatroom(ctx context.Context, name, createdBy string) (*domain.Chatroom, error) {
	if m.createChatroomFunc != nil {
		return m.createChatroomFunc(ctx, name, createdBy)
	}
	return nil, errors.New("not implemented")
}

func (m *mockChatService) ListChatrooms(ctx context.Context) ([]*domain.Chatroom, error) {
	if m.listChatroomsFunc != nil {
		return m.listChatroomsFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockChatService) JoinChatroom(ctx context.Context, chatroomID, userID string) error {
	if m.joinChatroomFunc != nil {
		return m.joinChatroomFunc(ctx, chatroomID, userID)
	}
	return errors.New("not implemented")
}

func (m *mockChatService) IsMember(ctx context.Context, chatroomID, userID string) (bool, error) {
	if m.isMemberFunc != nil {
		return m.isMemberFunc(ctx, chatroomID, userID)
	}
	return false, errors.New("not implemented")
}

func (m *mockChatService) GetMessages(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
	if m.getMessagesFunc != nil {
		return m.getMessagesFunc(ctx, chatroomID, limit)
	}
	return nil, errors.New("not implemented")
}

func (m *mockChatService) GetMessagesBefore(ctx context.Context, chatroomID, before string, limit int) ([]*domain.Message, error) {
	if m.getMessagesBeforeFunc != nil {
		return m.getMessagesBeforeFunc(ctx, chatroomID, before, limit)
	}
	return nil, errors.New("not implemented")
}

func (m *mockChatService) SendMessage(ctx context.Context, message *domain.Message) error {
	return errors.New("not implemented")
}

// mockHub implements HubInterface for testing
type mockHub struct {
	connectedCounts map[string]int
}

func (m *mockHub) GetConnectedUserCount(chatroomID string) int {
	return m.connectedCounts[chatroomID]
}

func (m *mockHub) GetAllConnectedCounts() map[string]int {
	return m.connectedCounts
}

func TestChatroomHandler_List_Success(t *testing.T) {
	now := time.Now()

	chatService := &mockChatService{
		listChatroomsFunc: func(ctx context.Context) ([]*domain.Chatroom, error) {
			return []*domain.Chatroom{
				{
					ID:        "room-1",
					Name:      "General",
					CreatedBy: "user-1",
					CreatedAt: now,
				},
				{
					ID:        "room-2",
					Name:      "Random",
					CreatedBy: "user-2",
					CreatedAt: now,
				},
			}, nil
		},
	}

	hub := &mockHub{
		connectedCounts: map[string]int{
			"room-1": 5,
			"room-2": 3,
		},
	}

	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chatrooms", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string][]ChatroomResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	rooms := resp["chatrooms"]
	if len(rooms) != 2 {
		t.Errorf("expected 2 chatrooms, got %d", len(rooms))
	}

	if rooms[0].ID != "room-1" || rooms[0].UserCount != 5 {
		t.Errorf("expected room-1 with 5 users, got %s with %d users", rooms[0].ID, rooms[0].UserCount)
	}

	if rooms[1].ID != "room-2" || rooms[1].UserCount != 3 {
		t.Errorf("expected room-2 with 3 users, got %s with %d users", rooms[1].ID, rooms[1].UserCount)
	}
}

func TestChatroomHandler_List_ServiceError(t *testing.T) {
	chatService := &mockChatService{
		listChatroomsFunc: func(ctx context.Context) ([]*domain.Chatroom, error) {
			return nil, errors.New("database error")
		},
	}

	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chatrooms", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestChatroomHandler_Create_Success(t *testing.T) {
	now := time.Now()

	chatService := &mockChatService{
		createChatroomFunc: func(ctx context.Context, name, createdBy string) (*domain.Chatroom, error) {
			return &domain.Chatroom{
				ID:        "room-123",
				Name:      name,
				CreatedBy: createdBy,
				CreatedAt: now,
			}, nil
		},
	}

	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	reqBody := `{"name":"Test Room"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	// Add user ID to context
	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d, body: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp domain.Chatroom
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "room-123" {
		t.Errorf("expected ID 'room-123', got '%s'", resp.ID)
	}
	if resp.Name != "Test Room" {
		t.Errorf("expected name 'Test Room', got '%s'", resp.Name)
	}
}

func TestChatroomHandler_Create_NoUserID(t *testing.T) {
	chatService := &mockChatService{}
	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	reqBody := `{"name":"Test Room"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestChatroomHandler_Create_InvalidJSON(t *testing.T) {
	chatService := &mockChatService{}
	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", strings.NewReader(`invalid json`))
	req.Header.Set("Content-Type", "application/json")

	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestChatroomHandler_Create_EmptyName(t *testing.T) {
	chatService := &mockChatService{
		createChatroomFunc: func(ctx context.Context, name, createdBy string) (*domain.Chatroom, error) {
			return nil, errors.New("chatroom name is required")
		},
	}

	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	reqBody := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestChatroomHandler_GetMessages_Success(t *testing.T) {
	now := time.Now()

	chatService := &mockChatService{
		isMemberFunc: func(ctx context.Context, chatroomID, userID string) (bool, error) {
			return true, nil
		},
		getMessagesFunc: func(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
			return []*domain.Message{
				{
					ID:         "msg-1",
					ChatroomID: chatroomID,
					UserID:     "user-1",
					Username:   "alice",
					Content:    "Hello",
					CreatedAt:  now,
				},
				{
					ID:         "msg-2",
					ChatroomID: chatroomID,
					UserID:     "user-2",
					Username:   "bob",
					Content:    "Hi",
					CreatedAt:  now,
				},
			}, nil
		},
	}

	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chatrooms/room-1/messages", nil)

	// Set up chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "room-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Add user ID to context
	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	handler.GetMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d, body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string][]*domain.Message
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	messages := resp["messages"]
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestChatroomHandler_GetMessages_WithBefore(t *testing.T) {
	now := time.Now()

	chatService := &mockChatService{
		isMemberFunc: func(ctx context.Context, chatroomID, userID string) (bool, error) {
			return true, nil
		},
		getMessagesBeforeFunc: func(ctx context.Context, chatroomID, before string, limit int) ([]*domain.Message, error) {
			return []*domain.Message{
				{
					ID:         "msg-1",
					ChatroomID: chatroomID,
					UserID:     "user-1",
					Username:   "alice",
					Content:    "Older message",
					CreatedAt:  now.Add(-1 * time.Hour),
				},
			}, nil
		},
	}

	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chatrooms/room-1/messages?before="+now.Format(time.RFC3339), nil)

	// Set up chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "room-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Add user ID to context
	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	handler.GetMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp map[string][]*domain.Message
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	messages := resp["messages"]
	if len(messages) != 1 {
		t.Errorf("expected 1 message, got %d", len(messages))
	}
}

func TestChatroomHandler_GetMessages_LimitValidation(t *testing.T) {
	tests := []struct {
		name          string
		limitParam    string
		expectedLimit int
	}{
		{"no limit uses default", "", 50},
		{"valid limit", "25", 25},
		{"limit below min clamped to min", "0", 1},
		{"limit above max clamped to max", "200", 100},
		{"invalid limit uses default", "abc", 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedLimit int

			chatService := &mockChatService{
				isMemberFunc: func(ctx context.Context, chatroomID, userID string) (bool, error) {
					return true, nil
				},
				getMessagesFunc: func(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
					capturedLimit = limit
					return []*domain.Message{}, nil
				},
			}

			hub := &mockHub{connectedCounts: make(map[string]int)}
			handler := NewChatroomHandler(chatService, hub)

			url := "/api/v1/chatrooms/room-1/messages"
			if tt.limitParam != "" {
				url += "?limit=" + tt.limitParam
			}

			req := httptest.NewRequest(http.MethodGet, url, nil)

			// Set up chi context
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", "room-1")
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			// Add user ID to context
			ctx := middleware.WithUserID(req.Context(), "user-123")
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			handler.GetMessages(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
			}

			if capturedLimit != tt.expectedLimit {
				t.Errorf("expected limit %d, got %d", tt.expectedLimit, capturedLimit)
			}
		})
	}
}

func TestChatroomHandler_GetMessages_NoUserID(t *testing.T) {
	chatService := &mockChatService{}
	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chatrooms/room-1/messages", nil)
	w := httptest.NewRecorder()

	handler.GetMessages(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestChatroomHandler_GetMessages_NotMember(t *testing.T) {
	chatService := &mockChatService{
		isMemberFunc: func(ctx context.Context, chatroomID, userID string) (bool, error) {
			return false, nil
		},
	}

	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chatrooms/room-1/messages", nil)

	// Set up chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "room-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Add user ID to context
	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	handler.GetMessages(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}

func TestChatroomHandler_Join_Success(t *testing.T) {
	chatService := &mockChatService{
		joinChatroomFunc: func(ctx context.Context, chatroomID, userID string) error {
			return nil
		},
	}

	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms/room-1/join", nil)

	// Set up chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "room-1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Add user ID to context
	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	handler.Join(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d, body: %s", http.StatusOK, w.Code, w.Body.String())
	}

	var resp map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp["success"] {
		t.Error("expected success to be true")
	}
}

func TestChatroomHandler_Join_NoUserID(t *testing.T) {
	chatService := &mockChatService{}
	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms/room-1/join", nil)
	w := httptest.NewRecorder()

	handler.Join(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestChatroomHandler_Join_ChatroomNotFound(t *testing.T) {
	chatService := &mockChatService{
		joinChatroomFunc: func(ctx context.Context, chatroomID, userID string) error {
			return errors.New("chatroom not found")
		},
	}

	hub := &mockHub{connectedCounts: make(map[string]int)}
	handler := NewChatroomHandler(chatService, hub)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/chatrooms/room-999/join", nil)

	// Set up chi context
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "room-999")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	// Add user ID to context
	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	handler.Join(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
