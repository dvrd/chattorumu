//go:build integration
// +build integration

package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestPostgresContainer manages PostgreSQL container lifecycle for integration tests
type TestPostgresContainer struct {
	container testcontainers.Container
	db        *sql.DB
	connStr   string
}

// setupPostgres starts a PostgreSQL container and returns a database connection
func setupPostgres(t *testing.T) (*TestPostgresContainer, func()) {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp"),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "failed to start PostgreSQL container")

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5432")
	require.NoError(t, err)

	connStr := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())

	// Wait for PostgreSQL to be fully ready
	time.Sleep(2 * time.Second)

	db, err := sql.Open("postgres", connStr)
	require.NoError(t, err, "failed to connect to PostgreSQL")

	// Run migrations
	err = runMigrations(db)
	require.NoError(t, err, "failed to run migrations")

	cleanup := func() {
		db.Close()
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return &TestPostgresContainer{
		container: container,
		db:        db,
		connStr:   connStr,
	}, cleanup
}

// runMigrations creates the database schema for testing
func runMigrations(db *sql.DB) error {
	schema := `
		CREATE EXTENSION IF NOT EXISTS "pgcrypto";

		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(50) UNIQUE NOT NULL CHECK (length(username) >= 3),
			email VARCHAR(255) UNIQUE NOT NULL CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token VARCHAR(255) UNIQUE NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS chatrooms (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(100) NOT NULL CHECK (length(name) >= 1),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
			created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS chatroom_members (
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			chatroom_id UUID NOT NULL REFERENCES chatrooms(id) ON DELETE CASCADE,
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
			PRIMARY KEY (user_id, chatroom_id)
		);

		CREATE TABLE IF NOT EXISTS messages (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			chatroom_id UUID NOT NULL REFERENCES chatrooms(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			content TEXT NOT NULL CHECK (length(content) > 0 AND length(content) <= 1000),
			is_bot BOOLEAN DEFAULT FALSE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
		);
	`
	_, err := db.Exec(schema)
	return err
}

// TestUserRepository_Integration tests the UserRepository with a real PostgreSQL database
func TestUserRepository_Integration(t *testing.T) {
	pg, cleanup := setupPostgres(t)
	defer cleanup()

	repo := postgres.NewUserRepository(pg.db)

	t.Run("Create_and_GetByID", func(t *testing.T) {
		user := &domain.User{
			Username:     "testuser1",
			Email:        "test1@example.com",
			PasswordHash: "hashed_password_123",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)
		assert.NotEmpty(t, user.ID, "user ID should be set after creation")
		assert.False(t, user.CreatedAt.IsZero(), "created_at should be set")

		// Retrieve the user
		retrieved, err := repo.GetByID(context.Background(), user.ID)
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, user.Username, retrieved.Username)
		assert.Equal(t, user.Email, retrieved.Email)
		assert.Equal(t, user.PasswordHash, retrieved.PasswordHash)
	})

	t.Run("Create_and_GetByUsername", func(t *testing.T) {
		user := &domain.User{
			Username:     "testuser2",
			Email:        "test2@example.com",
			PasswordHash: "hashed_password_456",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)

		// Retrieve by username
		retrieved, err := repo.GetByUsername(context.Background(), "testuser2")
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, "testuser2", retrieved.Username)
	})

	t.Run("Create_and_GetByEmail", func(t *testing.T) {
		user := &domain.User{
			Username:     "testuser3",
			Email:        "test3@example.com",
			PasswordHash: "hashed_password_789",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)

		// Retrieve by email
		retrieved, err := repo.GetByEmail(context.Background(), "test3@example.com")
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, "test3@example.com", retrieved.Email)
	})

	t.Run("Create_DuplicateUsername", func(t *testing.T) {
		user1 := &domain.User{
			Username:     "duplicate_user",
			Email:        "dup1@example.com",
			PasswordHash: "hash1",
		}
		err := repo.Create(context.Background(), user1)
		require.NoError(t, err)

		user2 := &domain.User{
			Username:     "duplicate_user", // Same username
			Email:        "dup2@example.com",
			PasswordHash: "hash2",
		}
		err = repo.Create(context.Background(), user2)
		assert.ErrorIs(t, err, domain.ErrUsernameExists)
	})

	t.Run("Create_DuplicateEmail", func(t *testing.T) {
		user1 := &domain.User{
			Username:     "email_user1",
			Email:        "duplicate@example.com",
			PasswordHash: "hash1",
		}
		err := repo.Create(context.Background(), user1)
		require.NoError(t, err)

		user2 := &domain.User{
			Username:     "email_user2",
			Email:        "duplicate@example.com", // Same email
			PasswordHash: "hash2",
		}
		err = repo.Create(context.Background(), user2)
		assert.ErrorIs(t, err, domain.ErrEmailExists)
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		_, err := repo.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
	})

	t.Run("GetByUsername_NotFound", func(t *testing.T) {
		_, err := repo.GetByUsername(context.Background(), "nonexistent_user")
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
	})

	t.Run("GetByEmail_NotFound", func(t *testing.T) {
		_, err := repo.GetByEmail(context.Background(), "nonexistent@example.com")
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
	})
}

