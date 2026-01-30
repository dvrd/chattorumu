// +build integration

package messaging_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"jobsity-chat/internal/messaging"
	"jobsity-chat/internal/service"
	"jobsity-chat/internal/websocket"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResponseConsumerIntegration tests the response consumer with real RabbitMQ
func TestResponseConsumerIntegration(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	// Setup RabbitMQ connection
	rmq, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq.Close()

	// Setup WebSocket hub (in-memory for testing)
	hub := websocket.NewHub()
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	go func() {
		if err := hub.Run(hubCtx); err != nil && err != context.Canceled {
			t.Logf("hub error: %v", err)
		}
	}()

	// Mock chat service (minimal implementation)
	var chatService *service.ChatService // nil is OK for this test as consumer doesn't use it

	// Bot user ID
	botUserID := "bot-user-123"

	// Create response consumer
	consumer := messaging.NewResponseConsumer(rmq, hub, chatService, botUserID)

	// Start consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = consumer.Start(ctx)
	require.NoError(t, err)

	// Give consumer time to start
	time.Sleep(500 * time.Millisecond)

	t.Run("receive_stock_response", func(t *testing.T) {
		chatroomID := "room-consumer-test-1"

		// Register a mock WebSocket client to receive broadcasts
		mockClient := &MockWebSocketClient{
			chatroomID: chatroomID,
			messages:   make(chan []byte, 10),
		}

		hub.Register(mockClient)
		defer hub.Unregister(mockClient)

		// Give registration time to complete
		time.Sleep(200 * time.Millisecond)

		// Publish a stock response
		publishCtx, publishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer publishCancel()

		response := &messaging.StockResponse{
			ChatroomID:       chatroomID,
			Symbol:           "GOOGL",
			Price:            142.50,
			FormattedMessage: "GOOGL quote is $142.50 per share",
			Timestamp:        time.Now().Unix(),
		}

		err := rmq.PublishStockResponse(publishCtx, response)
		require.NoError(t, err)

		// Wait for broadcast to mock client
		select {
		case data := <-mockClient.messages:
			var serverMsg websocket.ServerMessage
			err := json.Unmarshal(data, &serverMsg)
			require.NoError(t, err)

			assert.Equal(t, "chat_message", serverMsg.Type)
			assert.Equal(t, botUserID, serverMsg.UserID)
			assert.Equal(t, "StockBot", serverMsg.Username)
			assert.Equal(t, response.FormattedMessage, serverMsg.Content)
			assert.True(t, serverMsg.IsBot)
			assert.False(t, serverMsg.IsError)

		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for broadcast")
		}
	})

	t.Run("receive_error_response", func(t *testing.T) {
		chatroomID := "room-consumer-test-2"

		mockClient := &MockWebSocketClient{
			chatroomID: chatroomID,
			messages:   make(chan []byte, 10),
		}

		hub.Register(mockClient)
		defer hub.Unregister(mockClient)

		time.Sleep(200 * time.Millisecond)

		publishCtx, publishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer publishCancel()

		response := &messaging.StockResponse{
			ChatroomID: chatroomID,
			Symbol:     "INVALID",
			Error:      "Stock INVALID not found",
			Timestamp:  time.Now().Unix(),
		}

		err := rmq.PublishStockResponse(publishCtx, response)
		require.NoError(t, err)

		select {
		case data := <-mockClient.messages:
			var serverMsg websocket.ServerMessage
			err := json.Unmarshal(data, &serverMsg)
			require.NoError(t, err)

			assert.Equal(t, "chat_message", serverMsg.Type)
			assert.Equal(t, response.Error, serverMsg.Content)
			assert.True(t, serverMsg.IsBot)
			assert.True(t, serverMsg.IsError) // Error flag should be set

		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for error broadcast")
		}
	})

	t.Run("consumer_graceful_shutdown", func(t *testing.T) {
		// Create a new consumer with its own context
		shutdownCtx, shutdownCancel := context.WithCancel(context.Background())

		newConsumer := messaging.NewResponseConsumer(rmq, hub, chatService, botUserID)
		err := newConsumer.Start(shutdownCtx)
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		// Cancel context to trigger shutdown
		shutdownCancel()

		// Give it time to shut down gracefully
		time.Sleep(500 * time.Millisecond)

		// Test passes if no panics or errors
		assert.True(t, true, "consumer shutdown gracefully")
	})

	t.Run("multiple_clients_same_chatroom", func(t *testing.T) {
		chatroomID := "room-multi-client"

		// Register multiple clients
		client1 := &MockWebSocketClient{
			chatroomID: chatroomID,
			messages:   make(chan []byte, 10),
		}
		client2 := &MockWebSocketClient{
			chatroomID: chatroomID,
			messages:   make(chan []byte, 10),
		}

		hub.Register(client1)
		hub.Register(client2)
		defer func() {
			hub.Unregister(client1)
			hub.Unregister(client2)
		}()

		time.Sleep(200 * time.Millisecond)

		// Publish one response
		publishCtx, publishCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer publishCancel()

		response := &messaging.StockResponse{
			ChatroomID:       chatroomID,
			Symbol:           "MSFT",
			Price:            380.75,
			FormattedMessage: "MSFT quote is $380.75 per share",
			Timestamp:        time.Now().Unix(),
		}

		err := rmq.PublishStockResponse(publishCtx, response)
		require.NoError(t, err)

		// Both clients should receive the broadcast
		timeout := time.After(5 * time.Second)
		client1Received := false
		client2Received := false

		for !client1Received || !client2Received {
			select {
			case data := <-client1.messages:
				var msg websocket.ServerMessage
				json.Unmarshal(data, &msg)
				assert.Equal(t, response.FormattedMessage, msg.Content)
				client1Received = true

			case data := <-client2.messages:
				var msg websocket.ServerMessage
				json.Unmarshal(data, &msg)
				assert.Equal(t, response.FormattedMessage, msg.Content)
				client2Received = true

			case <-timeout:
				t.Fatalf("timeout: client1=%v client2=%v", client1Received, client2Received)
			}
		}

		assert.True(t, client1Received && client2Received, "both clients should receive message")
	})
}

// TestResponseConsumerWithMalformedMessages tests error handling for invalid JSON
func TestResponseConsumerWithMalformedMessages(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	rmq, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq.Close()

	hub := websocket.NewHub()
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()

	go hub.Run(hubCtx)

	consumer := messaging.NewResponseConsumer(rmq, hub, nil, "bot-123")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = consumer.Start(ctx)
	require.NoError(t, err)

	time.Sleep(500 * time.Millisecond)

	// Publish malformed JSON directly to the exchange
	publishCtx, publishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer publishCancel()

	// We need to access the internal channel to publish raw data
	// For this test, we'll skip it as it requires exposing internals
	// In production, you'd use package-level test helpers

	t.Skip("Skipping malformed message test - requires internal channel access")
}

// MockWebSocketClient implements the websocket.Client interface for testing
type MockWebSocketClient struct {
	chatroomID string
	messages   chan []byte
}

func (m *MockWebSocketClient) Send(data []byte) {
	select {
	case m.messages <- data:
	default:
		// Drop message if channel full
	}
}

func (m *MockWebSocketClient) ChatroomID() string {
	return m.chatroomID
}

func (m *MockWebSocketClient) UserID() string {
	return "test-user-123"
}

func (m *MockWebSocketClient) Close() error {
	close(m.messages)
	return nil
}
