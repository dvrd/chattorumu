package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"jobsity-chat/internal/middleware"
	"jobsity-chat/internal/service"

	"github.com/go-chi/chi/v5"
)

// ChatroomHandler handles chatroom endpoints
type ChatroomHandler struct {
	chatService *service.ChatService
}

// NewChatroomHandler creates a new chatroom handler
func NewChatroomHandler(chatService *service.ChatService) *ChatroomHandler {
	return &ChatroomHandler{
		chatService: chatService,
	}
}

// CreateChatroomRequest represents chatroom creation request
type CreateChatroomRequest struct {
	Name string `json:"name"`
}

// List retrieves all chatrooms
func (h *ChatroomHandler) List(w http.ResponseWriter, r *http.Request) {
	chatrooms, err := h.chatService.ListChatrooms(r.Context())
	if err != nil {
		http.Error(w, `{"error":"Failed to retrieve chatrooms"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"chatrooms": chatrooms,
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

	// Get limit from query parameter
	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil {
			limit = parsedLimit
		}
	}

	messages, err := h.chatService.GetMessages(r.Context(), chatroomID, limit)
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
