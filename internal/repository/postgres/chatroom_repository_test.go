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

func TestNewChatroomRepository(t *testing.T) {
	t.Run("successful_creation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)
		assert.NotNil(t, repo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fails_when_prepare_create_fails", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO chatrooms (name, created_by)
		VALUES ($1, $2)
		RETURNING id, created_at
	`)).WillReturnError(errors.New("prepare failed"))

		repo, err := NewChatroomRepository(db)
		require.Error(t, err)
		assert.Nil(t, repo)
		assert.Contains(t, err.Error(), "failed to prepare create statement")
	})
}

func TestChatroomRepository_Create(t *testing.T) {
	t.Run("successful_creation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		chatroomID := "room-123"
		createdAt := time.Now()

		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO chatrooms (name, created_by)
		VALUES ($1, $2)
		RETURNING id, created_at
	`)).
			WithArgs("Test Room", "user-123").
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(chatroomID, createdAt))

		chatroom := &domain.Chatroom{
			Name:      "Test Room",
			CreatedBy: "user-123",
		}

		err = repo.Create(context.Background(), chatroom)
		require.NoError(t, err)
		assert.Equal(t, chatroomID, chatroom.ID)
		assert.Equal(t, createdAt, chatroom.CreatedAt)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO chatrooms (name, created_by)
		VALUES ($1, $2)
		RETURNING id, created_at
	`)).
			WillReturnError(errors.New("database error"))

		chatroom := &domain.Chatroom{
			Name:      "Test Room",
			CreatedBy: "user-123",
		}

		err = repo.Create(context.Background(), chatroom)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create chatroom")
	})
}

func TestChatroomRepository_GetByID(t *testing.T) {
	t.Run("successful_retrieval", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		chatroomID := "room-123"
		createdAt := time.Now()

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, name, created_at, created_by
		FROM chatrooms
		WHERE id = $1
	`)).
			WithArgs(chatroomID).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at", "created_by"}).
				AddRow(chatroomID, "Test Room", createdAt, "user-123"))

		chatroom, err := repo.GetByID(context.Background(), chatroomID)
		require.NoError(t, err)
		assert.Equal(t, chatroomID, chatroom.ID)
		assert.Equal(t, "Test Room", chatroom.Name)
		assert.Equal(t, "user-123", chatroom.CreatedBy)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("chatroom_not_found", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, name, created_at, created_by
		FROM chatrooms
		WHERE id = $1
	`)).
			WithArgs("nonexistent").
			WillReturnError(sql.ErrNoRows)

		chatroom, err := repo.GetByID(context.Background(), "nonexistent")
		require.Error(t, err)
		assert.Nil(t, chatroom)
		assert.Equal(t, domain.ErrChatroomNotFound, err)
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, name, created_at, created_by
		FROM chatrooms
		WHERE id = $1
	`)).
			WithArgs("room-123").
			WillReturnError(errors.New("database error"))

		chatroom, err := repo.GetByID(context.Background(), "room-123")
		require.Error(t, err)
		assert.Nil(t, chatroom)
		assert.Contains(t, err.Error(), "failed to get chatroom by ID")
	})
}

