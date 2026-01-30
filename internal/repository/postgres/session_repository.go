package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"jobsity-chat/internal/domain"
)

type SessionRepository struct {
	db                *sql.DB
	createStmt        *sql.Stmt
	getByTokenStmt    *sql.Stmt
	deleteStmt        *sql.Stmt
	deleteExpiredStmt *sql.Stmt
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	repo := &SessionRepository{db: db}

	var err error
	repo.createStmt, err = db.Prepare(`
		INSERT INTO sessions (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare create statement: %v", err))
	}

	repo.getByTokenStmt, err = db.Prepare(`
		SELECT id, user_id, token, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > $2
	`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare getByToken statement: %v", err))
	}

	repo.deleteStmt, err = db.Prepare(`DELETE FROM sessions WHERE token = $1`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare delete statement: %v", err))
	}

	repo.deleteExpiredStmt, err = db.Prepare(`DELETE FROM sessions WHERE expires_at <= $1`)
	if err != nil {
		panic(fmt.Sprintf("failed to prepare deleteExpired statement: %v", err))
	}

	return repo
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	err := r.createStmt.QueryRowContext(ctx,
		session.UserID,
		session.Token,
		session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt)

	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	return nil
}

func (r *SessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	session := &domain.Session{}
	err := r.getByTokenStmt.QueryRowContext(ctx, token, time.Now()).Scan(
		&session.ID,
		&session.UserID,
		&session.Token,
		&session.ExpiresAt,
		&session.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by token: %w", err)
	}
	return session, nil
}

func (r *SessionRepository) Delete(ctx context.Context, token string) error {
	_, err := r.deleteStmt.ExecContext(ctx, token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := r.deleteExpiredStmt.ExecContext(ctx, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	return count, nil
}
