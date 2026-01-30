package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/middleware"
	"jobsity-chat/internal/service"

	"github.com/go-chi/chi/v5"
)

// HubInterface defines the interface for getting connected user counts
type HubInterface interface {
	GetConnectedUserCount(chatroomID string) int
	GetAllConnectedCounts() map[string]int
}

// ChatroomHandler handles chatroom endpoints
type ChatroomHandler struct {
	chatService *service.ChatService
	hub         HubInterface
}

// NewChatroomHandler creates a new chatroom handler
func NewChatroomHandler(chatService *service.ChatService, hub HubInterface) *ChatroomHandler {
	return &ChatroomHandler{
		chatService: chatService,
		hub:         hub,
	}
}

// CreateChatroomRequest represents chatroom creation request
type CreateChatroomRequest struct {
	Name string `json:"name"`
}

// ChatroomResponse extends domain.Chatroom with connected user count
type ChatroomResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
	UserCount int    `json:"user_count"`
}

// List retrieves all chatrooms with connected user counts
func (h *ChatroomHandler) List(w http.ResponseWriter, r *http.Request) {
	chatrooms, err := h.chatService.ListChatrooms(r.Context())
	if err != nil {
		http.Error(w, `{"error":"Failed to retrieve chatrooms"}`, http.StatusInternalServerError)
		return
	}

	// Get connected user counts from hub
	connectedCounts := h.hub.GetAllConnectedCounts()

	// Build response with user counts
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
	json.NewEncoder(w).Encode(map[string]interface{}{
		"chatrooms": response,
	})
}

// Create creates a new chatroom
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
	json.NewEncoder(w).Encode(chatroom)
}

// GetMessages retrieves messages for a chatroom
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

	// Check membership
	isMember, err := h.chatService.IsMember(r.Context(), chatroomID, userID)
	if err != nil || !isMember {
		http.Error(w, `{"error":"Not a member of this chatroom"}`, http.StatusForbidden)
		return
	}

	// Get limit from query parameter with bounds checking
	const (
		minLimit     = 1
		maxLimit     = 100
		defaultLimit = 50
	)

	limit := defaultLimit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			if parsedLimit < minLimit {
				limit = minLimit
			} else if parsedLimit > maxLimit {
				limit = maxLimit
			} else {
				limit = parsedLimit
			}
		}
	}

	// Get messages - if 'before' timestamp is provided, load messages before that timestamp
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"messages": messages,
	})
}

// Join adds a user to a chatroom
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
