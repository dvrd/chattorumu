//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

func TestChatroom_Create(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		client := setupTestUser(t, "create_room")

		chatroom, err := client.CreateChatroom("My Test Room")
		assertNoError(t, err, "create chatroom should succeed")

		assertEqual(t, chatroom.Name, "My Test Room", "chatroom name should match")
		if chatroom.ID == "" {
			t.Error("chatroom ID should not be empty")
		}
		assertEqual(t, chatroom.CreatedBy, client.userID, "created_by should match user ID")
	})

	t.Run("creator is automatically a member", func(t *testing.T) {
		client := setupTestUser(t, "auto_member")

		chatroom, err := client.CreateChatroom("Auto Member Room")
		assertNoError(t, err, "create chatroom should succeed")

		// Creator should be able to get messages (which requires membership)
		messages, err := client.GetMessages(chatroom.ID, 10)
		assertNoError(t, err, "get messages should succeed for creator")

		if messages == nil {
			t.Error("messages response should not be nil")
		}
	})

	t.Run("empty name rejected", func(t *testing.T) {
		client := setupTestUser(t, "empty_name")

		_, err := client.CreateChatroom("")
		if err == nil {
			t.Error("empty chatroom name should be rejected")
		}
	})

	t.Run("unauthorized without session", func(t *testing.T) {
		client := NewTestClient(t)

		resp, err := client.PostJSON("/api/v1/chatrooms", map[string]string{"name": "Test"})
		assertNoError(t, err, "request should not error")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})
}

func TestChatroom_List(t *testing.T) {
	t.Run("lists all chatrooms", func(t *testing.T) {
		client := setupTestUser(t, "list_rooms")

		// Create some chatrooms
		room1, err := client.CreateChatroom("List Room 1")
		assertNoError(t, err, "create room 1 should succeed")

		room2, err := client.CreateChatroom("List Room 2")
		assertNoError(t, err, "create room 2 should succeed")

		// List chatrooms
		result, err := client.ListChatrooms()
		assertNoError(t, err, "list chatrooms should succeed")

		// Should contain our created rooms
		foundRoom1 := false
		foundRoom2 := false
		for _, room := range result.Chatrooms {
			if room.ID == room1.ID {
				foundRoom1 = true
			}
			if room.ID == room2.ID {
				foundRoom2 = true
			}
		}

		if !foundRoom1 {
			t.Error("room 1 should be in the list")
		}
		if !foundRoom2 {
			t.Error("room 2 should be in the list")
		}
	})

	t.Run("includes user count", func(t *testing.T) {
		client := setupTestUser(t, "user_count")

		_, err := client.CreateChatroom("User Count Room")
		assertNoError(t, err, "create chatroom should succeed")

		result, err := client.ListChatrooms()
		assertNoError(t, err, "list chatrooms should succeed")

		// User count should be present (may be 0 if no one is connected via WebSocket)
		for _, room := range result.Chatrooms {
			if room.UserCount < 0 {
				t.Errorf("user count should not be negative for room %s", room.ID)
			}
		}
	})

	t.Run("unauthorized without session", func(t *testing.T) {
		client := NewTestClient(t)

		resp, err := client.Get(baseURL + "/api/v1/chatrooms")
		assertNoError(t, err, "request should not error")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})
}

func TestChatroom_Join(t *testing.T) {
	t.Run("successful join", func(t *testing.T) {
		// Create a room with user 1
		client1, chatroom := setupChatroomWithUser(t, "join_owner")

		// User 2 joins the room
		client2 := setupTestUser(t, "join_member")
		err := client2.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "join chatroom should succeed")

		// User 2 should be able to get messages
		messages, err := client2.GetMessages(chatroom.ID, 10)
		assertNoError(t, err, "get messages should succeed after joining")

		if messages == nil {
			t.Error("messages response should not be nil")
		}
		_ = client1 // Keep reference
	})

	t.Run("joining twice is idempotent", func(t *testing.T) {
		_, chatroom := setupChatroomWithUser(t, "join_twice_owner")

		client2 := setupTestUser(t, "join_twice_member")

		// Join twice
		err := client2.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "first join should succeed")

		err = client2.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "second join should succeed (idempotent)")
	})

	t.Run("non-existent chatroom returns error", func(t *testing.T) {
		client := setupTestUser(t, "join_nonexistent")

		err := client.JoinChatroom("00000000-0000-0000-0000-000000000000")
		if err == nil {
			t.Error("joining non-existent chatroom should fail")
		}
	})

	t.Run("unauthorized without session", func(t *testing.T) {
		_, chatroom := setupChatroomWithUser(t, "join_unauth_owner")

		client := NewTestClient(t)
		resp, err := client.PostJSON(fmt.Sprintf("/api/v1/chatrooms/%s/join", chatroom.ID), nil)
		assertNoError(t, err, "request should not error")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})
}

