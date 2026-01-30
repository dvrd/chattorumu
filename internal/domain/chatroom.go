package domain

import (
	"context"
	"errors"
	"time"
)

var (
	ErrChatroomNotFound = errors.New("chatroom not found")
	ErrNotMember        = errors.New("user is not a member of this chatroom")
)

// Chatroom represents a chat room
type Chatroom struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
}

// ChatroomRepository defines the interface for chatroom data access
type ChatroomRepository interface {
	Create(ctx context.Context, chatroom *Chatroom) error
	CreateWithMember(ctx context.Context, chatroom *Chatroom, userID string) error
	GetByID(ctx context.Context, id string) (*Chatroom, error)
	List(ctx context.Context) ([]*Chatroom, error)
	ListPaginated(ctx context.Context, limit int, cursor string) ([]*Chatroom, string, error)
	AddMember(ctx context.Context, chatroomID, userID string) error
	IsMember(ctx context.Context, chatroomID, userID string) (bool, error)
}
