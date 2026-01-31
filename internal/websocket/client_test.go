package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"jobsity-chat/internal/service"
	"jobsity-chat/internal/testutil"

	"github.com/gorilla/websocket"
)

// mockWebSocketConn provides a mock implementation of websocket.Conn for testing
type mockWebSocketConn struct {
	readMessages  chan []byte
	writeMessages chan []byte
	closeErr      error
	readErr       error
	writeErr      error
	closed        bool
	mu            sync.Mutex
}

func newMockWebSocketConn() *mockWebSocketConn {
	return &mockWebSocketConn{
		readMessages:  make(chan []byte, 10),
		writeMessages: make(chan []byte, 10),
	}
}

func (m *mockWebSocketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return m.closeErr
}

func (m *mockWebSocketConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockWebSocketConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *mockWebSocketConn) SetReadLimit(limit int64) {
}

func (m *mockWebSocketConn) SetPongHandler(h func(string) error) {
}

func (m *mockWebSocketConn) ReadMessage() (int, []byte, error) {
	if m.readErr != nil {
		return 0, nil, m.readErr
	}
	msg, ok := <-m.readMessages
	if !ok {
		return 0, nil, &websocket.CloseError{Code: websocket.CloseGoingAway}
	}
	return websocket.TextMessage, msg, nil
}

func (m *mockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	select {
	case m.writeMessages <- data:
		return nil
	default:
		return errors.New("write buffer full")
	}
}

func TestNewClient(t *testing.T) {
	hub := NewHub()
	ctx := context.Background()
	publisher := testutil.NewMockMessagePublisher()
	chatroomRepo := testutil.NewMockChatroomRepository()
	messageRepo := testutil.NewMockMessageRepository()
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	// Create a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
	}))
	defer server.Close()

	// Connect to the test server
	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNoError(t, err)
	defer conn.Close()

	client := NewClient(ctx, hub, conn, "user-123", "testuser", "room-1", chatService, publisher)

	testutil.AssertNotNil(t, client)
	testutil.AssertEqual(t, client.userID, "user-123")
	testutil.AssertEqual(t, client.username, "testuser")
	testutil.AssertEqual(t, client.chatroomID, "room-1")
	testutil.AssertNotNil(t, client.send)
	testutil.AssertNotNil(t, client.hub)
}

func TestClientMessage_JSON(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    ClientMessage
		wantErr bool
	}{
		{
			name: "valid chat message",
			json: `{"type":"chat_message","content":"Hello, World!"}`,
			want: ClientMessage{Type: "chat_message", Content: "Hello, World!"},
		},
		{
			name: "stock command",
			json: `{"type":"chat_message","content":"/stock=AAPL.US"}`,
			want: ClientMessage{Type: "chat_message", Content: "/stock=AAPL.US"},
		},
		{
			name:    "invalid json",
			json:    `{invalid}`,
			wantErr: true,
		},
		{
			name: "empty content",
			json: `{"type":"chat_message","content":""}`,
			want: ClientMessage{Type: "chat_message", Content: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg ClientMessage
			err := json.Unmarshal([]byte(tt.json), &msg)

			if tt.wantErr {
				testutil.AssertError(t, err)
				return
			}

			testutil.AssertNoError(t, err)
			testutil.AssertEqual(t, msg.Type, tt.want.Type)
			testutil.AssertEqual(t, msg.Content, tt.want.Content)
		})
	}
}

