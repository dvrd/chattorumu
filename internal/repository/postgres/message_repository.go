package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"jobsity-chat/internal/domain"
)

type MessageRepository struct {
	db                       *sql.DB
	createStmt               *sql.Stmt
	getByChatroomStmt        *sql.Stmt
	getByChatroomBeforeStmt  *sql.Stmt
}

func NewMessageRepository(db *sql.DB) *MessageRepository {
	repo := &MessageRepository{db: db}

	var err error
	repo.createStmt, err = db.Prepare(`
		INSERT INTO messages (chatroom_id, user_id, content, is_bot)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare create statement: %v", err))
	}

	repo.getByChatroomStmt, err = db.Prepare(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2
		) AS recent_messages
		ORDER BY created_at ASC
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare getByChatroom statement: %v", err))
	}

	repo.getByChatroomBeforeStmt, err = db.Prepare(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1 AND m.id < $2
			ORDER BY m.created_at DESC
			LIMIT $3
		) AS earlier_messages
		ORDER BY created_at ASC
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare getByChatroomBefore statement: %v", err))
	}

	return repo
}

func (r *MessageRepository) Create(ctx context.Context, message *domain.Message) error {
	err := r.createStmt.QueryRowContext(ctx,
		message.ChatroomID,
		message.UserID,
		message.Content,
		message.IsBot,
	).Scan(&message.ID, &message.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}
	return nil
}

func (r *MessageRepository) GetByChatroom(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
	rows, err := r.getByChatroomStmt.QueryContext(ctx, chatroomID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
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
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}

func (r *MessageRepository) GetByChatroomBefore(ctx context.Context, chatroomID string, before string, limit int) ([]*domain.Message, error) {
	rows, err := r.getByChatroomBeforeStmt.QueryContext(ctx, chatroomID, before, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages before timestamp: %w", err)
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
			return nil, fmt.Errorf("failed to scan message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating messages: %w", err)
	}

	return messages, nil
}
