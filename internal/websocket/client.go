package websocket

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/service"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = 54 * time.Second

	// Maximum message size allowed from peer
	maxMessageSize = 1024
)

// Client represents a WebSocket client
type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	userID      string
	username    string
	chatroomID  string
	chatService *service.ChatService
	publisher   MessagePublisher
}

// MessagePublisher defines the interface for publishing messages to RabbitMQ
type MessagePublisher interface {
	PublishStockCommand(ctx context.Context, chatroomID, stockCode, requestedBy string) error
}

// ClientMessage represents a message from the client
type ClientMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// ServerMessage represents a message to the client
type ServerMessage struct {
	Type      string    `json:"type"`
	ID        string    `json:"id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Username  string    `json:"username,omitempty"`
	Content   string    `json:"content,omitempty"`
	IsBot     bool      `json:"is_bot,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	Message   string    `json:"message,omitempty"`
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, userID, username, chatroomID string,
	chatService *service.ChatService, publisher MessagePublisher) *Client {
	return &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, 256),
		userID:      userID,
		username:    username,
		chatroomID:  chatroomID,
		chatService: chatService,
		publisher:   publisher,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()

		// Broadcast user left message
		leftMsg := ServerMessage{
			Type:     "user_left",
			Username: c.username,
		}
		if data, err := json.Marshal(leftMsg); err == nil {
			c.hub.Broadcast(c.chatroomID, data)
		}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Send user joined message
	joinedMsg := ServerMessage{
		Type:     "user_joined",
		Username: c.username,
	}
	if data, err := json.Marshal(joinedMsg); err == nil {
		c.hub.Broadcast(c.chatroomID, data)
	}

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Parse message
		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			log.Printf("Invalid message format: %v", err)
			continue
		}

		// Check if command
		if cmd, isCommand := service.ParseCommand(clientMsg.Content); isCommand {
			// Publish stock command to RabbitMQ (don't save to database)
			ctx := context.Background()
			if err := c.publisher.PublishStockCommand(ctx, c.chatroomID, cmd.StockCode, c.username); err != nil {
				log.Printf("Error publishing stock command: %v", err)

				// Send error message to client
				errorMsg := ServerMessage{
					Type:    "error",
					Message: "Failed to process stock command",
				}
				if data, err := json.Marshal(errorMsg); err == nil {
					c.send <- data
				}
			}
			continue
		}

		// Save regular message to database
		msg := &domain.Message{
			ChatroomID: c.chatroomID,
			UserID:     c.userID,
			Username:   c.username,
			Content:    clientMsg.Content,
			IsBot:      false,
		}

		ctx := context.Background()
		if err := c.chatService.SendMessage(ctx, msg); err != nil {
			log.Printf("Error saving message: %v", err)
			continue
		}

		// Broadcast to all clients in chatroom
		serverMsg := ServerMessage{
			Type:      "chat_message",
			ID:        msg.ID,
			UserID:    msg.UserID,
			Username:  msg.Username,
			Content:   msg.Content,
			IsBot:     msg.IsBot,
			CreatedAt: msg.CreatedAt,
		}

		if data, err := json.Marshal(serverMsg); err == nil {
			c.hub.Broadcast(c.chatroomID, data)
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
