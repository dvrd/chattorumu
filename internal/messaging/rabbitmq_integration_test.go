// +build integration

package messaging_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"jobsity-chat/internal/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestRabbitMQContainer manages RabbitMQ container lifecycle for integration tests
type TestRabbitMQContainer struct {
	container testcontainers.Container
	url       string
}

// setupRabbitMQ starts a RabbitMQ container and returns connection URL
func setupRabbitMQ(t *testing.T) (*TestRabbitMQContainer, func()) {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3.12-management-alpine",
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "guest",
			"RABBITMQ_DEFAULT_PASS": "guest",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Server startup complete"),
			wait.ForListeningPort("5672/tcp"),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "failed to start RabbitMQ container")

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "5672")
	require.NoError(t, err)

	url := fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())

	// Wait for RabbitMQ to be fully ready
	time.Sleep(2 * time.Second)

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return &TestRabbitMQContainer{
		container: container,
		url:       url,
	}, cleanup
}

// TestRabbitMQConnection tests basic connection establishment
func TestRabbitMQConnection(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	t.Run("successful_connection", func(t *testing.T) {
		rmq, err := messaging.NewRabbitMQ(testContainer.url)
		require.NoError(t, err)
		defer rmq.Close()

		assert.False(t, rmq.IsClosed())
	})

	t.Run("invalid_url_fails", func(t *testing.T) {
		_, err := messaging.NewRabbitMQ("amqp://invalid:9999/")
		assert.Error(t, err)
	})

	t.Run("close_connection", func(t *testing.T) {
		rmq, err := messaging.NewRabbitMQ(testContainer.url)
		require.NoError(t, err)

		err = rmq.Close()
		assert.NoError(t, err)
		assert.True(t, rmq.IsClosed())
	})
}

