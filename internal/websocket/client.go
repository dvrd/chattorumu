package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
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
	writeMu     sync.Mutex         // Protects writes to conn
	closed      atomic.Bool        // Tracks connection state
	ctx         context.Context    // Client context for operations
	ctxCancel   context.CancelFunc // Cancel function for cleanup
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
	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, 256),
		userID:      userID,
		username:    username,
		chatroomID:  chatroomID,
		chatService: chatService,
		publisher:   publisher,
		ctx:         ctx,
		ctxCancel:   cancel,
	}
}

// ReadPump pumps messages from the WebSocket connection to the hub
func (c *Client) ReadPump() {
	defer func() {
		c.ctxCancel() // Cancel all ongoing operations
		c.hub.Unregister(c)
		c.closeConnection()

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
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		slog.Warn("failed to set read deadline",
			slog.String("error", err.Error()),
			slog.String("user", c.username),
			slog.String("chatroom", c.chatroomID))
		return
	}
	c.conn.SetPongHandler(func(string) error {
		if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
			slog.Warn("failed to set read deadline in pong handler",
				slog.String("error", err.Error()),
				slog.String("user", c.username),
				slog.String("chatroom", c.chatroomID))
			return err
		}
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
				slog.Warn("websocket error",
					slog.String("error", err.Error()),
					slog.String("user", c.username))
			}
			break
		}

		// Parse message
		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			slog.Warn("invalid message format",
				slog.String("error", err.Error()),
				slog.String("user", c.username))
			continue
		}

		// Check if command
		if cmd, isCommand := service.ParseCommand(clientMsg.Content); isCommand {
			// Publish stock command to RabbitMQ (don't save to database)
			func() {
				ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
				defer cancel()

				if err := c.publisher.PublishStockCommand(ctx, c.chatroomID, cmd.StockCode, c.username); err != nil {
					slog.Error("error publishing stock command",
						slog.String("error", err.Error()),
						slog.String("stock_code", cmd.StockCode),
						slog.String("user", c.username))

					// Send error message to client
					errorMsg := ServerMessage{
						Type:    "error",
						Message: "Failed to process stock command",
					}
					if data, err := json.Marshal(errorMsg); err == nil {
						c.send <- data
					}
				}
			}()
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

		ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
		if err := c.chatService.SendMessage(ctx, msg); err != nil {
			cancel()
			slog.Error("error saving message",
				slog.String("error", err.Error()),
				slog.String("user", c.username),
				slog.String("chatroom_id", c.chatroomID))
			continue
		}
		cancel()

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
		c.closeConnection()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Hub closed the channel
				_ = c.writeMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.writeMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			if err := c.writeMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// writeMessage writes a message to the WebSocket connection in a thread-safe manner
func (c *Client) writeMessage(messageType int, data []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	if c.closed.Load() {
		return websocket.ErrCloseSent
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		slog.Warn("failed to set write deadline",
			slog.String("error", err.Error()),
			slog.String("user", c.username),
			slog.String("chatroom", c.chatroomID))
		return err
	}
	return c.conn.WriteMessage(messageType, data)
}

// closeConnection safely closes the WebSocket connection
func (c *Client) closeConnection() {
	if c.closed.CompareAndSwap(false, true) {
		c.writeMu.Lock()
		c.conn.Close()
		c.writeMu.Unlock()
	}
}
