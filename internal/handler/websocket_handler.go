package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/service"
	ws "jobsity-chat/internal/websocket"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// createUpgrader creates a WebSocket upgrader with origin checking
func createUpgrader(allowedOrigins []string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				// No origin header (non-browser clients)
				return true
			}

			// Check if origin is in allowed list
			for _, allowed := range allowedOrigins {
				if origin == allowed || allowed == "*" {
					return true
				}
			}

			slog.Warn("websocket origin rejected",
				slog.String("origin", origin),
				slog.String("remote_addr", r.RemoteAddr))
			return false
		},
	}
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	hub         *ws.Hub
	chatService *service.ChatService
	authService *service.AuthService
	publisher   ws.MessagePublisher
	upgrader    websocket.Upgrader
	sessionRepo domain.SessionRepository
}

// NewWebSocketHandler creates a new WebSocket handler with CORS-aware origin checking
func NewWebSocketHandler(hub *ws.Hub, chatService *service.ChatService, authService *service.AuthService, publisher ws.MessagePublisher, sessionRepo domain.SessionRepository, allowedOrigins string) *WebSocketHandler {
	// Parse allowed origins from comma-separated string
	origins := strings.Split(allowedOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return &WebSocketHandler{
		hub:         hub,
		chatService: chatService,
		authService: authService,
		publisher:   publisher,
		sessionRepo: sessionRepo,
		upgrader:    createUpgrader(origins),
	}
}

// HandleConnection handles WebSocket upgrade and connection
func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	// Try to get session token from multiple sources
	var sessionToken string

	// 1. Try cookie first (standard browser behavior)
	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	// 2. Fallback to query parameter (for browsers that don't send cookies with WebSocket)
	if sessionToken == "" {
		sessionToken = r.URL.Query().Get("token")
	}

	// 3. Check Authorization header as last resort
	if sessionToken == "" {
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			sessionToken = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	if sessionToken == "" {
		http.Error(w, `{"error":"No session token provided"}`, http.StatusUnauthorized)
		return
	}

	// Validate session
	session, err := h.sessionRepo.GetByToken(r.Context(), sessionToken)
	if err != nil {
		http.Error(w, `{"error":"Invalid or expired session"}`, http.StatusUnauthorized)
		return
	}

	userID := session.UserID

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
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade error",
			slog.String("error", err.Error()),
			slog.String("user_id", userID),
			slog.String("chatroom_id", chatroomID))
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
