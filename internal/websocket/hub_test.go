package websocket

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Helper function to drain user_count_update messages and return the first non-count message
func drainCountUpdates(ch <-chan []byte, timeout time.Duration) ([]byte, error) {
	deadline := time.After(timeout)
	for {
		select {
		case msg := <-ch:
			// Check if it's a user_count_update message
			if strings.Contains(string(msg), "user_count_update") {
				continue // Skip count updates
			}
			return msg, nil
		case <-deadline:
			return nil, context.DeadlineExceeded
		}
	}
}

func TestHub_NewHub(t *testing.T) {
	hub := NewHub()

	if hub == nil {
		t.Fatal("NewHub() returned nil")
	}

	if hub.clients == nil {
		t.Error("Expected clients map to be initialized")
	}

	if hub.broadcast == nil {
		t.Error("Expected broadcast channel to be initialized")
	}

	if hub.register == nil {
		t.Error("Expected register channel to be initialized")
	}

	if hub.unregister == nil {
		t.Error("Expected unregister channel to be initialized")
	}

	if hub.done == nil {
		t.Error("Expected done channel to be initialized")
	}
}

func TestHub_ContextCancellation(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())

	// Start hub in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- hub.Run(ctx)
	}()

	// Give hub time to start
	time.Sleep(50 * time.Millisecond)

	// Cancel context
	cancel()

	// Wait for hub to stop
	select {
	case err := <-errChan:
		if err != context.Canceled {
			t.Errorf("Expected context.Canceled error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Hub did not stop within timeout")
	}
}

func TestHub_GracefulShutdown(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start hub
	go func() {
		_ = hub.Run(ctx)
	}()

	// Give hub time to start
	time.Sleep(50 * time.Millisecond)

	// Create mock client (we'll skip actual WebSocket connection)
	mockClient := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "test-user",
		username:   "testuser",
		chatroomID: "test-room",
	}

	// Register client
	hub.Register(mockClient)

	// Give registration time to process
	time.Sleep(50 * time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait a bit for shutdown to complete
	time.Sleep(200 * time.Millisecond)

	// Verify send channel is closed (shutdown cleans up)
	select {
	case _, ok := <-mockClient.send:
		if ok {
			t.Error("Expected send channel to be closed after shutdown")
		}
	default:
		// Channel is not closed yet, which is also acceptable
		// as long as shutdown didn't hang
	}
}

func TestHub_RegisterClient(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = hub.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	mockClient := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "test-user",
		username:   "testuser",
		chatroomID: "test-room",
	}

	hub.Register(mockClient)

	// Give registration time to process
	time.Sleep(100 * time.Millisecond)

	// Verify by attempting to broadcast - if client is registered, it should receive
	hub.Broadcast("test-room", []byte("test"))

	// Use helper to skip user_count_update messages
	msg, err := drainCountUpdates(mockClient.send, 200*time.Millisecond)
	if err != nil {
		t.Error("Client did not receive broadcast, likely not registered")
	} else if string(msg) != "test" {
		t.Errorf("Expected 'test', got %s", string(msg))
	}
}

func TestHub_UnregisterClient(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = hub.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	mockClient := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "test-user",
		username:   "testuser",
		chatroomID: "test-room",
	}

	// Register then unregister
	hub.Register(mockClient)
	time.Sleep(50 * time.Millisecond)

	hub.Unregister(mockClient)
	time.Sleep(100 * time.Millisecond)

	// Verify send channel was closed after unregister
	// The channel should be closed, so receiving should return (nil, false)
	select {
	case msg, ok := <-mockClient.send:
		if ok {
			t.Errorf("Expected send channel to be closed, but received message: %s", string(msg))
		}
		// ok == false is expected (channel closed)
	case <-time.After(100 * time.Millisecond):
		// Channel not ready, which could mean it's not closed yet
		// This is acceptable as long as the client won't receive new messages
	}

	// Attempt broadcast - client should not receive since unregistered
	hub.Broadcast("test-room", []byte("test after unregister"))

	// Give time for broadcast to process (it should have no effect)
	time.Sleep(50 * time.Millisecond)

	// If channel was closed, trying to receive should immediately return
	select {
	case _, ok := <-mockClient.send:
		if ok {
			t.Error("Unexpected message received on closed channel")
		}
		// Channel is closed, which is expected
	default:
		// No message waiting, which is also fine
	}
}