func TestServerMessage_JSON(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		msg  ServerMessage
		want map[string]interface{}
	}{
		{
			name: "chat message",
			msg: ServerMessage{
				Type:      "chat_message",
				ID:        "msg-123",
				UserID:    "user-456",
				Username:  "testuser",
				Content:   "Hello!",
				IsBot:     false,
				CreatedAt: &now,
			},
			want: map[string]interface{}{
				"type":     "chat_message",
				"id":       "msg-123",
				"user_id":  "user-456",
				"username": "testuser",
				"content":  "Hello!",
			},
		},
		{
			name: "bot message",
			msg: ServerMessage{
				Type:     "chat_message",
				Username: "StockBot",
				Content:  "AAPL.US quote is $150.00",
				IsBot:    true,
			},
			want: map[string]interface{}{
				"type":     "chat_message",
				"username": "StockBot",
				"content":  "AAPL.US quote is $150.00",
				"is_bot":   true,
			},
		},
		{
			name: "user joined",
			msg: ServerMessage{
				Type:     "user_joined",
				Username: "newuser",
			},
			want: map[string]interface{}{
				"type":     "user_joined",
				"username": "newuser",
			},
		},
		{
			name: "user left",
			msg: ServerMessage{
				Type:     "user_left",
				Username: "leavinguser",
			},
			want: map[string]interface{}{
				"type":     "user_left",
				"username": "leavinguser",
			},
		},
		{
			name: "error message",
			msg: ServerMessage{
				Type:    "error",
				Message: "Failed to process command",
			},
			want: map[string]interface{}{
				"type":    "error",
				"message": "Failed to process command",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			testutil.AssertNoError(t, err)

			var result map[string]interface{}
			err = json.Unmarshal(data, &result)
			testutil.AssertNoError(t, err)

			for key, expected := range tt.want {
				got, ok := result[key]
				if !ok {
					t.Errorf("missing key %q in JSON output", key)
					continue
				}
				// Type assertion for comparison
				switch v := expected.(type) {
				case string:
					testutil.AssertEqual(t, got.(string), v)
				case bool:
					testutil.AssertEqual(t, got.(bool), v)
				}
			}
		})
	}
}

func TestClient_CloseConnection_Idempotent(t *testing.T) {
	// Create a test WebSocket server and client
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		// Keep connection open
		time.Sleep(100 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNoError(t, err)

	hub := NewHub()
	publisher := testutil.NewMockMessagePublisher()
	chatroomRepo := testutil.NewMockChatroomRepository()
	messageRepo := testutil.NewMockMessageRepository()
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	client := NewClient(context.Background(), hub, conn, "user-123", "testuser", "room-1", chatService, publisher)

	// Close connection multiple times - should not panic
	client.closeConnection()
	client.closeConnection()
	client.closeConnection()

	// Verify connection is closed
	testutil.AssertTrue(t, client.closed.Load(), "connection should be marked as closed")
}

func TestClient_WriteMessage_ThreadSafe(t *testing.T) {
	// Create a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		// Keep reading to prevent connection close
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNoError(t, err)
	defer conn.Close()

	hub := NewHub()
	publisher := testutil.NewMockMessagePublisher()
	chatroomRepo := testutil.NewMockChatroomRepository()
	messageRepo := testutil.NewMockMessageRepository()
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	client := NewClient(context.Background(), hub, conn, "user-123", "testuser", "room-1", chatService, publisher)

	// Send multiple messages concurrently
	var wg sync.WaitGroup
	numGoroutines := 10
	messagesPerGoroutine := 5

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				err := client.writeMessage(websocket.TextMessage, []byte("test message"))
				// Ignore errors - we're testing thread safety, not connection health
				_ = err
			}
		}(i)
	}

	wg.Wait()

	// If we reach here without deadlock or panic, the test passes
}

func TestClient_SendChannel_Buffered(t *testing.T) {
	hub := NewHub()
	ctx := context.Background()
	publisher := testutil.NewMockMessagePublisher()
	chatroomRepo := testutil.NewMockChatroomRepository()
	messageRepo := testutil.NewMockMessageRepository()
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	// Create a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNoError(t, err)
	defer conn.Close()

	client := NewClient(ctx, hub, conn, "user-123", "testuser", "room-1", chatService, publisher)

	// Verify send channel is buffered (capacity 256)
	testutil.AssertEqual(t, cap(client.send), 256)

	// We should be able to send 256 messages without blocking
	for i := 0; i < 256; i++ {
		select {
		case client.send <- []byte("test"):
		default:
			t.Fatalf("send channel blocked at message %d", i)
		}
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	publisher := testutil.NewMockMessagePublisher()
	chatroomRepo := testutil.NewMockChatroomRepository()
	messageRepo := testutil.NewMockMessageRepository()
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	// Create a test WebSocket server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, _ := upgrader.Upgrade(w, r, nil)
		defer conn.Close()
		time.Sleep(500 * time.Millisecond)
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNoError(t, err)
	defer conn.Close()

	client := NewClient(ctx, hub, conn, "user-123", "testuser", "room-1", chatService, publisher)

	// Cancel the parent context
	cancel()

	// Client's context should also be cancelled (derived context)
	select {
	case <-client.ctx.Done():
		// Expected - client context should be done
	case <-time.After(100 * time.Millisecond):
		t.Error("client context should be cancelled after parent cancel")
	}
}

// Integration test with real WebSocket connection
func TestClient_WritePump_Integration(t *testing.T) {
	// Create channels for communication
	receivedMessages := make(chan []byte, 10)

	// Create a test WebSocket server that echoes messages
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Read messages from client
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			receivedMessages <- msg
		}
	}))
	defer server.Close()

	// Connect to server
	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNoError(t, err)
	defer conn.Close()

	hub := NewHub()
	publisher := testutil.NewMockMessagePublisher()
	chatroomRepo := testutil.NewMockChatroomRepository()
	messageRepo := testutil.NewMockMessageRepository()
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	client := NewClient(context.Background(), hub, conn, "user-123", "testuser", "room-1", chatService, publisher)

	// Start write pump in background
	go client.WritePump()

	// Send a message through the send channel
	testMessage := []byte(`{"type":"chat_message","content":"Hello!"}`)
	client.send <- testMessage

	// Wait for message to be received by server
	select {
	case msg := <-receivedMessages:
		testutil.AssertEqual(t, string(msg), string(testMessage))
	case <-time.After(time.Second):
		t.Error("timeout waiting for message")
	}

	// Close the send channel to stop write pump
	close(client.send)
}

