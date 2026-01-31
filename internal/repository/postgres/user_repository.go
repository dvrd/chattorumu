package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"jobsity-chat/internal/domain"
)

type UserRepository struct {
	db                *sql.DB
	createStmt        *sql.Stmt
	getByIDStmt       *sql.Stmt
	getByUsernameStmt *sql.Stmt
}

// NewUserRepository creates a new UserRepository with prepared statements.
// Returns an error if statement preparation fails.
func NewUserRepository(db *sql.DB) (*UserRepository, error) {
	repo := &UserRepository{db: db}

	var err error
	repo.createStmt, err = db.Prepare(`
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare create statement: %w", err)
	}

	repo.getByIDStmt, err = db.Prepare(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE id = $1
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare getByID statement: %w", err)
	}

	repo.getByUsernameStmt, err = db.Prepare(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE username = $1
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare getByUsername statement: %w", err)
	}

	return repo, nil
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	err := r.createStmt.QueryRowContext(ctx,
		user.Username,
		user.Email,
		user.PasswordHash,
	).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		if IsUniqueViolation(err, "users_username_key") {
			return domain.ErrUsernameExists
		}
		if IsUniqueViolation(err, "users_email_key") {
			return domain.ErrEmailExists
		}
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	user := &domain.User{}
	err := r.getByIDStmt.QueryRowContext(ctx, id).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	user := &domain.User{}
	err := r.getByUsernameStmt.QueryRowContext(ctx, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return user, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`
	user := &domain.User{}
	err := r.db.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return user, nil
}
