package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"jobsity-chat/internal/domain"
)

type ChatroomRepository struct {
	db            *sql.DB
	tm            *TxManager
	createStmt    *sql.Stmt
	getByIDStmt   *sql.Stmt
	addMemberStmt *sql.Stmt
	isMemberStmt  *sql.Stmt
}

func NewChatroomRepository(db *sql.DB) *ChatroomRepository {
	repo := &ChatroomRepository{
		db: db,
		tm: NewTxManager(db),
	}

	var err error
	repo.createStmt, err = db.Prepare(`
		INSERT INTO chatrooms (name, created_by)
		VALUES ($1, $2)
		RETURNING id, created_at
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare create statement: %v", err))
	}

	repo.getByIDStmt, err = db.Prepare(`
		SELECT id, name, created_at, created_by
		FROM chatrooms
		WHERE id = $1
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare getByID statement: %v", err))
	}

	repo.addMemberStmt, err = db.Prepare(`
		INSERT INTO chatroom_members (chatroom_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (chatroom_id, user_id) DO NOTHING
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare addMember statement: %v", err))
	}

	repo.isMemberStmt, err = db.Prepare(`
		SELECT EXISTS(
			SELECT 1 FROM chatroom_members
			WHERE chatroom_id = $1 AND user_id = $2
		)
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare isMember statement: %v", err))
	}

	return repo
}

func (r *ChatroomRepository) Create(ctx context.Context, chatroom *domain.Chatroom) error {
	err := r.createStmt.QueryRowContext(ctx,
		chatroom.Name,
		chatroom.CreatedBy,
	).Scan(&chatroom.ID, &chatroom.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create chatroom: %w", err)
	}
	return nil
}

func (r *ChatroomRepository) GetByID(ctx context.Context, id string) (*domain.Chatroom, error) {
	chatroom := &domain.Chatroom{}
	err := r.getByIDStmt.QueryRowContext(ctx, id).Scan(
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

func (r *ChatroomRepository) ListPaginated(ctx context.Context, limit int, cursor string) ([]*domain.Chatroom, string, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	var query string
	var rows *sql.Rows
	var err error

	if cursor == "" {
		query = `
			SELECT id, name, created_at, created_by
			FROM chatrooms
			ORDER BY created_at DESC, id DESC
			LIMIT $1
		`
		rows, err = r.db.QueryContext(ctx, query, limit+1)
	} else {
		query = `
			SELECT id, name, created_at, created_by
			FROM chatrooms
			WHERE created_at < (SELECT created_at FROM chatrooms WHERE id = $1)
			   OR (created_at = (SELECT created_at FROM chatrooms WHERE id = $1) AND id < $1)
			ORDER BY created_at DESC, id DESC
			LIMIT $2
		`
		rows, err = r.db.QueryContext(ctx, query, cursor, limit+1)
	}

	if err != nil {
		return nil, "", fmt.Errorf("failed to query chatrooms: %w", err)
	}
	defer rows.Close()

	chatrooms := make([]*domain.Chatroom, 0, limit)
	for rows.Next() {
		chatroom := &domain.Chatroom{}
		err := rows.Scan(
			&chatroom.ID,
			&chatroom.Name,
			&chatroom.CreatedAt,
			&chatroom.CreatedBy,
		)
		if err != nil {
			return nil, "", fmt.Errorf("failed to scan chatroom: %w", err)
		}
		chatrooms = append(chatrooms, chatroom)
	}

	if err := rows.Err(); err != nil {
		return nil, "", fmt.Errorf("error iterating chatrooms: %w", err)
	}

	var nextCursor string
	if len(chatrooms) > limit {
		nextCursor = chatrooms[limit-1].ID
		chatrooms = chatrooms[:limit]
	}

	return chatrooms, nextCursor, nil
}

func (r *ChatroomRepository) AddMember(ctx context.Context, chatroomID, userID string) error {
	_, err := r.addMemberStmt.ExecContext(ctx, chatroomID, userID)
	if err != nil {
		return fmt.Errorf("failed to add member to chatroom: %w", err)
	}
	return nil
}

func (r *ChatroomRepository) IsMember(ctx context.Context, chatroomID, userID string) (bool, error) {
	var exists bool
	err := r.isMemberStmt.QueryRowContext(ctx, chatroomID, userID).Scan(&exists)
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