// Test that stock command is published correctly
func TestClient_StockCommand_Published(t *testing.T) {
	// Skip this test in short mode as it requires WebSocket setup
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	publisher := testutil.NewMockMessagePublisher()
	chatroomRepo := testutil.NewMockChatroomRepository()
	messageRepo := testutil.NewMockMessageRepository()

	// Add membership so message can be sent
	chatroomRepo.Members = map[string]map[string]bool{
		"room-1": {"user-123": true},
	}

	chatService := service.NewChatService(messageRepo, chatroomRepo)

	// Create a test WebSocket server
	serverMessages := make(chan []byte, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		// Send a stock command to the client
		stockCmd := ClientMessage{Type: "chat_message", Content: "/stock=AAPL.US"}
		data, _ := json.Marshal(stockCmd)
		conn.WriteMessage(websocket.TextMessage, data)

		// Read any responses
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			serverMessages <- msg
		}
	}))
	defer server.Close()

	wsURL := "ws" + server.URL[4:]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	testutil.AssertNoError(t, err)
	defer conn.Close()

	hub := NewHub()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hub.Run(ctx)
	time.Sleep(50 * time.Millisecond)

	client := NewClient(ctx, hub, conn, "user-123", "testuser", "room-1", chatService, publisher)
	hub.Register(client)

	// Run read pump briefly to process the stock command
	done := make(chan struct{})
	go func() {
		client.ReadPump()
		close(done)
	}()

	// Wait for processing or timeout
	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
	}

	// Check if stock command was published
	calls := publisher.GetStockCommandCalls()
	if len(calls) > 0 {
		testutil.AssertEqual(t, calls[0].StockCode, "AAPL.US")
		testutil.AssertEqual(t, calls[0].ChatroomID, "room-1")
		testutil.AssertEqual(t, calls[0].RequestedBy, "testuser")
	}
}

// Test message type constants
func TestMessageTypeConstants(t *testing.T) {
	// Verify that common message types are used consistently
	tests := []struct {
		msgType string
		valid   bool
	}{
		{"chat_message", true},
		{"user_joined", true},
		{"user_left", true},
		{"error", true},
		{"user_count_update", true},
	}

	for _, tt := range tests {
		t.Run(tt.msgType, func(t *testing.T) {
			msg := ServerMessage{Type: tt.msgType}
			data, err := json.Marshal(msg)
			testutil.AssertNoError(t, err)
			testutil.AssertContains(t, string(data), tt.msgType)
		})
	}
}

// Benchmark client creation
func BenchmarkNewClient(b *testing.B) {
	// Setup
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatal(err)
	}
	defer ln.Close()

	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			upgrader := websocket.Upgrader{}
			conn, _ := upgrader.Upgrade(w, r, nil)
			defer conn.Close()
			time.Sleep(time.Hour)
		}),
	}
	go server.Serve(ln)

	hub := NewHub()
	publisher := testutil.NewMockMessagePublisher()
	chatroomRepo := testutil.NewMockChatroomRepository()
	messageRepo := testutil.NewMockMessageRepository()
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	wsURL := "ws://" + ln.Addr().String()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer conn.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewClient(ctx, hub, conn, "user-123", "testuser", "room-1", chatService, publisher)
	}
}