func TestChatroom_GetMessages(t *testing.T) {
	t.Run("empty room returns empty messages", func(t *testing.T) {
		_, chatroom := setupChatroomWithUser(t, "empty_messages")

		client2 := setupTestUser(t, "empty_messages_reader")
		err := client2.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "join should succeed")

		messages, err := client2.GetMessages(chatroom.ID, 10)
		assertNoError(t, err, "get messages should succeed")

		if len(messages.Messages) != 0 {
			t.Errorf("expected 0 messages, got %d", len(messages.Messages))
		}
	})

	t.Run("non-member cannot get messages", func(t *testing.T) {
		_, chatroom := setupChatroomWithUser(t, "access_denied_owner")

		// Create another user who is NOT a member
		client2 := setupTestUser(t, "access_denied_nonmember")

		resp, err := client2.Get(fmt.Sprintf("%s/api/v1/chatrooms/%s/messages", baseURL, chatroom.ID))
		assertNoError(t, err, "request should not error")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("expected status 403 for non-member, got %d", resp.StatusCode)
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		client, chatroom := setupChatroomWithUser(t, "limit_test")

		// Get messages with limit
		messages, err := client.GetMessages(chatroom.ID, 5)
		assertNoError(t, err, "get messages should succeed")

		// Since room is empty, we just verify no error
		if messages == nil {
			t.Error("messages response should not be nil")
		}
	})

	t.Run("unauthorized without session", func(t *testing.T) {
		_, chatroom := setupChatroomWithUser(t, "messages_unauth")

		client := NewTestClient(t)
		resp, err := client.Get(fmt.Sprintf("%s/api/v1/chatrooms/%s/messages", baseURL, chatroom.ID))
		assertNoError(t, err, "request should not error")
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", resp.StatusCode)
		}
	})
}

func TestChatroom_AccessControl(t *testing.T) {
	t.Run("user can only access rooms they are a member of", func(t *testing.T) {
		// User 1 creates room A
		client1, roomA := setupChatroomWithUser(t, "access_user1")

		// User 2 creates room B
		client2, roomB := setupChatroomWithUser(t, "access_user2")

		// User 1 should be able to access room A
		_, err := client1.GetMessages(roomA.ID, 10)
		assertNoError(t, err, "user1 should access roomA")

		// User 2 should be able to access room B
		_, err = client2.GetMessages(roomB.ID, 10)
		assertNoError(t, err, "user2 should access roomB")

		// User 1 should NOT be able to access room B
		resp, err := client1.Get(fmt.Sprintf("%s/api/v1/chatrooms/%s/messages", baseURL, roomB.ID))
		assertNoError(t, err, "request should not error")
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("user1 should not access roomB, got status %d", resp.StatusCode)
		}

		// User 2 should NOT be able to access room A
		resp, err = client2.Get(fmt.Sprintf("%s/api/v1/chatrooms/%s/messages", baseURL, roomA.ID))
		assertNoError(t, err, "request should not error")
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("user2 should not access roomA, got status %d", resp.StatusCode)
		}
	})

	t.Run("user can access room after joining", func(t *testing.T) {
		_, chatroom := setupChatroomWithUser(t, "access_after_join_owner")

		client2 := setupTestUser(t, "access_after_join_joiner")

		// Before joining, should not have access
		resp, err := client2.Get(fmt.Sprintf("%s/api/v1/chatrooms/%s/messages", baseURL, chatroom.ID))
		assertNoError(t, err, "request should not error")
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("should not have access before joining, got status %d", resp.StatusCode)
		}

		// Join the room
		err = client2.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "join should succeed")

		// After joining, should have access
		messages, err := client2.GetMessages(chatroom.ID, 10)
		assertNoError(t, err, "should have access after joining")

		if messages == nil {
			t.Error("messages should not be nil")
		}
	})
}