// TestSessionRepository_Integration tests the SessionRepository with a real PostgreSQL database
func TestSessionRepository_Integration(t *testing.T) {
	pg, cleanup := setupPostgres(t)
	defer cleanup()

	userRepo := postgres.NewUserRepository(pg.db)
	sessionRepo := postgres.NewSessionRepository(pg.db)

	// Create a user first
	user := &domain.User{
		Username:     "session_test_user",
		Email:        "session@example.com",
		PasswordHash: "test_hash",
	}
	err := userRepo.Create(context.Background(), user)
	require.NoError(t, err)

	t.Run("Create_and_GetByToken", func(t *testing.T) {
		session := &domain.Session{
			UserID:    user.ID,
			Token:     "test_token_123",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		err := sessionRepo.Create(context.Background(), session)
		require.NoError(t, err)
		assert.NotEmpty(t, session.ID)

		// Retrieve by token
		retrieved, err := sessionRepo.GetByToken(context.Background(), "test_token_123")
		require.NoError(t, err)
		assert.Equal(t, session.ID, retrieved.ID)
		assert.Equal(t, user.ID, retrieved.UserID)
		assert.Equal(t, "test_token_123", retrieved.Token)
	})

	t.Run("Delete", func(t *testing.T) {
		session := &domain.Session{
			UserID:    user.ID,
			Token:     "token_to_delete",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		err := sessionRepo.Create(context.Background(), session)
		require.NoError(t, err)

		// Delete the session
		err = sessionRepo.Delete(context.Background(), "token_to_delete")
		require.NoError(t, err)

		// Should not be found anymore
		_, err = sessionRepo.GetByToken(context.Background(), "token_to_delete")
		assert.ErrorIs(t, err, domain.ErrSessionNotFound)
	})

	t.Run("DeleteExpired", func(t *testing.T) {
		// Create expired session
		expiredSession := &domain.Session{
			UserID:    user.ID,
			Token:     "expired_token",
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
		}
		err := sessionRepo.Create(context.Background(), expiredSession)
		require.NoError(t, err)

		// Create valid session
		validSession := &domain.Session{
			UserID:    user.ID,
			Token:     "valid_token",
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err = sessionRepo.Create(context.Background(), validSession)
		require.NoError(t, err)

		// Delete expired sessions
		count, err := sessionRepo.DeleteExpired(context.Background())
		require.NoError(t, err)
		assert.GreaterOrEqual(t, count, int64(1))

		// Expired session should be gone
		_, err = sessionRepo.GetByToken(context.Background(), "expired_token")
		assert.ErrorIs(t, err, domain.ErrSessionNotFound)

		// Valid session should still exist
		_, err = sessionRepo.GetByToken(context.Background(), "valid_token")
		assert.NoError(t, err)
	})

	t.Run("GetByToken_NotFound", func(t *testing.T) {
		_, err := sessionRepo.GetByToken(context.Background(), "nonexistent_token")
		assert.ErrorIs(t, err, domain.ErrSessionNotFound)
	})
}

// TestChatroomRepository_Integration tests the ChatroomRepository with a real PostgreSQL database
func TestChatroomRepository_Integration(t *testing.T) {
	pg, cleanup := setupPostgres(t)
	defer cleanup()

	userRepo := postgres.NewUserRepository(pg.db)
	chatroomRepo := postgres.NewChatroomRepository(pg.db)

	// Create a user first
	user := &domain.User{
		Username:     "chatroom_test_user",
		Email:        "chatroom@example.com",
		PasswordHash: "test_hash",
	}
	err := userRepo.Create(context.Background(), user)
	require.NoError(t, err)

	t.Run("Create_and_GetByID", func(t *testing.T) {
		chatroom := &domain.Chatroom{
			Name:      "Test Room",
			CreatedBy: user.ID,
		}

		err := chatroomRepo.Create(context.Background(), chatroom)
		require.NoError(t, err)
		assert.NotEmpty(t, chatroom.ID)
		assert.False(t, chatroom.CreatedAt.IsZero())

		// Retrieve
		retrieved, err := chatroomRepo.GetByID(context.Background(), chatroom.ID)
		require.NoError(t, err)
		assert.Equal(t, chatroom.ID, retrieved.ID)
		assert.Equal(t, "Test Room", retrieved.Name)
		assert.Equal(t, user.ID, retrieved.CreatedBy)
	})

	t.Run("CreateWithMember", func(t *testing.T) {
		chatroom := &domain.Chatroom{
			Name:      "Room With Member",
			CreatedBy: user.ID,
		}

		err := chatroomRepo.CreateWithMember(context.Background(), chatroom, user.ID)
		require.NoError(t, err)

		// User should be a member
		isMember, err := chatroomRepo.IsMember(context.Background(), chatroom.ID, user.ID)
		require.NoError(t, err)
		assert.True(t, isMember)
	})

	t.Run("AddMember_and_IsMember", func(t *testing.T) {
		chatroom := &domain.Chatroom{
			Name:      "Membership Test Room",
			CreatedBy: user.ID,
		}
		err := chatroomRepo.Create(context.Background(), chatroom)
		require.NoError(t, err)

		// User should NOT be a member initially
		isMember, err := chatroomRepo.IsMember(context.Background(), chatroom.ID, user.ID)
		require.NoError(t, err)
		assert.False(t, isMember)

		// Add member
		err = chatroomRepo.AddMember(context.Background(), chatroom.ID, user.ID)
		require.NoError(t, err)

		// User should now be a member
		isMember, err = chatroomRepo.IsMember(context.Background(), chatroom.ID, user.ID)
		require.NoError(t, err)
		assert.True(t, isMember)
	})

	t.Run("List", func(t *testing.T) {
		// Create some chatrooms
		for i := 0; i < 3; i++ {
			chatroom := &domain.Chatroom{
				Name:      fmt.Sprintf("List Test Room %d", i),
				CreatedBy: user.ID,
			}
			err := chatroomRepo.Create(context.Background(), chatroom)
			require.NoError(t, err)
		}

		// List all chatrooms
		chatrooms, err := chatroomRepo.List(context.Background())
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(chatrooms), 3)
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		_, err := chatroomRepo.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, domain.ErrChatroomNotFound)
	})
}

// TestMessageRepository_Integration tests the MessageRepository with a real PostgreSQL database
func TestMessageRepository_Integration(t *testing.T) {
	pg, cleanup := setupPostgres(t)
	defer cleanup()

	userRepo := postgres.NewUserRepository(pg.db)
	chatroomRepo := postgres.NewChatroomRepository(pg.db)
	messageRepo := postgres.NewMessageRepository(pg.db)

	// Create user and chatroom
	user := &domain.User{
		Username:     "message_test_user",
		Email:        "message@example.com",
		PasswordHash: "test_hash",
	}
	err := userRepo.Create(context.Background(), user)
	require.NoError(t, err)

	chatroom := &domain.Chatroom{
		Name:      "Message Test Room",
		CreatedBy: user.ID,
	}
	err = chatroomRepo.Create(context.Background(), chatroom)
	require.NoError(t, err)

	t.Run("Create_and_GetByChatroom", func(t *testing.T) {
		// Create some messages
		for i := 0; i < 5; i++ {
			msg := &domain.Message{
				ChatroomID: chatroom.ID,
				UserID:     user.ID,
				Username:   user.Username,
				Content:    fmt.Sprintf("Test message %d", i),
				IsBot:      false,
			}
			err := messageRepo.Create(context.Background(), msg)
			require.NoError(t, err)
			assert.NotEmpty(t, msg.ID)

			// Small delay to ensure different timestamps
			time.Sleep(10 * time.Millisecond)
		}

		// Retrieve messages
		messages, err := messageRepo.GetByChatroom(context.Background(), chatroom.ID, 10)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(messages), 5)
	})

	t.Run("Create_BotMessage", func(t *testing.T) {
		msg := &domain.Message{
			ChatroomID: chatroom.ID,
			UserID:     user.ID,
			Username:   "StockBot",
			Content:    "AAPL.US quote is $150.00",
			IsBot:      true,
		}
		err := messageRepo.Create(context.Background(), msg)
		require.NoError(t, err)
		assert.True(t, msg.IsBot)
	})

	t.Run("GetByChatroom_Limit", func(t *testing.T) {
		// Create a new chatroom for this test
		newChatroom := &domain.Chatroom{
			Name:      "Limit Test Room",
			CreatedBy: user.ID,
		}
		err := chatroomRepo.Create(context.Background(), newChatroom)
		require.NoError(t, err)

		// Create 10 messages
		for i := 0; i < 10; i++ {
			msg := &domain.Message{
				ChatroomID: newChatroom.ID,
				UserID:     user.ID,
				Username:   user.Username,
				Content:    fmt.Sprintf("Limit test message %d", i),
				IsBot:      false,
			}
			err := messageRepo.Create(context.Background(), msg)
			require.NoError(t, err)
		}

		// Retrieve with limit
		messages, err := messageRepo.GetByChatroom(context.Background(), newChatroom.ID, 5)
		require.NoError(t, err)
		assert.LessOrEqual(t, len(messages), 5)
	})

	t.Run("GetByChatroom_EmptyRoom", func(t *testing.T) {
		// Create a new empty chatroom
		emptyChatroom := &domain.Chatroom{
			Name:      "Empty Room",
			CreatedBy: user.ID,
		}
		err := chatroomRepo.Create(context.Background(), emptyChatroom)
		require.NoError(t, err)

		// Should return empty slice, not error
		messages, err := messageRepo.GetByChatroom(context.Background(), emptyChatroom.ID, 10)
		require.NoError(t, err)
		assert.Empty(t, messages)
	})
}
