package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/service"
	ws "jobsity-chat/internal/websocket"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// truncateToken safely truncates a token for logging purposes
func truncateToken(token string) string {
	if len(token) <= 8 {
		return token + "..."
	}
	return token[:8] + "..."
}

func createUpgrader(allowedOrigins []string) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}

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

type WebSocketHandler struct {
	hub         *ws.Hub
	chatService *service.ChatService
	authService *service.AuthService
	publisher   ws.MessagePublisher
	upgrader    websocket.Upgrader
	sessionRepo domain.SessionRepository
}

func NewWebSocketHandler(hub *ws.Hub, chatService *service.ChatService, authService *service.AuthService, publisher ws.MessagePublisher, sessionRepo domain.SessionRepository, allowedOrigins string) *WebSocketHandler {
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

func (h *WebSocketHandler) HandleConnection(w http.ResponseWriter, r *http.Request) {
	var sessionToken string

	if cookie, err := r.Cookie("session_id"); err == nil {
		sessionToken = cookie.Value
	}

	if sessionToken == "" {
		sessionToken = r.URL.Query().Get("token")
	}

	if sessionToken == "" {
		auth := r.Header.Get("Authorization")
		if token, ok := strings.CutPrefix(auth, "Bearer "); ok {
			sessionToken = token
		}
	}

	if sessionToken == "" {
		slog.Warn("websocket auth failed: no token",
			slog.String("remote_addr", r.RemoteAddr),
			slog.String("chatroom_id", chi.URLParam(r, "chatroom_id")))
		http.Error(w, `{"error":"No session token provided"}`, http.StatusUnauthorized)
		return
	}

	slog.Debug("websocket auth attempt",
		slog.String("token", truncateToken(sessionToken)),
		slog.String("chatroom_id", chi.URLParam(r, "chatroom_id")))

	session, err := h.sessionRepo.GetByToken(r.Context(), sessionToken)
	if err != nil {
		slog.Warn("websocket auth failed: invalid session",
			slog.String("error", err.Error()),
			slog.String("token_prefix", truncateToken(sessionToken)),
			slog.String("remote_addr", r.RemoteAddr))
		http.Error(w, `{"error":"Invalid or expired session"}`, http.StatusUnauthorized)
		return
	}

	slog.Info("websocket auth successful",
		slog.String("user_id", session.UserID),
		slog.String("chatroom_id", chi.URLParam(r, "chatroom_id")))

	userID := session.UserID

	chatroomID := chi.URLParam(r, "chatroom_id")
	if chatroomID == "" {
		http.Error(w, `{"error":"Chatroom ID required"}`, http.StatusBadRequest)
		return
	}

	isMember, err := h.chatService.IsMember(r.Context(), chatroomID, userID)
	if err != nil || !isMember {
		http.Error(w, `{"error":"Not a member of this chatroom"}`, http.StatusForbidden)
		return
	}

	user, err := h.authService.GetUserByID(r.Context(), userID)
	if err != nil {
		http.Error(w, `{"error":"User not found"}`, http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("websocket upgrade error",
			slog.String("error", err.Error()),
			slog.String("user_id", userID),
			slog.String("chatroom_id", chatroomID))
		return
	}

	// Use background context for the client since the HTTP request context
	// will be cancelled after the upgrade completes
	client := ws.NewClient(context.Background(), h.hub, conn, userID, user.Username, chatroomID, h.chatService, h.publisher)

	h.hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
