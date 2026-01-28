package postgres

import (
	"context"
	"database/sql"

	"jobsity-chat/internal/domain"
)

// MessageRepository implements domain.MessageRepository for PostgreSQL
type MessageRepository struct {
	db *sql.DB
}

// NewMessageRepository creates a new PostgreSQL message repository
func NewMessageRepository(db *sql.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

// Create inserts a new message into the database
func (r *MessageRepository) Create(ctx context.Context, message *domain.Message) error {
	query := `
		INSERT INTO messages (chatroom_id, user_id, content, is_bot)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`
	return r.db.QueryRowContext(ctx, query,
		message.ChatroomID,
		message.UserID,
		message.Content,
		message.IsBot,
	).Scan(&message.ID, &message.CreatedAt)
}

// GetByChatroom retrieves messages for a chatroom, ordered by timestamp (oldest first)
func (r *MessageRepository) GetByChatroom(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
	query := `
		SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
		FROM messages m
		JOIN users u ON m.user_id = u.id
		WHERE m.chatroom_id = $1
		ORDER BY m.created_at DESC
		LIMIT $2
	`

	rows, err := r.db.QueryContext(ctx, query, chatroomID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]*domain.Message, 0, limit)
	for rows.Next() {
		msg := &domain.Message{}
		err := rows.Scan(
			&msg.ID,
			&msg.ChatroomID,
			&msg.UserID,
			&msg.Username,
			&msg.Content,
			&msg.IsBot,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	// Reverse the slice to get oldest first
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, rows.Err()
}
