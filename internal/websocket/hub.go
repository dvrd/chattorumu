package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"jobsity-chat/internal/observability"
)

// BroadcastMessage represents a message to be sent to all clients in a chatroom.
type BroadcastMessage struct {
	ChatroomID string
	Message    []byte
}

// Hub maintains the set of active clients and broadcasts messages to them.
// All client map operations happen in the Run loop to avoid data races.
// The mutex is only used for read-only access from external goroutines
// (GetConnectedUserCount, GetAllConnectedCounts).
type Hub struct {
	// mutex protects read-only access to clients map from external goroutines.
	// Write access is only done in Run() loop, so no lock needed there.
	mutex sync.RWMutex

	// clients maps chatroom IDs to connected clients.
	// Only modified in Run() loop.
	clients map[string]map[*Client]bool

	// broadcast channel for sending messages to all clients in a chatroom.
	// Buffer of 256 allows burst handling without blocking senders.
	broadcast chan *BroadcastMessage

	// register channel for new client connections.
	register chan *Client

	// unregister channel for client disconnections.
	unregister chan *Client

	// userCountUpdate triggers sending user count updates to all clients.
	// Buffer of 10 prevents blocking on rapid connect/disconnect.
	userCountUpdate chan struct{}

	// done signals hub shutdown completion.
	done chan struct{}
}

// NewHub creates a new Hub instance.
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

// Run starts the hub's main event loop. It handles client registration,
// unregistration, broadcasts, and user count updates.
// All client map modifications happen here to avoid data races.
func (h *Hub) Run(ctx context.Context) error {
	defer h.shutdown()

	for {
		select {
		case <-ctx.Done():
			slog.Info("hub shutting down gracefully")
			return ctx.Err()

		case client := <-h.register:
			h.mutex.Lock()
			if h.clients[client.chatroomID] == nil {
				h.clients[client.chatroomID] = make(map[*Client]bool)
			}
			h.clients[client.chatroomID][client] = true
			h.mutex.Unlock()

			observability.WebSocketConnectionsActive.WithLabelValues(client.chatroomID).Inc()
			slog.Info("client registered",
				slog.String("user", client.username),
				slog.String("chatroom_id", client.chatroomID))

			select {
			case h.userCountUpdate <- struct{}{}:
			default:
			}

		case client := <-h.unregister:
			h.unregisterClient(client)

			select {
			case h.userCountUpdate <- struct{}{}:
			default:
			}

		case <-h.userCountUpdate:
			h.sendUserCountUpdate()

		case message := <-h.broadcast:
			h.mutex.RLock()
			clients, ok := h.clients[message.ChatroomID]
			h.mutex.RUnlock()

			if ok {
				var clientsToRemove []*Client
				for client := range clients {
					select {
					case client.send <- message.Message:
						observability.WebSocketMessagesSent.WithLabelValues(message.ChatroomID, "broadcast").Inc()
					default:
						clientsToRemove = append(clientsToRemove, client)
					}
				}
				// Remove clients with full send buffers
				if len(clientsToRemove) > 0 {
					h.mutex.Lock()
					for _, client := range clientsToRemove {
						client.closeSendOnce()
						delete(h.clients[message.ChatroomID], client)
					}
					h.mutex.Unlock()
				}
			}
		}
	}
}

func (h *Hub) unregisterClient(client *Client) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	clients, ok := h.clients[client.chatroomID]
	if !ok {
		return
	}

	if _, exists := clients[client]; !exists {
		return
	}

	delete(clients, client)
	client.closeSendOnce()
	observability.WebSocketConnectionsActive.WithLabelValues(client.chatroomID).Dec()
	slog.Info("client unregistered",
		slog.String("user", client.username),
		slog.String("chatroom_id", client.chatroomID))

	if len(clients) == 0 {
		delete(h.clients, client.chatroomID)
	}
}

func (h *Hub) shutdown() {
	close(h.done)

	h.mutex.Lock()
	defer h.mutex.Unlock()

	for chatroomID, clients := range h.clients {
		for client := range clients {
			client.closeSendOnce()
			slog.Info("closed client connection",
				slog.String("user", client.username),
				slog.String("chatroom_id", chatroomID))
		}
	}

	slog.Info("hub shutdown complete")
}

func (h *Hub) Broadcast(chatroomID string, message []byte) {
	h.broadcast <- &BroadcastMessage{
		ChatroomID: chatroomID,
		Message:    message,
	}
}

func (h *Hub) Register(client *Client) {
	h.register <- client
}

func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// GetConnectedUserCount returns the number of connected users in a chatroom.
// Thread-safe for external callers.
func (h *Hub) GetConnectedUserCount(chatroomID string) int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if clients, ok := h.clients[chatroomID]; ok {
		return len(clients)
	}
	return 0
}

// GetAllConnectedCounts returns user counts for all chatrooms.
// Thread-safe for external callers.
func (h *Hub) GetAllConnectedCounts() map[string]int {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	counts := make(map[string]int)
	for chatroomID, clients := range h.clients {
		counts[chatroomID] = len(clients)
	}
	return counts
}

// sendUserCountUpdate must only be called from within the Hub's Run loop
func (h *Hub) sendUserCountUpdate() {
	counts := make(map[string]int)
	for chatroomID, clients := range h.clients {
		counts[chatroomID] = len(clients)
	}

	message := map[string]any{
		"type":        "user_count_update",
		"user_counts": counts,
	}

	data, err := json.Marshal(message)
	if err != nil {
		slog.Error("failed to marshal user count update", slog.String("error", err.Error()))
		return
	}

	for chatroomID := range h.clients {
		if clients, ok := h.clients[chatroomID]; ok && len(clients) > 0 {
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
