package websocket

import (
	"context"
	"encoding/json"
	"log/slog"

	"jobsity-chat/internal/observability"
)

type BroadcastMessage struct {
	ChatroomID string
	Message    []byte
}

type Hub struct {
	clients         map[string]map[*Client]bool
	broadcast       chan *BroadcastMessage
	register        chan *Client
	unregister      chan *Client
	userCountUpdate chan struct{}
	done            chan struct{}
}

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

func (h *Hub) Run(ctx context.Context) error {
	defer h.shutdown()

	for {
		select {
		case <-ctx.Done():
			slog.Info("hub shutting down gracefully")
			return ctx.Err()

		case client := <-h.register:
			if h.clients[client.chatroomID] == nil {
				h.clients[client.chatroomID] = make(map[*Client]bool)
			}
			h.clients[client.chatroomID][client] = true
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
			if clients, ok := h.clients[message.ChatroomID]; ok {
				for client := range clients {
					select {
					case client.send <- message.Message:
						observability.WebSocketMessagesSent.WithLabelValues(message.ChatroomID, "broadcast").Inc()
					default:
						h.closeClientSend(client)
						delete(clients, client)
					}
				}
			}
		}
	}
}

func (h *Hub) unregisterClient(client *Client) {
	if clients, ok := h.clients[client.chatroomID]; ok {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			h.closeClientSend(client)
			observability.WebSocketConnectionsActive.WithLabelValues(client.chatroomID).Dec()
			slog.Info("client unregistered",
				slog.String("user", client.username),
				slog.String("chatroom_id", client.chatroomID))

			if len(clients) == 0 {
				delete(h.clients, client.chatroomID)
			}
		}
	}
}

func (h *Hub) closeClientSend(client *Client) {
	select {
	case <-client.send:
	default:
		close(client.send)
	}
}

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

func (h *Hub) GetConnectedUserCount(chatroomID string) int {
	if clients, ok := h.clients[chatroomID]; ok {
		return len(clients)
	}
	return 0
}

func (h *Hub) GetAllConnectedCounts() map[string]int {
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
