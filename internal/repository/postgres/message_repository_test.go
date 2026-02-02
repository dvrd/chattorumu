package postgres

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"jobsity-chat/internal/domain"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessageRepository(t *testing.T) {
	t.Run("successful_creation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)
		assert.NotNil(t, repo)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("fails_when_prepare_create_fails", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO messages (chatroom_id, user_id, content, is_bot)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`)).WillReturnError(errors.New("prepare failed"))

		repo, err := NewMessageRepository(db)
		require.Error(t, err)
		assert.Nil(t, repo)
		assert.Contains(t, err.Error(), "failed to prepare create statement")
	})
}

func TestMessageRepository_Create(t *testing.T) {
	t.Run("successful_creation", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		messageID := "msg-123"
		createdAt := time.Now()

		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO messages (chatroom_id, user_id, content, is_bot)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`)).
			WithArgs("room-123", "user-123", "Hello World", false).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(messageID, createdAt))

		message := &domain.Message{
			ChatroomID: "room-123",
			UserID:     "user-123",
			Content:    "Hello World",
			IsBot:      false,
		}

		err = repo.Create(context.Background(), message)
		require.NoError(t, err)
		assert.Equal(t, messageID, message.ID)
		assert.Equal(t, createdAt, message.CreatedAt)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("create_bot_message", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		messageID := "msg-124"
		createdAt := time.Now()

		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO messages (chatroom_id, user_id, content, is_bot)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`)).
			WithArgs("room-123", "bot-user", "AAPL.US quote is $150.00", true).
			WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
				AddRow(messageID, createdAt))

		message := &domain.Message{
			ChatroomID: "room-123",
			UserID:     "bot-user",
			Content:    "AAPL.US quote is $150.00",
			IsBot:      true,
		}

		err = repo.Create(context.Background(), message)
		require.NoError(t, err)
		assert.Equal(t, messageID, message.ID)
		assert.True(t, message.IsBot)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		INSERT INTO messages (chatroom_id, user_id, content, is_bot)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`)).
			WillReturnError(errors.New("database error"))

		message := &domain.Message{
			ChatroomID: "room-123",
			UserID:     "user-123",
			Content:    "Hello World",
			IsBot:      false,
		}

		err = repo.Create(context.Background(), message)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create message")
	})
}

func TestMessageRepository_GetByChatroom(t *testing.T) {
	t.Run("successful_retrieval", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		createdAt := time.Now()
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2
		) AS recent_messages
		ORDER BY created_at ASC
	`)).
			WithArgs("room-123", 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "chatroom_id", "user_id", "username", "content", "is_bot", "created_at"}).
				AddRow("msg-1", "room-123", "user-1", "Alice", "Hello", false, createdAt).
				AddRow("msg-2", "room-123", "user-2", "Bob", "Hi", false, createdAt.Add(1*time.Second)))

		messages, err := repo.GetByChatroom(context.Background(), "room-123", 10)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
		assert.Equal(t, "msg-1", messages[0].ID)
		assert.Equal(t, "Alice", messages[0].Username)
		assert.Equal(t, "msg-2", messages[1].ID)
		assert.Equal(t, "Bob", messages[1].Username)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("empty_chatroom", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2
		) AS recent_messages
		ORDER BY created_at ASC
	`)).
			WithArgs("room-123", 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "chatroom_id", "user_id", "username", "content", "is_bot", "created_at"}))

		messages, err := repo.GetByChatroom(context.Background(), "room-123", 10)
		require.NoError(t, err)
		assert.Len(t, messages, 0)
		assert.NotNil(t, messages)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("with_limit", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		createdAt := time.Now()
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2
		) AS recent_messages
		ORDER BY created_at ASC
	`)).
			WithArgs("room-123", 5).
			WillReturnRows(sqlmock.NewRows([]string{"id", "chatroom_id", "user_id", "username", "content", "is_bot", "created_at"}).
				AddRow("msg-1", "room-123", "user-1", "Alice", "Message 1", false, createdAt).
				AddRow("msg-2", "room-123", "user-1", "Alice", "Message 2", false, createdAt.Add(1*time.Second)).
				AddRow("msg-3", "room-123", "user-1", "Alice", "Message 3", false, createdAt.Add(2*time.Second)).
				AddRow("msg-4", "room-123", "user-1", "Alice", "Message 4", false, createdAt.Add(3*time.Second)).
				AddRow("msg-5", "room-123", "user-1", "Alice", "Message 5", false, createdAt.Add(4*time.Second)))

		messages, err := repo.GetByChatroom(context.Background(), "room-123", 5)
		require.NoError(t, err)
		assert.Len(t, messages, 5)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2
		) AS recent_messages
		ORDER BY created_at ASC
	`)).
			WithArgs("room-123", 10).
			WillReturnError(errors.New("database error"))

		messages, err := repo.GetByChatroom(context.Background(), "room-123", 10)
		require.Error(t, err)
		assert.Nil(t, messages)
		assert.Contains(t, err.Error(), "failed to query messages")
	})
}

