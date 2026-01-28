package websocket

import (
	"log"
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
}

// NewHub creates a new Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		broadcast:  make(chan *BroadcastMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			// Create chatroom map if it doesn't exist
			if h.clients[client.chatroomID] == nil {
				h.clients[client.chatroomID] = make(map[*Client]bool)
			}
			h.clients[client.chatroomID][client] = true
			log.Printf("Client registered: user=%s, chatroom=%s", client.username, client.chatroomID)

		case client := <-h.unregister:
			if clients, ok := h.clients[client.chatroomID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
					log.Printf("Client unregistered: user=%s, chatroom=%s", client.username, client.chatroomID)

					// Clean up empty chatroom
					if len(clients) == 0 {
						delete(h.clients, client.chatroomID)
					}
				}
			}

		case message := <-h.broadcast:
			// Send to all clients in the chatroom
			if clients, ok := h.clients[message.ChatroomID]; ok {
				for client := range clients {
					select {
					case client.send <- message.Message:
					default:
						// Client's send buffer is full, close connection
						close(client.send)
						delete(clients, client)
					}
				}
			}
		}
	}
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