func TestChatroomRepository_List(t *testing.T) {
	t.Run("successful_list", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		createdAt := time.Now()
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, name, created_at, created_by
		FROM chatrooms
		ORDER BY created_at DESC
	`)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at", "created_by"}).
				AddRow("room-1", "Room 1", createdAt, "user-1").
				AddRow("room-2", "Room 2", createdAt.Add(-time.Hour), "user-2"))

		chatrooms, err := repo.List(context.Background())
		require.NoError(t, err)
		assert.Len(t, chatrooms, 2)
		assert.Equal(t, "room-1", chatrooms[0].ID)
		assert.Equal(t, "room-2", chatrooms[1].ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty_list", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, name, created_at, created_by
		FROM chatrooms
		ORDER BY created_at DESC
	`)).
			WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at", "created_by"}))

		chatrooms, err := repo.List(context.Background())
		require.NoError(t, err)
		assert.Len(t, chatrooms, 0)
		assert.NotNil(t, chatrooms)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, name, created_at, created_by
		FROM chatrooms
		ORDER BY created_at DESC
	`)).
			WillReturnError(errors.New("database error"))

		chatrooms, err := repo.List(context.Background())
		require.Error(t, err)
		assert.Nil(t, chatrooms)
		assert.Contains(t, err.Error(), "failed to query chatrooms")
	})
}

func TestChatroomRepository_AddMember(t *testing.T) {
	t.Run("successful_add", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO chatroom_members (chatroom_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (chatroom_id, user_id) DO NOTHING
	`)).
			WithArgs("room-123", "user-456").
			WillReturnResult(sqlmock.NewResult(0, 1))

		err = repo.AddMember(context.Background(), "room-123", "user-456")
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("duplicate_member_ignored", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		// ON CONFLICT DO NOTHING means no rows affected for duplicates
		mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO chatroom_members (chatroom_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (chatroom_id, user_id) DO NOTHING
	`)).
			WithArgs("room-123", "user-456").
			WillReturnResult(sqlmock.NewResult(0, 0))

		err = repo.AddMember(context.Background(), "room-123", "user-456")
		require.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectExec(regexp.QuoteMeta(`
		INSERT INTO chatroom_members (chatroom_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (chatroom_id, user_id) DO NOTHING
	`)).
			WithArgs("room-123", "user-456").
			WillReturnError(errors.New("database error"))

		err = repo.AddMember(context.Background(), "room-123", "user-456")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to add member to chatroom")
	})
}

func TestChatroomRepository_IsMember(t *testing.T) {
	t.Run("user_is_member", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT EXISTS(
			SELECT 1 FROM chatroom_members
			WHERE chatroom_id = $1 AND user_id = $2
		)
	`)).
			WithArgs("room-123", "user-456").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))

		isMember, err := repo.IsMember(context.Background(), "room-123", "user-456")
		require.NoError(t, err)
		assert.True(t, isMember)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("user_not_member", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT EXISTS(
			SELECT 1 FROM chatroom_members
			WHERE chatroom_id = $1 AND user_id = $2
		)
	`)).
			WithArgs("room-123", "user-456").
			WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(false))

		isMember, err := repo.IsMember(context.Background(), "room-123", "user-456")
		require.NoError(t, err)
		assert.False(t, isMember)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT EXISTS(
			SELECT 1 FROM chatroom_members
			WHERE chatroom_id = $1 AND user_id = $2
		)
	`)).
			WithArgs("room-123", "user-456").
			WillReturnError(errors.New("database error"))

		isMember, err := repo.IsMember(context.Background(), "room-123", "user-456")
		require.Error(t, err)
		assert.False(t, isMember)
		assert.Contains(t, err.Error(), "failed to check chatroom membership")
	})
}

func TestChatroomRepository_CreateWithMember(t *testing.T) {
	t.Run("successful_creation_with_member", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		chatroomID := "room-123"
		createdAt := time.Now()

		// Expect transaction
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`
			INSERT INTO chatrooms (name, created_by)
			VALUES ($1, $2)
			RETURNING id, created_at
		`)).
			WithArgs("Test Room", "user-123").
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(chatroomID, createdAt))
		mock.ExpectExec(regexp.QuoteMeta(`
			INSERT INTO chatroom_members (chatroom_id, user_id)
			VALUES ($1, $2)
		`)).
			WithArgs(chatroomID, "user-123").
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		chatroom := &domain.Chatroom{
			Name:      "Test Room",
			CreatedBy: "user-123",
		}

		err = repo.CreateWithMember(context.Background(), chatroom, "user-123")
		require.NoError(t, err)
		assert.Equal(t, chatroomID, chatroom.ID)
		assert.Equal(t, createdAt, chatroom.CreatedAt)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fails_on_chatroom_insert_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupChatroomRepositoryMocks(mock)

		repo, err := NewChatroomRepository(db)
		require.NoError(t, err)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(`
			INSERT INTO chatrooms (name, created_by)
			VALUES ($1, $2)
			RETURNING id, created_at
		`)).
			WillReturnError(errors.New("database error"))
		mock.ExpectRollback()

		chatroom := &domain.Chatroom{
			Name:      "Test Room",
			CreatedBy: "user-123",
		}

		err = repo.CreateWithMember(context.Background(), chatroom, "user-123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to insert chatroom")
	})
}

// Helper function to set up common mock expectations
func setupChatroomRepositoryMocks(mock sqlmock.Sqlmock) {
	mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO chatrooms (name, created_by)
		VALUES ($1, $2)
		RETURNING id, created_at
	`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`
		SELECT id, name, created_at, created_by
		FROM chatrooms
		WHERE id = $1
	`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO chatroom_members (chatroom_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (chatroom_id, user_id) DO NOTHING
	`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`
		SELECT EXISTS(
			SELECT 1 FROM chatroom_members
			WHERE chatroom_id = $1 AND user_id = $2
		)
	`)).WillReturnCloseError(nil)
}