func TestMessageRepository_GetByChatroomBefore(t *testing.T) {
	t.Run("successful_retrieval", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		createdAt := time.Now()
		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1 AND m.id < $2
			ORDER BY m.created_at DESC
			LIMIT $3
		) AS earlier_messages
		ORDER BY created_at ASC
	`)).
			WithArgs("room-123", "msg-100", 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "chatroom_id", "user_id", "username", "content", "is_bot", "created_at"}).
				AddRow("msg-99", "room-123", "user-1", "Alice", "Message 99", false, createdAt).
				AddRow("msg-98", "room-123", "user-2", "Bob", "Message 98", false, createdAt.Add(1*time.Second)))

		messages, err := repo.GetByChatroomBefore(context.Background(), "room-123", "msg-100", 10)
		require.NoError(t, err)
		assert.Len(t, messages, 2)
		assert.Equal(t, "msg-99", messages[0].ID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("no_earlier_messages", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1 AND m.id < $2
			ORDER BY m.created_at DESC
			LIMIT $3
		) AS earlier_messages
		ORDER BY created_at ASC
	`)).
			WithArgs("room-123", "msg-1", 10).
			WillReturnRows(sqlmock.NewRows([]string{"id", "chatroom_id", "user_id", "username", "content", "is_bot", "created_at"}))

		messages, err := repo.GetByChatroomBefore(context.Background(), "room-123", "msg-1", 10)
		require.NoError(t, err)
		assert.Len(t, messages, 0)
		assert.NotNil(t, messages)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database_error", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer db.Close()

		setupMessageRepositoryMocks(mock)

		repo, err := NewMessageRepository(db)
		require.NoError(t, err)

		mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1 AND m.id < $2
			ORDER BY m.created_at DESC
			LIMIT $3
		) AS earlier_messages
		ORDER BY created_at ASC
	`)).
			WithArgs("room-123", "msg-100", 10).
			WillReturnError(errors.New("database error"))

		messages, err := repo.GetByChatroomBefore(context.Background(), "room-123", "msg-100", 10)
		require.Error(t, err)
		assert.Nil(t, messages)
		assert.Contains(t, err.Error(), "failed to query messages before timestamp")
	})
}

// Helper function to set up common mock expectations
func setupMessageRepositoryMocks(mock sqlmock.Sqlmock) {
	mock.ExpectPrepare(regexp.QuoteMeta(`
		INSERT INTO messages (chatroom_id, user_id, content, is_bot)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at
	`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1
			ORDER BY m.created_at DESC
			LIMIT $2
		) AS recent_messages
		ORDER BY created_at ASC
	`)).WillReturnCloseError(nil)

	mock.ExpectPrepare(regexp.QuoteMeta(`
		SELECT id, chatroom_id, user_id, username, content, is_bot, created_at
		FROM (
			SELECT m.id, m.chatroom_id, m.user_id, u.username, m.content, m.is_bot, m.created_at
			FROM messages m
			JOIN users u ON m.user_id = u.id
			WHERE m.chatroom_id = $1 AND m.id < $2
			ORDER BY m.created_at DESC
			LIMIT $3
		) AS earlier_messages
		ORDER BY created_at ASC
	`)).WillReturnCloseError(nil)
}
