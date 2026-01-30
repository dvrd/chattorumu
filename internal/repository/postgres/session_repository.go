package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"jobsity-chat/internal/domain"
)

type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, session *domain.Session) error {
	query := `
		INSERT INTO sessions (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`
	err := r.db.QueryRowContext(ctx, query,
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
	query := `
		SELECT id, user_id, token, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > $2
	`
	session := &domain.Session{}
	err := r.db.QueryRowContext(ctx, query, token, time.Now()).Scan(
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
	query := `DELETE FROM sessions WHERE token = $1`
	_, err := r.db.ExecContext(ctx, query, token)
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	return nil
}

func (r *SessionRepository) DeleteExpired(ctx context.Context) error {
	query := `DELETE FROM sessions WHERE expires_at <= $1`
	_, err := r.db.ExecContext(ctx, query, time.Now())
	if err != nil {
		return fmt.Errorf("failed to delete expired sessions: %w", err)
	}
	return nil
}
