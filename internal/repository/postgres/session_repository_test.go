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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSessionRepository(t *testing.T) {
	t.Run("successful_creation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)
		assert.NotNil(t, repo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fails_when_prepare_create_fails", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO sessions (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).WillReturnError(errors.New("prepare failed"))

		repo, err := NewSessionRepository(db)
		require.Error(t, err)
		assert.Nil(t, repo)
		assert.Contains(t, err.Error(), "failed to prepare create statement")
	})
}

func TestSessionRepository_Create(t *testing.T) {
	t.Run("successful_creation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		sessionID := "550e8400-e29b-41d4-a716-446655440000"
		userID := "user-123"
		createdAt := time.Now()

		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO sessions (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).
			WithArgs(userID, "token123", time.Time{}).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(sessionID, createdAt))

		session := &domain.Session{
			UserID:    userID,
			Token:     "token123",
			ExpiresAt: time.Time{},
		}

		err = repo.Create(context.Background(), session)
		require.NoError(t, err)
		assert.Equal(t, sessionID, session.ID)
		assert.Equal(t, createdAt, session.CreatedAt)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO sessions (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).
			WillReturnError(errors.New("database error"))

		session := &domain.Session{
			UserID:    "user-123",
			Token:     "token123",
			ExpiresAt: time.Time{},
		}

		err = repo.Create(context.Background(), session)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create session")
	})
}

func TestSessionRepository_GetByToken(t *testing.T) {
	t.Run("successful_retrieval", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		sessionID := "550e8400-e29b-41d4-a716-446655440000"
		userID := "user-123"
		createdAt := time.Now()
		expiresAt := time.Now().Add(24 * time.Hour)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, user_id, token, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > $2
	`)).
			WithArgs("token123", sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "token", "expires_at", "created_at"}).
				AddRow(sessionID, userID, "token123", expiresAt, createdAt))

		session, err := repo.GetByToken(context.Background(), "token123")
		require.NoError(t, err)
		assert.Equal(t, sessionID, session.ID)
		assert.Equal(t, userID, session.UserID)
		assert.Equal(t, "token123", session.Token)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("session_not_found", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, user_id, token, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > $2
	`)).
			WithArgs("nonexistent", sqlmock.AnyArg()).
			WillReturnError(sql.ErrNoRows)

		session, err := repo.GetByToken(context.Background(), "nonexistent")
		require.Error(t, err)
		assert.Nil(t, session)
		assert.Equal(t, domain.ErrSessionNotFound, err)
	})

	t.Run("expired_session_returns_not_found", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		// Expired sessions should not be returned
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, user_id, token, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > $2
	`)).
			WithArgs("expired_token", sqlmock.AnyArg()).
			WillReturnError(sql.ErrNoRows)

		session, err := repo.GetByToken(context.Background(), "expired_token")
		require.Error(t, err)
		assert.Nil(t, session)
		assert.Equal(t, domain.ErrSessionNotFound, err)
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, user_id, token, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > $2
	`)).
			WithArgs("token123", sqlmock.AnyArg()).
			WillReturnError(errors.New("database error"))

		session, err := repo.GetByToken(context.Background(), "token123")
		require.Error(t, err)
		assert.Nil(t, session)
		assert.Contains(t, err.Error(), "failed to get session by token")
	})
}

func TestSessionRepository_Delete(t *testing.T) {
	t.Run("successful_deletion", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sessions WHERE token = $1`)).
			WithArgs("token123").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err = repo.Delete(context.Background(), "token123")
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("delete_non_existent_session", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		// Deleting non-existent session should not error
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sessions WHERE token = $1`)).
			WithArgs("nonexistent").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err = repo.Delete(context.Background(), "nonexistent")
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sessions WHERE token = $1`)).
			WithArgs("token123").
			WillReturnError(errors.New("database error"))

		err = repo.Delete(context.Background(), "token123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete session")
	})
}

func TestSessionRepository_DeleteExpired(t *testing.T) {
	t.Run("successful_deletion", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sessions WHERE expires_at <= $1`)).
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 5))

		count, err := repo.DeleteExpired(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(5), count)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no_expired_sessions", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sessions WHERE expires_at <= $1`)).
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(0, 0))

		count, err := repo.DeleteExpired(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sessions WHERE expires_at <= $1`)).
			WithArgs(sqlmock.AnyArg()).
			WillReturnError(errors.New("database error"))

		count, err := repo.DeleteExpired(context.Background())
		require.Error(t, err)
		assert.Equal(t, int64(0), count)
		assert.Contains(t, err.Error(), "failed to delete expired sessions")
	})

	t.Run("rows_affected_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupSessionRepositoryMocks(mock)

		repo, err := NewSessionRepository(db)
		require.NoError(t, err)

		// Return a result that errors on RowsAffected
		mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM sessions WHERE expires_at <= $1`)).
			WithArgs(sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewErrorResult(errors.New("failed to get rows affected")))

		count, err := repo.DeleteExpired(context.Background())
		require.Error(t, err)
		assert.Equal(t, int64(0), count)
		assert.Contains(t, err.Error(), "failed to get rows affected")
	})
}

// Helper function to set up common mock expectations
func setupSessionRepositoryMocks(mock sqlmock.Sqlmock) {
	mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO sessions (user_id, token, expires_at)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`
		SELECT id, user_id, token, expires_at, created_at
		FROM sessions
		WHERE token = $1 AND expires_at > $2
	`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`DELETE FROM sessions WHERE token = $1`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`DELETE FROM sessions WHERE expires_at <= $1`)).WillReturnCloseError(nil)
}