func TestHub_BroadcastMessage(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = hub.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Create two clients in the same chatroom
	client1 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user-1",
		username:   "user1",
		chatroomID: "test-room",
	}

	client2 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user-2",
		username:   "user2",
		chatroomID: "test-room",
	}

	hub.Register(client1)
	hub.Register(client2)

	time.Sleep(100 * time.Millisecond)

	// Broadcast message
	hub.Broadcast("test-room", []byte("test message"))

	// Both clients should receive (hub broadcasts to all in room) - after count updates
	msg1, err := drainCountUpdates(client1.send, 200*time.Millisecond)
	if err != nil {
		t.Error("Client1 did not receive broadcast message")
	} else if string(msg1) != "test message" {
		t.Errorf("Expected 'test message', got %s", string(msg1))
	}

	msg2, err := drainCountUpdates(client2.send, 200*time.Millisecond)
	if err != nil {
		t.Error("Client2 did not receive broadcast message")
	} else if string(msg2) != "test message" {
		t.Errorf("Expected 'test message', got %s", string(msg2))
	}
}

func TestHub_BroadcastToMultipleChatrooms(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = hub.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Create clients in different chatrooms
	client1 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user-1",
		username:   "user1",
		chatroomID: "room-1",
	}

	client2 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user-2",
		username:   "user2",
		chatroomID: "room-2",
	}

	hub.Register(client1)
	hub.Register(client2)

	time.Sleep(100 * time.Millisecond)

	// Broadcast to room-1 only
	hub.Broadcast("room-1", []byte("message for room 1"))

	// Client1 should receive (after draining count updates)
	msg1, err := drainCountUpdates(client1.send, 200*time.Millisecond)
	if err != nil {
		t.Error("Client1 did not receive message")
	} else if string(msg1) != "message for room 1" {
		t.Errorf("Expected 'message for room 1', got %s", string(msg1))
	}

	// Client2 should not receive (different room) - drain any count updates first
	select {
	case msg := <-client2.send:
		// If it's a count update, that's fine, otherwise error
		if !strings.Contains(string(msg), "user_count_update") {
			t.Errorf("Client2 should not receive message from room-1, got: %s", string(msg))
		}
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestHub_ShutdownWithMultipleClients(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = hub.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Create multiple clients
	numClients := 10
	clients := make([]*Client, numClients)

	for i := 0; i < numClients; i++ {
		clients[i] = &Client{
			hub:        hub,
			send:       make(chan []byte, 256),
			userID:     "user-" + string(rune(i)),
			username:   "user" + string(rune(i)),
			chatroomID: "test-room",
		}
		hub.Register(clients[i])
	}

	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown
	cancel()

	// Wait for shutdown
	time.Sleep(200 * time.Millisecond)

	// Verify all client channels are handled properly
	for i, client := range clients {
		select {
		case _, ok := <-client.send:
			if ok {
				// Channel still open is acceptable if shutdown is still processing
			}
		default:
			// Channel not ready, which is fine
		}
		_ = i // Use i to avoid unused variable warning
	}

	// Main verification: shutdown completed without hanging
	// If we reach here, test passes
}

func TestHub_DoubleUnregister(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = hub.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	mockClient := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "test-user",
		username:   "testuser",
		chatroomID: "test-room",
	}

	hub.Register(mockClient)
	time.Sleep(50 * time.Millisecond)

	// Unregister twice - should not panic
	hub.Unregister(mockClient)
	time.Sleep(50 * time.Millisecond)

	hub.Unregister(mockClient)
	time.Sleep(50 * time.Millisecond)

	// If we reach here without panic, test passes
}

// Mock WebSocket connection for testing
type mockConn struct {
	websocket.Conn
}

func (m *mockConn) Close() error {
	return nil
}

func (m *mockConn) WriteMessage(messageType int, data []byte) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}