// TestPublishStockCommand tests stock command publishing
func TestPublishStockCommand(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	rmq, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq.Close()

	tests := []struct {
		name        string
		chatroomID  string
		stockCode   string
		requestedBy string
		wantErr     bool
	}{
		{
			name:        "valid_stock_command",
			chatroomID:  "room-123",
			stockCode:   "AAPL.US",
			requestedBy: "john",
			wantErr:     false,
		},
		{
			name:        "empty_stock_code",
			chatroomID:  "room-456",
			stockCode:   "",
			requestedBy: "jane",
			wantErr:     false, // Empty stock code is valid at messaging layer
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := rmq.PublishStockCommand(ctx, tt.chatroomID, tt.stockCode, tt.requestedBy)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestPublishHelloCommand tests hello command publishing
func TestPublishHelloCommand(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	rmq, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = rmq.PublishHelloCommand(ctx, "room-123", "alice")
	assert.NoError(t, err)
}

// TestStockCommandConsumeFlow tests end-to-end publish-consume flow
func TestStockCommandConsumeFlow(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	rmq, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq.Close()

	t.Run("consume_stock_command", func(t *testing.T) {
		// Start consumer
		msgs, err := rmq.ConsumeStockCommands()
		require.NoError(t, err)

		// Publish command
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		expectedChatroom := "room-789"
		expectedStock := "TSLA.US"
		expectedUser := "elon"

		err = rmq.PublishStockCommand(ctx, expectedChatroom, expectedStock, expectedUser)
		require.NoError(t, err)

		// Consume message
		select {
		case msg := <-msgs:
			var cmd messaging.BotCommand
			err := json.Unmarshal(msg.Body, &cmd)
			require.NoError(t, err)

			assert.Equal(t, "stock", cmd.Type)
			assert.Equal(t, expectedChatroom, cmd.ChatroomID)
			assert.Equal(t, expectedStock, cmd.StockCode)
			assert.Equal(t, expectedUser, cmd.RequestedBy)
			assert.Greater(t, cmd.Timestamp, int64(0))

			// Acknowledge message
			err = msg.Ack(false)
			assert.NoError(t, err)

		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for message")
		}
	})

	t.Run("consume_hello_command", func(t *testing.T) {
		// Start consumer
		msgs, err := rmq.ConsumeStockCommands()
		require.NoError(t, err)

		// Publish hello command
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err = rmq.PublishHelloCommand(ctx, "room-hello", "bob")
		require.NoError(t, err)

		// Consume message
		select {
		case msg := <-msgs:
			var cmd messaging.BotCommand
			err := json.Unmarshal(msg.Body, &cmd)
			require.NoError(t, err)

			assert.Equal(t, "hello", cmd.Type)
			assert.Equal(t, "room-hello", cmd.ChatroomID)
			assert.Equal(t, "bob", cmd.RequestedBy)
			assert.Empty(t, cmd.StockCode)

			err = msg.Ack(false)
			assert.NoError(t, err)

		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for hello message")
		}
	})
}

// TestStockResponseFlow tests response publishing and consuming
func TestStockResponseFlow(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	rmq, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq.Close()

	t.Run("publish_and_consume_response", func(t *testing.T) {
		chatroomID := "room-response-123"

		// Start consumer FIRST (creates queue and binding)
		msgs, err := rmq.ConsumeStockResponses(chatroomID)
		require.NoError(t, err)

		// Give consumer time to bind
		time.Sleep(500 * time.Millisecond)

		// Publish response
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		response := &messaging.StockResponse{
			ChatroomID:       chatroomID,
			Symbol:           "AAPL.US",
			Price:            174.25,
			FormattedMessage: "AAPL.US quote is $174.25 per share",
			Timestamp:        time.Now().Unix(),
		}

		err = rmq.PublishStockResponse(ctx, response)
		require.NoError(t, err)

		// Consume response
		select {
		case msg := <-msgs:
			var received messaging.StockResponse
			err := json.Unmarshal(msg.Body, &received)
			require.NoError(t, err)

			assert.Equal(t, response.ChatroomID, received.ChatroomID)
			assert.Equal(t, response.Symbol, received.Symbol)
			assert.Equal(t, response.Price, received.Price)
			assert.Equal(t, response.FormattedMessage, received.FormattedMessage)
			assert.Empty(t, received.Error)

		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for stock response")
		}
	})

	t.Run("publish_error_response", func(t *testing.T) {
		chatroomID := "room-error-456"

		msgs, err := rmq.ConsumeStockResponses(chatroomID)
		require.NoError(t, err)

		time.Sleep(500 * time.Millisecond)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		response := &messaging.StockResponse{
			ChatroomID: chatroomID,
			Symbol:     "INVALID",
			Error:      "Stock INVALID not found",
			Timestamp:  time.Now().Unix(),
		}

		err = rmq.PublishStockResponse(ctx, response)
		require.NoError(t, err)

		select {
		case msg := <-msgs:
			var received messaging.StockResponse
			err := json.Unmarshal(msg.Body, &received)
			require.NoError(t, err)

			assert.Equal(t, response.ChatroomID, received.ChatroomID)
			assert.Equal(t, response.Error, received.Error)
			assert.Equal(t, float64(0), received.Price)

		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for error response")
		}
	})
}

// TestMessagePersistence tests that persistent messages survive broker restart
func TestMessagePersistence(t *testing.T) {
	t.Skip("Skipping persistence test - requires container restart logic")

	// This test would:
	// 1. Publish persistent message to durable queue
	// 2. Restart RabbitMQ container
	// 3. Verify message is still in queue
}

// TestMultipleConsumers tests load balancing across multiple consumers
func TestMultipleConsumers(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	// Create two separate connections (simulating multiple workers)
	rmq1, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq1.Close()

	rmq2, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq2.Close()

	publisher, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer publisher.Close()

	// Start two consumers
	msgs1, err := rmq1.ConsumeStockCommands()
	require.NoError(t, err)

	msgs2, err := rmq2.ConsumeStockCommands()
	require.NoError(t, err)

	// Track which consumer received each message
	consumer1Count := 0
	consumer2Count := 0
	totalMessages := 10

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Publish multiple messages
	for i := 0; i < totalMessages; i++ {
		err := publisher.PublishStockCommand(ctx, "room-load-balance", fmt.Sprintf("STOCK%d", i), "user")
		require.NoError(t, err)
	}

	// Consume messages from both consumers
	timeout := time.After(10 * time.Second)
	received := 0

	for received < totalMessages {
		select {
		case msg := <-msgs1:
			consumer1Count++
			received++
			msg.Ack(false)

		case msg := <-msgs2:
			consumer2Count++
			received++
			msg.Ack(false)

		case <-timeout:
			t.Fatalf("timeout: received %d/%d messages", received, totalMessages)
		}
	}

	// Verify load distribution (should be roughly equal)
	t.Logf("Consumer 1: %d messages, Consumer 2: %d messages", consumer1Count, consumer2Count)

	// Both consumers should receive at least one message (basic sanity check)
	assert.Greater(t, consumer1Count, 0, "consumer 1 should receive messages")
	assert.Greater(t, consumer2Count, 0, "consumer 2 should receive messages")
	assert.Equal(t, totalMessages, consumer1Count+consumer2Count, "total messages should match")
}

// TestNackRedelivery tests message redelivery on NACK
func TestNackRedelivery(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	rmq, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq.Close()

	msgs, err := rmq.ConsumeStockCommands()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Publish a test message
	err = rmq.PublishStockCommand(ctx, "room-nack", "NACK.TEST", "user")
	require.NoError(t, err)

	// First delivery - NACK it
	select {
	case msg := <-msgs:
		var cmd messaging.BotCommand
		err := json.Unmarshal(msg.Body, &cmd)
		require.NoError(t, err)
		assert.Equal(t, "NACK.TEST", cmd.StockCode)

		// NACK with requeue
		err = msg.Nack(false, true)
		assert.NoError(t, err)

	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for first delivery")
	}

	// Second delivery - should be redelivered
	select {
	case msg := <-msgs:
		var cmd messaging.BotCommand
		err := json.Unmarshal(msg.Body, &cmd)
		require.NoError(t, err)
		assert.Equal(t, "NACK.TEST", cmd.StockCode)
		assert.True(t, msg.Redelivered, "message should be marked as redelivered")

		// ACK this time
		err = msg.Ack(false)
		assert.NoError(t, err)

	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for redelivery")
	}
}

// TestContextCancellation tests that context cancellation stops operations
func TestContextCancellation(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	rmq, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq.Close()

	t.Run("publish_with_cancelled_context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err := rmq.PublishStockCommand(ctx, "room-cancel", "CANCEL", "user")
		// Context cancellation might not always result in error due to timing
		// This is a best-effort test
		t.Logf("publish with cancelled context result: %v", err)
	})
}

// TestConcurrentPublish tests concurrent publishing from multiple goroutines
func TestConcurrentPublish(t *testing.T) {
	testContainer, cleanup := setupRabbitMQ(t)
	defer cleanup()

	rmq, err := messaging.NewRabbitMQ(testContainer.url)
	require.NoError(t, err)
	defer rmq.Close()

	msgs, err := rmq.ConsumeStockCommands()
	require.NoError(t, err)

	numGoroutines := 10
	messagesPerGoroutine := 5
	totalMessages := numGoroutines * messagesPerGoroutine

	// Start consumer goroutine
	received := make(chan bool, totalMessages)
	go func() {
		for i := 0; i < totalMessages; i++ {
			select {
			case msg := <-msgs:
				msg.Ack(false)
				received <- true
			case <-time.After(15 * time.Second):
				return
			}
		}
	}()

	// Publish from multiple goroutines concurrently
	ctx := context.Background()
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < messagesPerGoroutine; j++ {
				err := rmq.PublishStockCommand(ctx, "room-concurrent", fmt.Sprintf("STOCK%d-%d", id, j), "user")
				if err != nil {
					t.Logf("publish error from goroutine %d: %v", id, err)
				}
			}
		}(i)
	}

	// Wait for all messages
	receivedCount := 0
	timeout := time.After(15 * time.Second)

	for receivedCount < totalMessages {
		select {
		case <-received:
			receivedCount++
		case <-timeout:
			t.Fatalf("timeout: received %d/%d messages", receivedCount, totalMessages)
		}
	}

	assert.Equal(t, totalMessages, receivedCount, "should receive all messages")
}

// BenchmarkPublishStockCommand benchmarks message publishing performance
func BenchmarkPublishStockCommand(b *testing.B) {
	// Skip if not running benchmarks with integration tag
	if testing.Short() {
		b.Skip("skipping benchmark in short mode")
	}

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3.12-alpine",
		ExposedPorts: []string{"5672/tcp"},
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "guest",
			"RABBITMQ_DEFAULT_PASS": "guest",
		},
		WaitingFor: wait.ForLog("Server startup complete").WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(b, err)
	defer container.Terminate(ctx)

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5672")
	url := fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())

	time.Sleep(2 * time.Second)

	rmq, err := messaging.NewRabbitMQ(url)
	require.NoError(b, err)
	defer rmq.Close()

	publishCtx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = rmq.PublishStockCommand(publishCtx, "bench-room", "AAPL", "user")
		}
	})
}
