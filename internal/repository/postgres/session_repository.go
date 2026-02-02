package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"jobsity-chat/internal/domain"
)

type SessionRepository struct {
	db                  *sql.DB
	createStmt          *sql.Stmt
	getByTokenStmt      *sql.Stmt
	getByCSRFTokenStmt  *sql.Stmt
	deleteStmt          *sql.Stmt
	deleteExpiredStmt   *sql.Stmt
	updateCSRFTokenStmt *sql.Stmt
}

// NewSessionRepository creates a new SessionRepository with prepared statements.
// Returns an error if statement preparation fails.
func NewSessionRepository(db *sql.DB) (*SessionRepository, error) {
	repo := &SessionRepository{db: db}

	var err error
	repo.createStmt, err = db.Prepare(`
		INSERT INTO sessions (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare create statement: %w", err)
	}

	repo.getByTokenStmt, err = db.Prepare(`
		SELECT id, user_id, token, csrf_token, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > $2
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare getByToken statement: %w", err)
	}

	repo.deleteStmt, err = db.Prepare(`DELETE FROM sessions WHERE token = $1`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare delete statement: %w", err)
	}

	repo.deleteExpiredStmt, err = db.Prepare(`DELETE FROM sessions WHERE expires_at <= $1`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare deleteExpired statement: %w", err)
	}

	repo.getByCSRFTokenStmt, err = db.Prepare(`
		SELECT id, user_id, token, csrf_token, expires_at, created_at
		FROM sessions
		WHERE csrf_token = $1 AND expires_at > $2
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare getByCSRFToken statement: %w", err)
	}

	repo.updateCSRFTokenStmt, err = db.Prepare(`
		UPDATE sessions SET csrf_token = $1 WHERE token = $2
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare updateCSRFToken statement: %w", err)
	}

	return repo, nil
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
		&session.CSRFToken,
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

// GetByCSRFToken retrieves a session by its CSRF token if it hasn't expired.
// Returns ErrSessionNotFound if the token is invalid or session expired.
func (r *SessionRepository) GetByCSRFToken(ctx context.Context, csrfToken string) (*domain.Session, error) {
	session := &domain.Session{}
	err := r.getByCSRFTokenStmt.QueryRowContext(ctx, csrfToken, time.Now()).Scan(
		&session.ID,
		&session.UserID,
		&session.Token,
		&session.CSRFToken,
		&session.ExpiresAt,
		&session.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session by csrf token: %w", err)
	}
	return session, nil
}

// UpdateCSRFToken updates the CSRF token for a session.
// Used to set the token after session creation.
func (r *SessionRepository) UpdateCSRFToken(ctx context.Context, csrfToken, sessionToken string) error {
	_, err := r.updateCSRFTokenStmt.ExecContext(ctx, csrfToken, sessionToken)
	if err != nil {
		return fmt.Errorf("failed to update csrf token: %w", err)
	}
	return nil
}
