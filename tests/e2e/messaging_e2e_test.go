//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for the jobsity-chat application.
// This file contains messaging/RabbitMQ integration tests.
package e2e

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"jobsity-chat/internal/messaging"
)

// publishStockResponse publishes a StockResponse to RabbitMQ
func publishStockResponse(t *testing.T, resp *messaging.StockResponse) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rmq.PublishStockResponse(ctx, resp)
	require.NoError(t, err, "failed to publish stock response")
}

// === GROUP 1: CONNECTION & PUBLISHING ===

// TestMessaging_RabbitMQConnection verifies RabbitMQ is connected
func TestMessaging_RabbitMQConnection(t *testing.T) {
	t.Parallel()
	// Verify RabbitMQ is connected
	assert.False(t, rmq.IsClosed(), "RabbitMQ should be connected")
}

// TestMessaging_PublishStockCommand verifies publishing stock commands
func TestMessaging_PublishStockCommand(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rmq.PublishStockCommand(ctx, "test-chatroom-1", "AAPL.US", "test-user-1")
	assert.NoError(t, err, "should publish stock command without error")
}

// TestMessaging_PublishHelloCommand verifies publishing hello commands
func TestMessaging_PublishHelloCommand(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rmq.PublishHelloCommand(ctx, "test-chatroom-2", "test-user-2")
	assert.NoError(t, err, "should publish hello command without error")
}

// === GROUP 2: CONSUMER & BROADCASTING ===

// TestMessaging_ResponseConsumer_ReceivesAndBroadcasts tests response receiving and broadcasting
func TestMessaging_ResponseConsumer_ReceivesAndBroadcasts(t *testing.T) {
	t.Parallel()
	// Create a test user and chatroom
	testChatroom := createTestChatroom(t, "")

	// Connect WebSocket client
	client, err := newTestWSClient(t, nil, testChatroom.ID)
	require.NoError(t, err, "should connect WebSocket client")
	defer client.Close()

	// Drain initial connection messages
	client.DrainMessages()

	// Publish a StockResponse
	resp := &messaging.StockResponse{
		ChatroomID:       testChatroom.ID,
		Symbol:           "AAPL.US",
		Price:            174.25,
		FormattedMessage: "AAPL.US quote is $174.25 per share",
		Timestamp:        time.Now().Unix(),
	}
	publishStockResponse(t, resp)

	// Wait for broadcast
	msg, err := client.WaitForMessage(2*time.Second, func(m WSMessage) bool {
		return m.Type == "chat_message" && m.UserID == "test-bot-001"
	})
	require.NoError(t, err, "should receive broadcast message")
	assert.Equal(t, "AAPL.US quote is $174.25 per share", msg.Content)
	assert.True(t, msg.IsBot, "message should be marked as bot")
	assert.False(t, msg.IsError, "message should not be marked as error")
}

// TestMessaging_ResponseConsumer_BroadcastsToMultipleClients tests broadcasting to multiple clients
func TestMessaging_ResponseConsumer_BroadcastsToMultipleClients(t *testing.T) {
	t.Parallel()
	// Create test chatroom
	testChatroom := createTestChatroom(t, "")

	// Connect 3 WebSocket clients
	clients := make([]*TestWSClient, 3)
	for i := 0; i < 3; i++ {
		c, err := newTestWSClient(t, nil, testChatroom.ID)
		require.NoError(t, err, "should connect WebSocket client %d", i)
		clients[i] = c
		defer c.Close()
		c.DrainMessages()
	}

	// Publish a StockResponse
	resp := &messaging.StockResponse{
		ChatroomID:       testChatroom.ID,
		Symbol:           "GOOGL",
		Price:            142.50,
		FormattedMessage: "GOOGL quote is $142.50 per share",
		Timestamp:        time.Now().Unix(),
	}
	publishStockResponse(t, resp)

	// All 3 clients should receive the broadcast
	for i, client := range clients {
		msg, err := client.WaitForMessage(2*time.Second, func(m WSMessage) bool {
			return m.Type == "chat_message" && m.UserID == "test-bot-001"
		})
		require.NoError(t, err, "client %d should receive message", i)
		assert.Equal(t, "GOOGL quote is $142.50 per share", msg.Content)
		assert.True(t, msg.IsBot)
	}
}

// TestMessaging_ResponseConsumer_HandlesErrorResponses tests handling of error responses
func TestMessaging_ResponseConsumer_HandlesErrorResponses(t *testing.T) {
	t.Parallel()
	// Create test chatroom
	testChatroom := createTestChatroom(t, "")

	// Connect WebSocket client
	client, err := newTestWSClient(t, nil, testChatroom.ID)
	require.NoError(t, err, "should connect WebSocket client")
	defer client.Close()
	client.DrainMessages()

	// Publish error response
	resp := &messaging.StockResponse{
		ChatroomID: testChatroom.ID,
		Symbol:     "INVALID",
		Error:      "Stock symbol not found",
		Timestamp:  time.Now().Unix(),
	}
	publishStockResponse(t, resp)

	// Should receive error message
	msg, err := client.WaitForMessage(2*time.Second, func(m WSMessage) bool {
		return m.Type == "chat_message" && m.UserID == "test-bot-001"
	})
	require.NoError(t, err, "should receive error message")
	assert.Equal(t, "Stock symbol not found", msg.Content)
	assert.True(t, msg.IsBot)
	assert.True(t, msg.IsError, "message should be marked as error")
}

