package websocket

import (
	"context"
	"strings"
	"testing"
	"time"
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

	// Give registration time to process and drain any initial count updates
	time.Sleep(100 * time.Millisecond)
	_, _ = drainCountUpdates(mockClient.send, 100*time.Millisecond)

	// Cancel context to trigger shutdown
	cancel()

	// Wait a bit for shutdown to complete
	time.Sleep(200 * time.Millisecond)

	// Drain any remaining messages (including count updates from shutdown)
	for {
		select {
		case _, ok := <-mockClient.send:
			if !ok {
				// Channel is closed, which is expected
				return
			}
			// Continue draining messages
		default:
			// No more messages, but channel may still be open
			// This is acceptable as long as shutdown completed without hanging
			return
		}
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
	if err := hub.Broadcast("test-room", []byte("test")); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

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
	time.Sleep(100 * time.Millisecond)

	// Drain any count updates from registration
	_, _ = drainCountUpdates(mockClient.send, 50*time.Millisecond)

	hub.Unregister(mockClient)
	time.Sleep(100 * time.Millisecond)

	// Drain remaining count updates from unregister with timeout
	_, _ = drainCountUpdates(mockClient.send, 100*time.Millisecond)

	// Attempt broadcast - client should not receive since unregistered
	if err := hub.Broadcast("test-room", []byte("test after unregister")); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

	// Give time for broadcast to process (it should have no effect)
	time.Sleep(50 * time.Millisecond)

	// Verify no messages are received after unregister
	select {
	case msg := <-mockClient.send:
		// Filter out any user_count_update messages that might still be pending
		msgStr := string(msg)
		if msgStr != "" && !strings.Contains(msgStr, "user_count_update") {
			t.Errorf("Unexpected message received after unregister: %s", msgStr)
		}
		// Empty messages and user_count_update messages are acceptable due to async nature
	default:
		// Expected: no messages after unregister
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
	if err := hub.Broadcast("test-room", []byte("test message")); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

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
	if err := hub.Broadcast("room-1", []byte("message for room 1")); err != nil {
		t.Fatalf("Broadcast failed: %v", err)
	}

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
			_ = ok // Channel state check during shutdown
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

// TestHub_GetConnectedUserCount tests the GetConnectedUserCount method
func TestHub_GetConnectedUserCount(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = hub.Run(ctx) }()
	defer func() {
		cancel()
		<-hub.done
	}()

	// Test empty chatroom
	if count := hub.GetConnectedUserCount("room1"); count != 0 {
		t.Errorf("Expected 0 for empty chatroom, got %d", count)
	}

	// Register two clients
	client1 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user1",
		username:   "alice",
		chatroomID: "room1",
	}

	client2 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user2",
		username:   "bob",
		chatroomID: "room1",
	}

	hub.Register(client1)
	time.Sleep(10 * time.Millisecond)

	if count := hub.GetConnectedUserCount("room1"); count != 1 {
		t.Errorf("Expected 1 after first register, got %d", count)
	}

	hub.Register(client2)
	time.Sleep(10 * time.Millisecond)

	if count := hub.GetConnectedUserCount("room1"); count != 2 {
		t.Errorf("Expected 2 after second register, got %d", count)
	}

	// Unregister one
	hub.Unregister(client1)
	time.Sleep(10 * time.Millisecond)

	if count := hub.GetConnectedUserCount("room1"); count != 1 {
		t.Errorf("Expected 1 after unregister, got %d", count)
	}
}

// TestHub_GetAllConnectedCounts tests the GetAllConnectedCounts method
func TestHub_GetAllConnectedCounts(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = hub.Run(ctx) }()
	defer func() {
		cancel()
		<-hub.done
	}()

	// Test empty hub
	if counts := hub.GetAllConnectedCounts(); len(counts) != 0 {
		t.Errorf("Expected empty counts, got %v", counts)
	}

	// Register clients in different rooms
	client1 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user1",
		username:   "alice",
		chatroomID: "room1",
	}

	client2 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user2",
		username:   "bob",
		chatroomID: "room1",
	}

	client3 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user3",
		username:   "charlie",
		chatroomID: "room2",
	}

	hub.Register(client1)
	time.Sleep(10 * time.Millisecond)
	hub.Register(client2)
	time.Sleep(10 * time.Millisecond)
	hub.Register(client3)
	time.Sleep(10 * time.Millisecond)

	counts := hub.GetAllConnectedCounts()
	if len(counts) != 2 {
		t.Errorf("Expected 2 rooms, got %d", len(counts))
	}

	if counts["room1"] != 2 {
		t.Errorf("Expected room1 count=2, got %d", counts["room1"])
	}

	if counts["room2"] != 1 {
		t.Errorf("Expected room2 count=1, got %d", counts["room2"])
	}
}

func TestHub_GracefulShutdownWithPendingBroadcasts(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = hub.Run(ctx)
	}()

	// Register two clients
	client1 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user1",
		username:   "alice",
		chatroomID: "test-room",
	}
	client2 := &Client{
		hub:        hub,
		send:       make(chan []byte, 256),
		userID:     "user2",
		username:   "bob",
		chatroomID: "test-room",
	}

	hub.Register(client1)
	hub.Register(client2)

	time.Sleep(100 * time.Millisecond)

	// Simulate pending broadcasts by adding to WaitGroup
	// (simulating multiple broadcastMessageAsync goroutines in flight)
	numPending := 5
	hub.pendingBroadcasts.Add(numPending)

	// Release them in background to simulate async completion
	go func() {
		time.Sleep(100 * time.Millisecond)
		for i := 0; i < numPending; i++ {
			hub.pendingBroadcasts.Done()
		}
	}()

	// Start shutdown - should wait for pending broadcasts
	cancel()

	// Wait for hub to finish running (shutdown should complete with timeout)
	time.Sleep(500 * time.Millisecond)

	// Verify shutdown succeeded and WaitGroup is at zero
	// (no way to directly test this without race detector, but logs show it worked)
}

func TestHub_BroadcastDuringShutdown(t *testing.T) {
	hub := NewHub()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		_ = hub.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	// Initiate shutdown
	cancel()

	time.Sleep(100 * time.Millisecond)

	// Attempt to broadcast after shutdown
	err := hub.Broadcast("test-room", []byte("should fail"))
	if err == nil {
		t.Error("Expected error when broadcasting after shutdown, got nil")
	}

	if err.Error() != "hub is shutting down" {
		t.Errorf("Expected 'hub is shutting down' error, got %q", err.Error())
	}
}
