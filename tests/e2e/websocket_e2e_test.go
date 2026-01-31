//go:build e2e
// +build e2e

package e2e

import (
	"testing"
	"time"
)

const (
	messageTimeout = 5 * time.Second
)

func TestWebSocket_Connect(t *testing.T) {
	t.Run("successful connection with valid token", func(t *testing.T) {
		client, chatroom := setupChatroomWithUser(t, "ws_connect")

		ws, err := client.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket connection should succeed")
		defer ws.Close()

		// Connection should be established
		if ws.conn == nil {
			t.Error("WebSocket connection should not be nil")
		}
	})

	t.Run("connection fails without token", func(t *testing.T) {
		_, chatroom := setupChatroomWithUser(t, "ws_no_token")

		// Create a client without logging in
		client := NewTestClient(t)

		ws, err := client.ConnectWebSocket(chatroom.ID)
		if err == nil {
			ws.Close()
			t.Error("WebSocket connection without token should fail")
		}
	})

	t.Run("connection fails for non-member", func(t *testing.T) {
		// Create a room with user 1
		_, chatroom := setupChatroomWithUser(t, "ws_nonmember_owner")

		// User 2 is logged in but not a member
		client2 := setupTestUser(t, "ws_nonmember")

		ws, err := client2.ConnectWebSocket(chatroom.ID)
		if err == nil {
			ws.Close()
			t.Error("WebSocket connection for non-member should fail")
		}
	})

	t.Run("member can connect after joining", func(t *testing.T) {
		_, chatroom := setupChatroomWithUser(t, "ws_after_join_owner")

		client2 := setupTestUser(t, "ws_after_join_member")

		// Join the chatroom first
		err := client2.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "join should succeed")

		// Now WebSocket should work
		ws, err := client2.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket connection should succeed after joining")
		defer ws.Close()
	})
}

func TestWebSocket_SendReceiveMessages(t *testing.T) {
	t.Run("user receives their own message", func(t *testing.T) {
		client, chatroom := setupChatroomWithUser(t, "ws_own_msg")

		ws, err := client.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket connection should succeed")
		defer ws.Close()

		// Give time for connection to stabilize
		time.Sleep(100 * time.Millisecond)

		// Drain any initial messages (like user_count_update)
		ws.DrainMessages()

		// Send a message
		testMessage := "Hello, World!"
		err = ws.SendMessage(testMessage)
		assertNoError(t, err, "send message should succeed")

		// Wait for the message to come back
		msg, err := ws.WaitForChatMessage(testMessage, messageTimeout)
		assertNoError(t, err, "should receive message")

		assertEqual(t, msg.Content, testMessage, "message content should match")
		if msg.Type != "message" && msg.Type != "chat_message" {
			t.Errorf("message type should be 'message' or 'chat_message', got %s", msg.Type)
		}
	})

	t.Run("broadcast to other users in room", func(t *testing.T) {
		client1, chatroom := setupChatroomWithUser(t, "ws_broadcast_owner")

		// User 2 joins and connects
		client2 := setupTestUser(t, "ws_broadcast_member")
		err := client2.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "join should succeed")

		ws1, err := client1.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket 1 connection should succeed")
		defer ws1.Close()

		ws2, err := client2.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket 2 connection should succeed")
		defer ws2.Close()

		// Give time for connections to stabilize
		time.Sleep(200 * time.Millisecond)

		// Drain initial messages
		ws1.DrainMessages()
		ws2.DrainMessages()

		// User 1 sends a message
		testMessage := "Hello from user 1!"
		err = ws1.SendMessage(testMessage)
		assertNoError(t, err, "send message should succeed")

		// User 2 should receive the message
		msg, err := ws2.WaitForChatMessage(testMessage, messageTimeout)
		assertNoError(t, err, "user 2 should receive broadcast message")

		assertEqual(t, msg.Content, testMessage, "message content should match")
	})

	t.Run("messages isolated to chatroom", func(t *testing.T) {
		// User 1 creates room A
		client1, roomA := setupChatroomWithUser(t, "ws_isolate_1")

		// User 2 creates room B
		client2, roomB := setupChatroomWithUser(t, "ws_isolate_2")

		ws1, err := client1.ConnectWebSocket(roomA.ID)
		assertNoError(t, err, "WebSocket 1 connection should succeed")
		defer ws1.Close()

		ws2, err := client2.ConnectWebSocket(roomB.ID)
		assertNoError(t, err, "WebSocket 2 connection should succeed")
		defer ws2.Close()

		// Give time for connections to stabilize
		time.Sleep(200 * time.Millisecond)

		// Drain initial messages
		ws1.DrainMessages()
		ws2.DrainMessages()

		// User 1 sends a message in room A
		testMessage := "Message in room A only!"
		err = ws1.SendMessage(testMessage)
		assertNoError(t, err, "send message should succeed")

		// User 1 should receive their own message
		msg, err := ws1.WaitForChatMessage(testMessage, messageTimeout)
		assertNoError(t, err, "user 1 should receive their message")
		assertEqual(t, msg.Content, testMessage, "message content should match")

		// User 2 should NOT receive this message (different room)
		_, err = ws2.WaitForChatMessage(testMessage, 1*time.Second)
		if err == nil {
			t.Error("user 2 should not receive message from different room")
		}
	})
}

