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
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 54 * time.Second // Must be less than pongWait
	maxMessageSize = 1024
)

type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	userID      string
	username    string
	chatroomID  string
	chatService *service.ChatService
	publisher   MessagePublisher
	writeMu     sync.Mutex
	closed      atomic.Bool
	ctx         context.Context
	ctxCancel   context.CancelFunc
}

type MessagePublisher interface {
	PublishStockCommand(ctx context.Context, chatroomID, stockCode, requestedBy string) error
	PublishHelloCommand(ctx context.Context, chatroomID, requestedBy string) error
}

type ClientMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type ServerMessage struct {
	Type      string     `json:"type"`
	ID        string     `json:"id,omitempty"`
	UserID    string     `json:"user_id,omitempty"`
	Username  string     `json:"username,omitempty"`
	Content   string     `json:"content,omitempty"`
	IsBot     bool       `json:"is_bot,omitempty"`
	IsError   bool       `json:"is_error,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
	Message   string     `json:"message,omitempty"`
}

func NewClient(ctx context.Context, hub *Hub, conn *websocket.Conn, userID, username, chatroomID string,
	chatService *service.ChatService, publisher MessagePublisher) *Client {
	clientCtx, cancel := context.WithCancel(ctx)

	return &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, 256),
		userID:      userID,
		username:    username,
		chatroomID:  chatroomID,
		chatService: chatService,
		publisher:   publisher,
		ctx:         clientCtx,
		ctxCancel:   cancel,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.ctxCancel()
		c.hub.Unregister(c)
		c.closeConnection()

		leftMsg := ServerMessage{
			Type:     "user_left",
			Username: c.username,
		}
		data, err := json.Marshal(leftMsg)
		if err != nil {
			slog.Error("failed to marshal user left message",
				slog.String("error", err.Error()),
				slog.String("username", c.username))
		} else {
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

	joinedMsg := ServerMessage{
		Type:     "user_joined",
		Username: c.username,
	}
	data, err := json.Marshal(joinedMsg)
	if err != nil {
		slog.Error("failed to marshal user joined message",
			slog.String("error", err.Error()),
			slog.String("username", c.username))
	} else {
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

		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			slog.Warn("invalid message format",
				slog.String("error", err.Error()),
				slog.String("user", c.username))
			continue
		}

		if cmd, isCommand := service.ParseCommand(clientMsg.Content); isCommand {
			func() {
				ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
				defer cancel()

				var err error
				switch cmd.Type {
				case "stock":
					err = c.publisher.PublishStockCommand(ctx, c.chatroomID, cmd.StockCode, c.username)
				case "hello":
					err = c.publisher.PublishHelloCommand(ctx, c.chatroomID, c.username)
				default:
					slog.Warn("unknown command type",
						slog.String("type", cmd.Type),
						slog.String("user", c.username))
					return
				}

				if err != nil {
					slog.Error("error publishing command",
						slog.String("error", err.Error()),
						slog.String("type", cmd.Type),
						slog.String("user", c.username))

					// Send error message to client
					errorMsg := ServerMessage{
						Type:    "error",
						Message: "Failed to process command",
					}
					data, marshalErr := json.Marshal(errorMsg)
					if marshalErr != nil {
						slog.Error("failed to marshal error message",
							slog.String("error", marshalErr.Error()))
					} else {
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
			CreatedAt: &msg.CreatedAt,
		}

		data, err := json.Marshal(serverMsg)
		if err != nil {
			slog.Error("failed to marshal chat message",
				slog.String("error", err.Error()),
				slog.String("message_id", msg.ID))
		} else {
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
