package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"jobsity-chat/internal/domain"
)

type ChatroomRepository struct {
	db *sql.DB
	tm *TxManager
}

func NewChatroomRepository(db *sql.DB) *ChatroomRepository {
	return &ChatroomRepository{
		db: db,
		tm: NewTxManager(db),
	}
}

func (r *ChatroomRepository) Create(ctx context.Context, chatroom *domain.Chatroom) error {
	query := `
		INSERT INTO chatrooms (name, created_by)
		VALUES ($1, $2)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
		chatroom.Name,
		chatroom.CreatedBy,
	).Scan(&chatroom.ID, &chatroom.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create chatroom: %w", err)
	}
	return nil
}

func (r *ChatroomRepository) GetByID(ctx context.Context, id string) (*domain.Chatroom, error) {
	query := `
		SELECT id, name, created_at, created_by
		FROM chatrooms
		WHERE id = $1
	`
	chatroom := &domain.Chatroom{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&chatroom.ID,
		&chatroom.Name,
		&chatroom.CreatedAt,
		&chatroom.CreatedBy,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrChatroomNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get chatroom by ID: %w", err)
	}
	return chatroom, nil
}

func (r *ChatroomRepository) List(ctx context.Context) ([]*domain.Chatroom, error) {
	query := `
		SELECT id, name, created_at, created_by
		FROM chatrooms
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query chatrooms: %w", err)
	}
	defer rows.Close()

	chatrooms := make([]*domain.Chatroom, 0)
	for rows.Next() {
		chatroom := &domain.Chatroom{}
		err := rows.Scan(
			&chatroom.ID,
			&chatroom.Name,
			&chatroom.CreatedAt,
			&chatroom.CreatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chatroom: %w", err)
		}
		chatrooms = append(chatrooms, chatroom)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating chatrooms: %w", err)
	}

	return chatrooms, nil
}

func (r *ChatroomRepository) AddMember(ctx context.Context, chatroomID, userID string) error {
	query := `
		INSERT INTO chatroom_members (chatroom_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (chatroom_id, user_id) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, chatroomID, userID)
	if err != nil {
		return fmt.Errorf("failed to add member to chatroom: %w", err)
	}
	return nil
}

func (r *ChatroomRepository) IsMember(ctx context.Context, chatroomID, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM chatroom_members
			WHERE chatroom_id = $1 AND user_id = $2
		)
	`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, chatroomID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check chatroom membership: %w", err)
	}
	return exists, nil
}

// CreateWithMember atomically creates a chatroom and adds a member
func (r *ChatroomRepository) CreateWithMember(ctx context.Context, chatroom *domain.Chatroom, userID string) error {
	return r.tm.WithTx(ctx, func(tx *sql.Tx) error {
		query := `
			INSERT INTO chatrooms (name, created_by)
			VALUES ($1, $2)
			RETURNING id, created_at
		`
		if err := tx.QueryRowContext(ctx, query, chatroom.Name, chatroom.CreatedBy).
			Scan(&chatroom.ID, &chatroom.CreatedAt); err != nil {
			return fmt.Errorf("failed to insert chatroom: %w", err)
		}

		memberQuery := `
			INSERT INTO chatroom_members (chatroom_id, user_id)
			VALUES ($1, $2)
		`
		if _, err := tx.ExecContext(ctx, memberQuery, chatroom.ID, userID); err != nil {
			return fmt.Errorf("failed to add member: %w", err)
		}

		return nil
	})
}