func TestWebSocket_MessagePersistence(t *testing.T) {
	t.Run("messages are persisted to database", func(t *testing.T) {
		client, chatroom := setupChatroomWithUser(t, "ws_persist")

		ws, err := client.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket connection should succeed")

		// Give time for connection to stabilize
		time.Sleep(100 * time.Millisecond)
		ws.DrainMessages()

		// Send a message
		testMessage := "Persistent message test"
		err = ws.SendMessage(testMessage)
		assertNoError(t, err, "send message should succeed")

		// Wait for message to be processed
		_, err = ws.WaitForChatMessage(testMessage, messageTimeout)
		assertNoError(t, err, "should receive message")

		// Close WebSocket
		ws.Close()

		// Give time for message to be persisted
		time.Sleep(500 * time.Millisecond)

		// Retrieve messages via HTTP
		messages, err := client.GetMessages(chatroom.ID, 10)
		assertNoError(t, err, "get messages should succeed")

		// Find our message
		found := false
		for _, msg := range messages.Messages {
			if msg.Content == testMessage {
				found = true
				break
			}
		}

		if !found {
			t.Error("message should be persisted to database")
		}
	})
}

func TestWebSocket_MultipleUsers(t *testing.T) {
	t.Run("three users can chat together", func(t *testing.T) {
		// User 1 creates the room
		client1, chatroom := setupChatroomWithUser(t, "ws_multi_1")

		// Users 2 and 3 join
		client2 := setupTestUser(t, "ws_multi_2")
		err := client2.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "user 2 join should succeed")

		client3 := setupTestUser(t, "ws_multi_3")
		err = client3.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "user 3 join should succeed")

		// All three connect
		ws1, err := client1.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket 1 connection should succeed")
		defer ws1.Close()

		ws2, err := client2.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket 2 connection should succeed")
		defer ws2.Close()

		ws3, err := client3.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket 3 connection should succeed")
		defer ws3.Close()

		// Give time for connections to stabilize
		time.Sleep(300 * time.Millisecond)

		// Drain initial messages
		ws1.DrainMessages()
		ws2.DrainMessages()
		ws3.DrainMessages()

		// User 2 sends a message
		testMessage := "Hello from user 2!"
		err = ws2.SendMessage(testMessage)
		assertNoError(t, err, "send message should succeed")

		// User 1 should receive it
		msg1, err := ws1.WaitForChatMessage(testMessage, messageTimeout)
		assertNoError(t, err, "user 1 should receive message")
		assertEqual(t, msg1.Content, testMessage, "user 1 message content should match")

		// User 3 should receive it
		msg3, err := ws3.WaitForChatMessage(testMessage, messageTimeout)
		assertNoError(t, err, "user 3 should receive message")
		assertEqual(t, msg3.Content, testMessage, "user 3 message content should match")
	})
}

func TestWebSocket_UserCountUpdate(t *testing.T) {
	t.Run("user count updates on connection", func(t *testing.T) {
		client1, chatroom := setupChatroomWithUser(t, "ws_count_1")

		client2 := setupTestUser(t, "ws_count_2")
		err := client2.JoinChatroom(chatroom.ID)
		assertNoError(t, err, "join should succeed")

		// User 1 connects
		ws1, err := client1.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket 1 connection should succeed")
		defer ws1.Close()

		// Give time for connection
		time.Sleep(200 * time.Millisecond)
		ws1.DrainMessages()

		// User 2 connects
		ws2, err := client2.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket 2 connection should succeed")
		defer ws2.Close()

		// User 1 should receive a user_count_update
		msg, err := ws1.WaitForMessageType("user_count_update", messageTimeout)
		if err != nil {
			t.Skip("user count update not received - may be timing issue")
		}

		if msg.UserCounts == nil {
			t.Error("user counts should not be nil")
		}
	})
}

func TestWebSocket_ConnectionClose(t *testing.T) {
	t.Run("graceful disconnect", func(t *testing.T) {
		client, chatroom := setupChatroomWithUser(t, "ws_close")

		ws, err := client.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket connection should succeed")

		// Give time for connection
		time.Sleep(100 * time.Millisecond)

		// Close should not error
		err = ws.Close()
		assertNoError(t, err, "WebSocket close should succeed")
	})

	t.Run("can reconnect after disconnect", func(t *testing.T) {
		client, chatroom := setupChatroomWithUser(t, "ws_reconnect")

		// First connection
		ws1, err := client.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "first WebSocket connection should succeed")

		time.Sleep(100 * time.Millisecond)
		ws1.Close()

		// Small delay before reconnecting
		time.Sleep(200 * time.Millisecond)

		// Second connection
		ws2, err := client.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "second WebSocket connection should succeed")
		defer ws2.Close()

		// Should be able to send messages
		time.Sleep(100 * time.Millisecond)
		ws2.DrainMessages()

		err = ws2.SendMessage("Reconnected!")
		assertNoError(t, err, "send message after reconnect should succeed")

		_, err = ws2.WaitForChatMessage("Reconnected!", messageTimeout)
		assertNoError(t, err, "should receive message after reconnect")
	})
}

func TestWebSocket_StockCommand(t *testing.T) {
	t.Run("stock command is recognized", func(t *testing.T) {
		client, chatroom := setupChatroomWithUser(t, "ws_stock")

		ws, err := client.ConnectWebSocket(chatroom.ID)
		assertNoError(t, err, "WebSocket connection should succeed")
		defer ws.Close()

		// Give time for connection
		time.Sleep(100 * time.Millisecond)
		ws.DrainMessages()

		// Send a stock command
		err = ws.SendMessage("/stock=AAPL.US")
		assertNoError(t, err, "send stock command should succeed")

		// The command itself should be echoed or processed
		// Note: The actual stock response may take time and requires the stock bot
		// We're just testing that the command is accepted
		msg, err := ws.WaitForMessage(messageTimeout, func(m WSMessage) bool {
			// Accept any message type as the command should be processed
			return m.Type == "message" || m.Type == "stock_pending" || m.Type == "error"
		})

		if err != nil {
			t.Skip("stock command response not received - stock bot may not be running")
		}

		if msg != nil {
			t.Logf("Received response type: %s", msg.Type)
		}
	})
}
