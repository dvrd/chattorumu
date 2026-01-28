package postgres

import (
	"context"
	"database/sql"

	"jobsity-chat/internal/domain"
)

// ChatroomRepository implements domain.ChatroomRepository for PostgreSQL
type ChatroomRepository struct {
	db *sql.DB
}

// NewChatroomRepository creates a new PostgreSQL chatroom repository
func NewChatroomRepository(db *sql.DB) *ChatroomRepository {
	return &ChatroomRepository{db: db}
}

// Create inserts a new chatroom into the database
func (r *ChatroomRepository) Create(ctx context.Context, chatroom *domain.Chatroom) error {
	query := `
		INSERT INTO chatrooms (name, created_by)
		VALUES ($1, $2)
		RETURNING id, created_at
	`
	return r.db.QueryRowContext(ctx, query,
		chatroom.Name,
		chatroom.CreatedBy,
	).Scan(&chatroom.ID, &chatroom.CreatedAt)
}

// GetByID retrieves a chatroom by ID
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
	return chatroom, err
}

// List retrieves all chatrooms
func (r *ChatroomRepository) List(ctx context.Context) ([]*domain.Chatroom, error) {
	query := `
		SELECT id, name, created_at, created_by
		FROM chatrooms
		ORDER BY created_at DESC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
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
			return nil, err
		}
		chatrooms = append(chatrooms, chatroom)
	}

	return chatrooms, rows.Err()
}

// AddMember adds a user to a chatroom
func (r *ChatroomRepository) AddMember(ctx context.Context, chatroomID, userID string) error {
	query := `
		INSERT INTO chatroom_members (chatroom_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (chatroom_id, user_id) DO NOTHING
	`
	_, err := r.db.ExecContext(ctx, query, chatroomID, userID)
	return err
}

// IsMember checks if a user is a member of a chatroom
func (r *ChatroomRepository) IsMember(ctx context.Context, chatroomID, userID string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM chatroom_members
			WHERE chatroom_id = $1 AND user_id = $2
		)
	`
	var exists bool
	err := r.db.QueryRowContext(ctx, query, chatroomID, userID).Scan(&exists)
	return exists, err
}
