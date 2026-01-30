package websocket

import (
	"context"
	"encoding/json"
	"log/slog"

	"jobsity-chat/internal/observability"
)

// BroadcastMessage represents a message to be broadcast
type BroadcastMessage struct {
	ChatroomID string
	Message    []byte
}

// Hub maintains active clients and broadcasts messages
type Hub struct {
	// Registered clients by chatroom
	clients map[string]map[*Client]bool

	// Broadcast channel
	broadcast chan *BroadcastMessage

	// Register client
	register chan *Client

	// Unregister client
	unregister chan *Client

	// User count update channel
	userCountUpdate chan struct{}

	// Shutdown signal
	done chan struct{}
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		clients:         make(map[string]map[*Client]bool),
		broadcast:       make(chan *BroadcastMessage, 256),
		register:        make(chan *Client),
		unregister:      make(chan *Client),
		userCountUpdate: make(chan struct{}, 10),
		done:            make(chan struct{}),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run(ctx context.Context) error {
	defer h.shutdown()

	for {
		select {
		case <-ctx.Done():
			slog.Info("hub shutting down gracefully")
			return ctx.Err()

		case client := <-h.register:
			// Create chatroom map if it doesn't exist
			if h.clients[client.chatroomID] == nil {
				h.clients[client.chatroomID] = make(map[*Client]bool)
			}
			h.clients[client.chatroomID][client] = true
			observability.WebSocketConnectionsActive.WithLabelValues(client.chatroomID).Inc()
			slog.Info("client registered",
				slog.String("user", client.username),
				slog.String("chatroom_id", client.chatroomID))

			// Signal user count update (non-blocking)
			select {
			case h.userCountUpdate <- struct{}{}:
			default:
				// Channel full, update already pending
			}

		case client := <-h.unregister:
			h.unregisterClient(client)

			// Signal user count update (non-blocking)
			select {
			case h.userCountUpdate <- struct{}{}:
			default:
				// Channel full, update already pending
			}

		case <-h.userCountUpdate:
			// Send user count update to all clients
			h.sendUserCountUpdate()

		case message := <-h.broadcast:
			// Send to all clients in the chatroom
			if clients, ok := h.clients[message.ChatroomID]; ok {
				for client := range clients {
					select {
					case client.send <- message.Message:
						observability.WebSocketMessagesSent.WithLabelValues(message.ChatroomID, "broadcast").Inc()
					default:
						// Client's send buffer is full, close connection
						h.closeClientSend(client)
						delete(clients, client)
					}
				}
			}
		}
	}
}

// unregisterClient safely removes a client from the hub
func (h *Hub) unregisterClient(client *Client) {
	if clients, ok := h.clients[client.chatroomID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			h.closeClientSend(client)
			observability.WebSocketConnectionsActive.WithLabelValues(client.chatroomID).Dec()
			slog.Info("client unregistered",
				slog.String("user", client.username),
				slog.String("chatroom_id", client.chatroomID))

			// Clean up empty chatroom
			if len(clients) == 0 {
				delete(h.clients, client.chatroomID)
			}
		}
	}
}

// closeClientSend safely closes a client's send channel
func (h *Hub) closeClientSend(client *Client) {
	select {
	case <-client.send:
		// Channel already closed
	default:
		close(client.send)
	}
}

// shutdown performs graceful cleanup of all connections
func (h *Hub) shutdown() {
	close(h.done)

	for chatroomID, clients := range h.clients {
		for client := range clients {
			h.closeClientSend(client)
			slog.Info("closed client connection",
				slog.String("user", client.username),
				slog.String("chatroom_id", chatroomID))
		}
	}

	slog.Info("hub shutdown complete")
}

// Broadcast sends a message to all clients in a chatroom
func (h *Hub) Broadcast(chatroomID string, message []byte) {
	h.broadcast <- &BroadcastMessage{
		ChatroomID: chatroomID,
		Message:    message,
	}
}

// Register registers a client with the hub
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// GetConnectedUserCount returns the number of connected users in a chatroom
func (h *Hub) GetConnectedUserCount(chatroomID string) int {
	if clients, ok := h.clients[chatroomID]; ok {
		return len(clients)
	}
	return 0
}

// GetAllConnectedCounts returns a map of chatroom IDs to their connected user counts
func (h *Hub) GetAllConnectedCounts() map[string]int {
	counts := make(map[string]int)
	for chatroomID, clients := range h.clients {
		counts[chatroomID] = len(clients)
	}
	return counts
}

// sendUserCountUpdate sends user count updates to all connected clients
// This method should only be called from within the Hub's Run loop
func (h *Hub) sendUserCountUpdate() {
	counts := make(map[string]int)
	for chatroomID, clients := range h.clients {
		counts[chatroomID] = len(clients)
	}

	// Marshal the counts to JSON
	message := map[string]interface{}{
		"type":        "user_count_update",
		"user_counts": counts,
	}

	data, err := json.Marshal(message)
	if err != nil {
		slog.Error("failed to marshal user count update", slog.String("error", err.Error()))
		return
	}

	// Broadcast to all chatrooms
	for chatroomID := range h.clients {
		if clients, ok := h.clients[chatroomID]; ok && len(clients) > 0 {
			// Send using the broadcast channel (non-blocking)
			select {
			case h.broadcast <- &BroadcastMessage{
				ChatroomID: chatroomID,
				Message:    data,
			}:
			default:
				slog.Warn("broadcast channel full, skipping user count update",
					slog.String("chatroom_id", chatroomID))
			}
		}
	}
}
