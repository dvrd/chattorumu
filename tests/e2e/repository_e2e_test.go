//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for the jobsity-chat application.
// This file contains repository integration tests that verify database operations
// against a real PostgreSQL database running in a Docker container.
package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/repository/postgres"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserRepository_Integration tests the UserRepository with a real PostgreSQL database
func TestUserRepository_Integration(t *testing.T) {
	repo, err := postgres.NewUserRepository(testDB)
	require.NoError(t, err, "failed to create user repository")

	t.Run("Create_and_GetByID", func(t *testing.T) {
		user := &domain.User{
			Username:     "testuser_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			Email:        fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()),
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
		uniqueUsername := "testuser_" + fmt.Sprintf("%d", time.Now().UnixNano())
		user := &domain.User{
			Username:     uniqueUsername,
			Email:        fmt.Sprintf("test_%d@example.com", time.Now().UnixNano()),
			PasswordHash: "hashed_password_456",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)

		// Retrieve by username
		retrieved, err := repo.GetByUsername(context.Background(), uniqueUsername)
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, uniqueUsername, retrieved.Username)
	})

	t.Run("Create_and_GetByEmail", func(t *testing.T) {
		uniqueEmail := fmt.Sprintf("test_%d@example.com", time.Now().UnixNano())
		user := &domain.User{
			Username:     "testuser_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			Email:        uniqueEmail,
			PasswordHash: "hashed_password_789",
		}

		err := repo.Create(context.Background(), user)
		require.NoError(t, err)

		// Retrieve by email
		retrieved, err := repo.GetByEmail(context.Background(), uniqueEmail)
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, uniqueEmail, retrieved.Email)
	})

	t.Run("Create_DuplicateUsername", func(t *testing.T) {
		duplicateUsername := "duplicate_user_" + fmt.Sprintf("%d", time.Now().UnixNano())
		user1 := &domain.User{
			Username:     duplicateUsername,
			Email:        fmt.Sprintf("dup1_%d@example.com", time.Now().UnixNano()),
			PasswordHash: "hash1",
		}
		err := repo.Create(context.Background(), user1)
		require.NoError(t, err)

		user2 := &domain.User{
			Username:     duplicateUsername, // Same username
			Email:        fmt.Sprintf("dup2_%d@example.com", time.Now().UnixNano()),
			PasswordHash: "hash2",
		}
		err = repo.Create(context.Background(), user2)
		assert.ErrorIs(t, err, domain.ErrUsernameExists)
	})

	t.Run("Create_DuplicateEmail", func(t *testing.T) {
		duplicateEmail := fmt.Sprintf("duplicate_%d@example.com", time.Now().UnixNano())
		user1 := &domain.User{
			Username:     "email_user1_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			Email:        duplicateEmail,
			PasswordHash: "hash1",
		}
		err := repo.Create(context.Background(), user1)
		require.NoError(t, err)

		user2 := &domain.User{
			Username:     "email_user2_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			Email:        duplicateEmail, // Same email
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
		_, err := repo.GetByUsername(context.Background(), "nonexistent_user_"+fmt.Sprintf("%d", time.Now().UnixNano()))
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
	})

	t.Run("GetByEmail_NotFound", func(t *testing.T) {
		_, err := repo.GetByEmail(context.Background(), "nonexistent_"+fmt.Sprintf("%d", time.Now().UnixNano())+"@example.com")
		assert.ErrorIs(t, err, domain.ErrUserNotFound)
	})
}

