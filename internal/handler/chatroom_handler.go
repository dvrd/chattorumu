package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/middleware"

	"github.com/go-chi/chi/v5"
)

type HubInterface interface {
	GetConnectedUserCount(chatroomID string) int
	GetAllConnectedCounts() map[string]int
}

type ChatServiceInterface interface {
	CreateChatroom(ctx context.Context, name, createdBy string) (*domain.Chatroom, error)
	ListChatrooms(ctx context.Context) ([]*domain.Chatroom, error)
	ListChatroomsPaginated(ctx context.Context, limit int, cursor string) ([]*domain.Chatroom, string, error)
	JoinChatroom(ctx context.Context, chatroomID, userID string) error
	IsMember(ctx context.Context, chatroomID, userID string) (bool, error)
	GetMessages(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error)
	GetMessagesBefore(ctx context.Context, chatroomID, before string, limit int) ([]*domain.Message, error)
	SendMessage(ctx context.Context, message *domain.Message) error
}

type ChatroomHandler struct {
	chatService ChatServiceInterface
	hub         HubInterface
}

func NewChatroomHandler(chatService ChatServiceInterface, hub HubInterface) *ChatroomHandler {
	return &ChatroomHandler{
		chatService: chatService,
		hub:         hub,
	}
}

type CreateChatroomRequest struct {
	Name string `json:"name"`
}

type ChatroomResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
	UserCount int    `json:"user_count"`
}

func (h *ChatroomHandler) List(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("cursor")
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	chatrooms, nextCursor, err := h.chatService.ListChatroomsPaginated(r.Context(), limit, cursor)
	if err != nil {
		http.Error(w, `{"error":"Failed to retrieve chatrooms"}`, http.StatusInternalServerError)
		return
	}

	connectedCounts := h.hub.GetAllConnectedCounts()

	response := make([]ChatroomResponse, len(chatrooms))
	for i, room := range chatrooms {
		response[i] = ChatroomResponse{
			ID:        room.ID,
			Name:      room.Name,
			CreatedAt: room.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			CreatedBy: room.CreatedBy,
			UserCount: connectedCounts[room.ID],
		}
	}

	w.Header().Set("Content-Type", "application/json")
	responseData := map[string]any{
		"chatrooms": response,
	}
	if nextCursor != "" {
		responseData["next_cursor"] = nextCursor
	}
	if err := json.NewEncoder(w).Encode(responseData); err != nil {
		slog.Error("failed to encode list chatrooms response", slog.String("error", err.Error()))
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *ChatroomHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, `{"error":"User not authenticated"}`, http.StatusUnauthorized)
		return
	}

	var req CreateChatroomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	chatroom, err := h.chatService.CreateChatroom(r.Context(), req.Name, userID)
	if err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(chatroom); err != nil {
		slog.Error("failed to encode create chatroom response", slog.String("error", err.Error()))
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *ChatroomHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, `{"error":"User not authenticated"}`, http.StatusUnauthorized)
		return
	}

	chatroomID := chi.URLParam(r, "id")
	if chatroomID == "" {
		http.Error(w, `{"error":"Chatroom ID required"}`, http.StatusBadRequest)
		return
	}

	isMember, err := h.chatService.IsMember(r.Context(), chatroomID, userID)
	if err != nil || !isMember {
		http.Error(w, `{"error":"Not a member of this chatroom"}`, http.StatusForbidden)
		return
	}

	const (
		minLimit     = 1
		maxLimit     = 100
		defaultLimit = 50
	)

	limit := defaultLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, parseErr := strconv.Atoi(limitStr); parseErr == nil {
			switch {
			case parsedLimit < minLimit:
				limit = minLimit
			case parsedLimit > maxLimit:
				limit = maxLimit
			default:
				limit = parsedLimit
			}
		}
	}

	var messages []*domain.Message
	before := r.URL.Query().Get("before")
	if before != "" {
		messages, err = h.chatService.GetMessagesBefore(r.Context(), chatroomID, before, limit)
	} else {
		messages, err = h.chatService.GetMessages(r.Context(), chatroomID, limit)
	}

	if err != nil {
		http.Error(w, `{"error":"Failed to retrieve messages"}`, http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]any{
		"messages": messages,
	}); err != nil {
		slog.Error("failed to encode get messages response", slog.String("error", err.Error()))
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *ChatroomHandler) Join(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, `{"error":"User not authenticated"}`, http.StatusUnauthorized)
		return
	}

	chatroomID := chi.URLParam(r, "id")
	if chatroomID == "" {
		http.Error(w, `{"error":"Chatroom ID required"}`, http.StatusBadRequest)
		return
	}

	if err := h.chatService.JoinChatroom(r.Context(), chatroomID, userID); err != nil {
		http.Error(w, `{"error":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]bool{"success": true}); err != nil {
		slog.Error("failed to encode join chatroom response", slog.String("error", err.Error()))
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}
