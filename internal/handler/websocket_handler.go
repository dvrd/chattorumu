package handler

import (
	"log"
	"net/http"

	"jobsity-chat/internal/middleware"
	"jobsity-chat/internal/service"
	ws "jobsity-chat/internal/websocket"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Allow all origins for development
		// In production, check against allowed origins
		return true
	},
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub         *ws.Hub
	chatService *service.ChatService
	authService *service.AuthService
	publisher   ws.MessagePublisher
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(hub *ws.Hub, chatService *service.ChatService, authService *service.AuthService, publisher ws.MessagePublisher) *WebSocketHandler {
	return &WebSocketHandler{
		hub:         hub,
		chatService: chatService,
		authService: authService,
		publisher:   publisher,
	}
}

// HandleConnection handles WebSocket upgrade and connection
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context (set by auth middleware)
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		http.Error(w, `{"error":"Not authenticated"}`, http.StatusUnauthorized)
		return
	}

	// Get chatroom ID from URL
	chatroomID := chi.URLParam(r, "chatroom_id")
	if chatroomID == "" {
		http.Error(w, `{"error":"Chatroom ID required"}`, http.StatusBadRequest)
		return
	}

	// Check if user is member of chatroom
	isMember, err := h.chatService.IsMember(r.Context(), chatroomID, userID)
	if err != nil || !isMember {
		http.Error(w, `{"error":"Not a member of this chatroom"}`, http.StatusForbidden)
		return
	}

	// Get user info
	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"User not found"}`, http.StatusUnauthorized)
		return
	}

	// Upgrade connection
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Create client
	client := ws.NewClient(h.hub, conn, userID, user.Username, chatroomID, h.chatService, h.publisher)

	// Register client with hub
	h.hub.Register(client)

	// Start client pumps
	go client.WritePump()
	go client.ReadPump()
}