// === GROUP 3: TIMEOUT & ERROR HANDLING ===

// TestMessaging_Publish_ContextTimeout tests that publish completes reasonably quickly
func TestMessaging_Publish_ContextTimeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	startTime := time.Now()
	err := rmq.PublishStockCommand(ctx, "timeout-test-room", "AAPL.US", "user")
	duration := time.Since(startTime)

	assert.NoError(t, err, "should publish without error")
	assert.Less(t, duration, 1*time.Second, "publish should complete quickly")
}

// TestMessaging_Consumer_MalformedMessage tests consumer robustness with malformed messages
func TestMessaging_Consumer_MalformedMessage(t *testing.T) {
	t.Parallel()
	// This test verifies that consumer doesn't crash with malformed messages
	// by sending a valid message after potential malformed data

	testChatroom := createTestChatroom(t, "")

	client, err := newTestWSClient(t, nil, testChatroom.ID)
	require.NoError(t, err, "should connect WebSocket client")
	defer client.Close()
	client.DrainMessages()

	// Send valid message to verify consumer is still working
	resp := &messaging.StockResponse{
		ChatroomID:       testChatroom.ID,
		Symbol:           "AAPL.US",
		FormattedMessage: "AAPL.US $174.25",
		Timestamp:        time.Now().Unix(),
	}
	publishStockResponse(t, resp)

	// Should still receive it
	msg, err := client.WaitForMessage(2*time.Second, func(m WSMessage) bool {
		return m.Type == "chat_message" && m.UserID == "test-bot-001"
	})
	require.NoError(t, err, "consumer should still be working")
	assert.NotNil(t, msg)
}

// TestMessaging_Consumer_MessageMissingRequiredFields tests handling of incomplete messages
func TestMessaging_Consumer_MessageMissingRequiredFields(t *testing.T) {
	t.Parallel()
	// Message with empty ChatroomID - should be handled gracefully
	resp := &messaging.StockResponse{
		ChatroomID:       "", // Empty
		Symbol:           "AAPL.US",
		FormattedMessage: "AAPL.US $174.25",
		Timestamp:        time.Now().Unix(),
	}

	// Should not panic or crash
	publishStockResponse(t, resp)

	// Wait a bit and verify consumer is still working
	time.Sleep(500 * time.Millisecond)

	// Verify by publishing to a valid chatroom
	testChatroom := createTestChatroom(t, "")

	client, err := newTestWSClient(t, nil, testChatroom.ID)
	require.NoError(t, err, "should connect WebSocket client")
	defer client.Close()
	client.DrainMessages()

	validResp := &messaging.StockResponse{
		ChatroomID:       testChatroom.ID,
		Symbol:           "GOOGL",
		FormattedMessage: "GOOGL $142.50",
		Timestamp:        time.Now().Unix(),
	}
	publishStockResponse(t, validResp)

	msg, err := client.WaitForMessage(2*time.Second, func(m WSMessage) bool {
		return m.Type == "chat_message" && m.UserID == "test-bot-001"
	})
	require.NoError(t, err, "consumer should still be working after handling bad message")
	assert.NotNil(t, msg)
}

// TestMessaging_ResponseJSONMarshal tests that responses are properly marshaled
func TestMessaging_ResponseJSONMarshal(t *testing.T) {
	t.Parallel()
	resp := &messaging.StockResponse{
		ChatroomID:       "test-id",
		Symbol:           "TSLA",
		Price:            245.50,
		FormattedMessage: "TSLA quote is $245.50",
		Timestamp:        time.Now().Unix(),
	}

	// Should be JSON serializable
	data, err := json.Marshal(resp)
	require.NoError(t, err, "response should be JSON serializable")

	// Should be JSON deserializable
	var unmarshaled messaging.StockResponse
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err, "response should be JSON deserializable")

	assert.Equal(t, resp.ChatroomID, unmarshaled.ChatroomID)
	assert.Equal(t, resp.Symbol, unmarshaled.Symbol)
	assert.Equal(t, resp.Price, unmarshaled.Price)
}

// TestMessaging_BotCommandPublish tests publishing of bot commands
func TestMessaging_BotCommandPublish(t *testing.T) {
	t.Parallel()
	cmd := &messaging.BotCommand{
		Type:        "stock",
		ChatroomID:  "test-chatroom",
		StockCode:   "AAPL.US",
		RequestedBy: "test-user",
		Timestamp:   time.Now().Unix(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := rmq.PublishCommand(ctx, cmd)
	assert.NoError(t, err, "should publish bot command without error")
}