// TestSessionRepository_Integration tests the SessionRepository with a real PostgreSQL database
func TestSessionRepository_Integration(t *testing.T) {
	userRepo, err := postgres.NewUserRepository(testDB)
	require.NoError(t, err, "failed to create user repository")
	sessionRepo, err := postgres.NewSessionRepository(testDB)
	require.NoError(t, err, "failed to create session repository")

	// Create a user first
	user := &domain.User{
		Username:     "session_test_user_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("session_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "test_hash",
	}
	err = userRepo.Create(context.Background(), user)
	require.NoError(t, err)

	t.Run("Create_and_GetByToken", func(t *testing.T) {
		session := &domain.Session{
			UserID:    user.ID,
			Token:     "test_token_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		err := sessionRepo.Create(context.Background(), session)
		require.NoError(t, err)
		assert.NotEmpty(t, session.ID)

		// Retrieve by token
		retrieved, err := sessionRepo.GetByToken(context.Background(), session.Token)
		require.NoError(t, err)
		assert.Equal(t, session.ID, retrieved.ID)
		assert.Equal(t, user.ID, retrieved.UserID)
		assert.Equal(t, session.Token, retrieved.Token)
	})

	t.Run("Delete", func(t *testing.T) {
		tokenToDelete := "token_to_delete_" + fmt.Sprintf("%d", time.Now().UnixNano())
		session := &domain.Session{
			UserID:    user.ID,
			Token:     tokenToDelete,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}

		err := sessionRepo.Create(context.Background(), session)
		require.NoError(t, err)

		// Delete the session
		err = sessionRepo.Delete(context.Background(), tokenToDelete)
		require.NoError(t, err)

		// Should not be found anymore
		_, err = sessionRepo.GetByToken(context.Background(), tokenToDelete)
		assert.ErrorIs(t, err, domain.ErrSessionNotFound)
	})

	t.Run("DeleteExpired", func(t *testing.T) {
		// Create expired session
		expiredSession := &domain.Session{
			UserID:    user.ID,
			Token:     "expired_token_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Already expired
		}
		err := sessionRepo.Create(context.Background(), expiredSession)
		require.NoError(t, err)

		// Create valid session
		validSession := &domain.Session{
			UserID:    user.ID,
			Token:     "valid_token_" + fmt.Sprintf("%d", time.Now().UnixNano()),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		err = sessionRepo.Create(context.Background(), validSession)
		require.NoError(t, err)

		// Delete expired sessions
		count, err := sessionRepo.DeleteExpired(context.Background())
		require.NoError(t, err)
		assert.Greater(t, count, int64(0))

		// Expired session should be gone
		_, err = sessionRepo.GetByToken(context.Background(), expiredSession.Token)
		assert.ErrorIs(t, err, domain.ErrSessionNotFound)

		// Valid session should still exist
		_, err = sessionRepo.GetByToken(context.Background(), validSession.Token)
		assert.NoError(t, err)
	})

	t.Run("GetByToken_NotFound", func(t *testing.T) {
		_, err := sessionRepo.GetByToken(context.Background(), "nonexistent_"+fmt.Sprintf("%d", time.Now().UnixNano()))
		assert.ErrorIs(t, err, domain.ErrSessionNotFound)
	})
}

// TestChatroomRepository_Integration tests the ChatroomRepository with a real PostgreSQL database
func TestChatroomRepository_Integration(t *testing.T) {
	userRepo, err := postgres.NewUserRepository(testDB)
	require.NoError(t, err, "failed to create user repository")
	chatroomRepo, err := postgres.NewChatroomRepository(testDB)
	require.NoError(t, err, "failed to create chatroom repository")

	// Create a user first
	user := &domain.User{
		Username:     "chatroom_test_user_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("chatroom_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "test_hash",
	}
	err = userRepo.Create(context.Background(), user)
	require.NoError(t, err)

	t.Run("Create_and_GetByID", func(t *testing.T) {
		chatroom := &domain.Chatroom{
			Name:      "Test Room " + fmt.Sprintf("%d", time.Now().UnixNano()),
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
		assert.Equal(t, chatroom.Name, retrieved.Name)
		assert.Equal(t, user.ID, retrieved.CreatedBy)
	})

	t.Run("CreateWithMember", func(t *testing.T) {
		chatroom := &domain.Chatroom{
			Name:      "Room With Member " + fmt.Sprintf("%d", time.Now().UnixNano()),
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
			Name:      "Membership Test Room " + fmt.Sprintf("%d", time.Now().UnixNano()),
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
				Name:      fmt.Sprintf("List Test Room %d %d", i, time.Now().UnixNano()),
				CreatedBy: user.ID,
			}
			err := chatroomRepo.Create(context.Background(), chatroom)
			require.NoError(t, err)
		}

		// List all chatrooms
		chatrooms, err := chatroomRepo.List(context.Background())
		require.NoError(t, err)
		assert.Greater(t, len(chatrooms), 0)
	})

	t.Run("GetByID_NotFound", func(t *testing.T) {
		_, err := chatroomRepo.GetByID(context.Background(), "00000000-0000-0000-0000-000000000000")
		assert.ErrorIs(t, err, domain.ErrChatroomNotFound)
	})
}

// TestMessageRepository_Integration tests the MessageRepository with a real PostgreSQL database
func TestMessageRepository_Integration(t *testing.T) {
	userRepo, err := postgres.NewUserRepository(testDB)
	require.NoError(t, err, "failed to create user repository")
	chatroomRepo, err := postgres.NewChatroomRepository(testDB)
	require.NoError(t, err, "failed to create chatroom repository")
	messageRepo, err := postgres.NewMessageRepository(testDB)
	require.NoError(t, err, "failed to create message repository")

	// Create user and chatroom
	user := &domain.User{
		Username:     "message_test_user_" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Email:        fmt.Sprintf("message_%d@example.com", time.Now().UnixNano()),
		PasswordHash: "test_hash",
	}
	err = userRepo.Create(context.Background(), user)
	require.NoError(t, err)

	chatroom := &domain.Chatroom{
		Name:      "Message Test Room " + fmt.Sprintf("%d", time.Now().UnixNano()),
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
		assert.Greater(t, len(messages), 0)
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
			Name:      "Limit Test Room " + fmt.Sprintf("%d", time.Now().UnixNano()),
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
			Name:      "Empty Room " + fmt.Sprintf("%d", time.Now().UnixNano()),
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
