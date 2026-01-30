package domain

import (
	"context"
	"time"
)

// Message represents a chat message
type Message struct {
	ID         string    `json:"id"`
	ChatroomID string    `json:"chatroom_id"`
	UserID     string    `json:"user_id"`
	Username   string    `json:"username"`
	Content    string    `json:"content"`
	IsBot      bool      `json:"is_bot"`
	CreatedAt  time.Time `json:"created_at"`
}

// MessageRepository defines the interface for message data access
type MessageRepository interface {
	Create(ctx context.Context, message *Message) error
	GetByChatroom(ctx context.Context, chatroomID string, limit int) ([]*Message, error)
	GetByChatroomBefore(ctx context.Context, chatroomID string, before string, limit int) ([]*Message, error)
}
