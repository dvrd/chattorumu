package postgres

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"jobsity-chat/internal/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUserRepository(t *testing.T) {
	t.Run("successful_creation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		// Expect prepared statements
		mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).WillReturnCloseError(nil)

		mock.ExpectPrepare(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE id = $1
	`)).WillReturnCloseError(nil)

		mock.ExpectPrepare(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE username = $1
	`)).WillReturnCloseError(nil)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)
		assert.NotNil(t, repo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fails_when_prepare_create_fails", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).WillReturnError(errors.New("prepare failed"))

		repo, err := NewUserRepository(db)
		require.Error(t, err)
		assert.Nil(t, repo)
		assert.Contains(t, err.Error(), "failed to prepare create statement")
	})
}

func TestUserRepository_Create(t *testing.T) {
	t.Run("successful_creation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		createdAt := time.Now()
		userID := "550e8400-e29b-41d4-a716-446655440000"

		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).
			WithArgs("testuser", "test@example.com", "hashed_password").
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(userID, createdAt))

		user := &domain.User{
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "hashed_password",
		}

		err = repo.Create(context.Background(), user)
		require.NoError(t, err)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, createdAt, user.CreatedAt)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("duplicate_username_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		// Simulate PostgreSQL unique constraint violation for username
		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).
			WithArgs("testuser", "test@example.com", "hashed_password").
			WillReturnError(&pq.Error{Code: "23505", Constraint: "users_username_key"})

		user := &domain.User{
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "hashed_password",
		}

		err = repo.Create(context.Background(), user)
		require.Error(t, err)
		assert.Equal(t, domain.ErrUsernameExists, err)
	})

	t.Run("duplicate_email_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		// Simulate PostgreSQL unique constraint violation for email
		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).
			WithArgs("testuser", "test@example.com", "hashed_password").
			WillReturnError(&pq.Error{Code: "23505", Constraint: "users_email_key"})

		user := &domain.User{
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "hashed_password",
		}

		err = repo.Create(context.Background(), user)
		require.Error(t, err)
		assert.Equal(t, domain.ErrEmailExists, err)
	})

	t.Run("query_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).
			WithArgs("testuser", "test@example.com", "hashed_password").
			WillReturnError(errors.New("database error"))

		user := &domain.User{
			Username:     "testuser",
			Email:        "test@example.com",
			PasswordHash: "hashed_password",
		}

		err = repo.Create(context.Background(), user)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create user")
	})
}

func TestUserRepository_GetByID(t *testing.T) {
	t.Run("successful_retrieval", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		userID := "550e8400-e29b-41d4-a716-446655440000"
		createdAt := time.Now()

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE id = $1
	`)).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "created_at"}).
				AddRow(userID, "testuser", "test@example.com", "hashed_password", createdAt))

		user, err := repo.GetByID(context.Background(), userID)
		require.NoError(t, err)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "testuser", user.Username)
		assert.Equal(t, "test@example.com", user.Email)
		assert.Equal(t, "hashed_password", user.PasswordHash)
		assert.Equal(t, createdAt, user.CreatedAt)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("user_not_found", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		userID := "nonexistent-id"

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE id = $1
	`)).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetByID(context.Background(), userID)
		require.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, domain.ErrUserNotFound, err)
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		userID := "550e8400-e29b-41d4-a716-446655440000"

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE id = $1
	`)).
			WithArgs(userID).
			WillReturnError(errors.New("database connection error"))

		user, err := repo.GetByID(context.Background(), userID)
		require.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to get user by ID")
	})
}

func TestUserRepository_GetByUsername(t *testing.T) {
	t.Run("successful_retrieval", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		userID := "550e8400-e29b-41d4-a716-446655440000"
		createdAt := time.Now()

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE username = $1
	`)).
			WithArgs("testuser").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "created_at"}).
				AddRow(userID, "testuser", "test@example.com", "hashed_password", createdAt))

		user, err := repo.GetByUsername(context.Background(), "testuser")
		require.NoError(t, err)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "testuser", user.Username)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("user_not_found", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE username = $1
	`)).
			WithArgs("nonexistent").
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetByUsername(context.Background(), "nonexistent")
		require.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, domain.ErrUserNotFound, err)
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE username = $1
	`)).
			WithArgs("testuser").
			WillReturnError(errors.New("database error"))

		user, err := repo.GetByUsername(context.Background(), "testuser")
		require.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to get user by username")
	})
}

func TestUserRepository_GetByEmail(t *testing.T) {
	t.Run("successful_retrieval", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		userID := "550e8400-e29b-41d4-a716-446655440000"
		createdAt := time.Now()

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`)).
			WithArgs("test@example.com").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username", "email", "password_hash", "created_at"}).
				AddRow(userID, "testuser", "test@example.com", "hashed_password", createdAt))

		user, err := repo.GetByEmail(context.Background(), "test@example.com")
		require.NoError(t, err)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "test@example.com", user.Email)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("email_not_found", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`)).
			WithArgs("nonexistent@example.com").
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetByEmail(context.Background(), "nonexistent@example.com")
		require.Error(t, err)
		assert.Nil(t, user)
		assert.Equal(t, domain.ErrUserNotFound, err)
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`)).
			WithArgs("test@example.com").
			WillReturnError(errors.New("database error"))

		user, err := repo.GetByEmail(context.Background(), "test@example.com")
		require.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to get user by email")
	})

	t.Run("scan_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupUserRepositoryMocks(mock)

		repo, err := NewUserRepository(db)
		require.NoError(t, err)

		// Return wrong number of columns
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE email = $1
	`)).
			WithArgs("test@example.com").
			WillReturnRows(sqlmock.NewRows([]string{"id", "username"}).
				AddRow("123", "testuser"))

		user, err := repo.GetByEmail(context.Background(), "test@example.com")
		require.Error(t, err)
		assert.Nil(t, user)
	})
}

// Helper function to set up common mock expectations
func setupUserRepositoryMocks(mock sqlmock.Sqlmock) {
	mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO users (username, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE id = $1
	`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`
		SELECT id, username, email, password_hash, created_at
		FROM users
		WHERE username = $1
	`)).WillReturnCloseError(nil)
}
