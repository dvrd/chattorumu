//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestClient wraps http.Client with cookie handling for a single user session
type TestClient struct {
	*http.Client
	t            *testing.T
	sessionToken string
	userID       string
	username     string
}

// NewTestClient creates a new test client with cookie jar
func NewTestClient(t *testing.T) *TestClient {
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("failed to create cookie jar: %v", err)
	}

	return &TestClient{
		Client: &http.Client{
			Timeout: 30 * time.Second,
			Jar:     jar,
		},
		t: t,
	}
}

// RegisterUser registers a new user and returns the response
func (tc *TestClient) RegisterUser(username, email, password string) (*RegisterResponse, error) {
	body := map[string]string{
		"username": username,
		"email":    email,
		"password": password,
	}

	resp, err := tc.PostJSON("/api/v1/auth/register", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("register failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode register response: %w", err)
	}

	tc.userID = result.ID
	tc.username = result.Username
	return &result, nil
}

// LoginUser logs in a user and stores the session token
func (tc *TestClient) LoginUser(username, password string) (*LoginResponse, error) {
	body := map[string]string{
		"username": username,
		"password": password,
	}

	resp, err := tc.PostJSON("/api/v1/auth/login", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("login failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode login response: %w", err)
	}

	tc.sessionToken = result.SessionToken
	tc.userID = result.User.ID
	tc.username = result.User.Username
	return &result, nil
}

// Logout logs out the current user
func (tc *TestClient) Logout() error {
	resp, err := tc.PostJSON("/api/v1/auth/logout", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("logout failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	tc.sessionToken = ""
	return nil
}

// GetMe returns the current user information
func (tc *TestClient) GetMe() (*RegisterResponse, error) {
	resp, err := tc.Get(baseURL + "/api/v1/auth/me")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get me failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode me response: %w", err)
	}

	return &result, nil
}

// CreateChatroom creates a new chatroom
func (tc *TestClient) CreateChatroom(name string) (*ChatroomResponse, error) {
	body := map[string]string{
		"name": name,
	}

	resp, err := tc.PostJSON("/api/v1/chatrooms", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create chatroom failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ChatroomResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode chatroom response: %w", err)
	}

	return &result, nil
}

// ListChatrooms lists all chatrooms
func (tc *TestClient) ListChatrooms() (*ListChatroomsResponse, error) {
	resp, err := tc.Get(baseURL + "/api/v1/chatrooms")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("list chatrooms failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ListChatroomsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode chatrooms response: %w", err)
	}

	return &result, nil
}

// JoinChatroom joins a chatroom
func (tc *TestClient) JoinChatroom(chatroomID string) error {
	resp, err := tc.PostJSON(fmt.Sprintf("/api/v1/chatrooms/%s/join", chatroomID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("join chatroom failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// GetMessages gets messages from a chatroom
func (tc *TestClient) GetMessages(chatroomID string, limit int) (*MessagesResponse, error) {
	url := fmt.Sprintf("%s/api/v1/chatrooms/%s/messages", baseURL, chatroomID)
	if limit > 0 {
		url = fmt.Sprintf("%s?limit=%d", url, limit)
	}

	resp, err := tc.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get messages failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result MessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode messages response: %w", err)
	}

	return &result, nil
}

// PostJSON makes a POST request with JSON body
func (tc *TestClient) PostJSON(path string, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequest(http.MethodPost, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	return tc.Do(req)
}

// Response types
type RegisterResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type LoginResponse struct {
	Success      bool             `json:"success"`
	User         RegisterResponse `json:"user"`
	SessionToken string           `json:"session_token"`
}

type ChatroomResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
	UserCount int    `json:"user_count"`
}

type ListChatroomsResponse struct {
	Chatrooms  []ChatroomResponse `json:"chatrooms"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

type MessageResponse struct {
	ID         string `json:"id"`
	ChatroomID string `json:"chatroom_id"`
	UserID     string `json:"user_id"`
	Username   string `json:"username"`
	Content    string `json:"content"`
	IsBot      bool   `json:"is_bot"`
	CreatedAt  string `json:"created_at"`
}

type MessagesResponse struct {
	Messages []MessageResponse `json:"messages"`
}

// WebSocket helpers

// WSClient represents a WebSocket client for testing
type WSClient struct {
	t          *testing.T
	conn       *websocket.Conn
	mu         sync.Mutex
	messages   chan WSMessage
	done       chan struct{}
	chatroomID string
}

// WSMessage represents a WebSocket message
type WSMessage struct {
	Type       string         `json:"type"`
	Content    string         `json:"content,omitempty"`
	Username   string         `json:"username,omitempty"`
	UserID     string         `json:"user_id,omitempty"`
	ChatroomID string         `json:"chatroom_id,omitempty"`
	MessageID  string         `json:"message_id,omitempty"`
	ID         string         `json:"id,omitempty"`
	IsBot      bool           `json:"is_bot,omitempty"`
	IsError    bool           `json:"is_error,omitempty"`
	Timestamp  string         `json:"timestamp,omitempty"`
	CreatedAt  *time.Time     `json:"created_at,omitempty"`
	UserCounts map[string]int `json:"user_counts,omitempty"`
	Error      string         `json:"error,omitempty"`
}

// ConnectWebSocket connects to a chatroom via WebSocket
func (tc *TestClient) ConnectWebSocket(chatroomID string) (*WSClient, error) {
	url := fmt.Sprintf("%s/ws/chat/%s?token=%s", wsURL, chatroomID, tc.sessionToken)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	wsc := &WSClient{
		t:          tc.t,
		conn:       conn,
		messages:   make(chan WSMessage, 100),
		done:       make(chan struct{}),
		chatroomID: chatroomID,
	}

	go wsc.readLoop()

	return wsc, nil
}

// readLoop reads messages from the WebSocket connection
func (wsc *WSClient) readLoop() {
	defer close(wsc.messages)

	for {
		select {
		case <-wsc.done:
			return
		default:
			_, data, err := wsc.conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					return
				}
				// Connection closed
				return
			}

			var msg WSMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				wsc.t.Logf("failed to unmarshal WebSocket message: %v", err)
				continue
			}

			select {
			case wsc.messages <- msg:
			default:
				wsc.t.Log("message channel full, dropping message")
			}
		}
	}
}

// SendMessage sends a chat message
func (wsc *WSClient) SendMessage(content string) error {
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	msg := map[string]string{
		"type":    "message",
		"content": content,
	}

	return wsc.conn.WriteJSON(msg)
}

// WaitForMessage waits for a message matching the predicate
func (wsc *WSClient) WaitForMessage(timeout time.Duration, predicate func(WSMessage) bool) (*WSMessage, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case msg, ok := <-wsc.messages:
			if !ok {
				return nil, fmt.Errorf("connection closed while waiting for message")
			}
			if predicate(msg) {
				return &msg, nil
			}
		case <-timer.C:
			return nil, fmt.Errorf("timeout waiting for message")
		}
	}
}

// WaitForMessageType waits for a message of a specific type
func (wsc *WSClient) WaitForMessageType(msgType string, timeout time.Duration) (*WSMessage, error) {
	return wsc.WaitForMessage(timeout, func(msg WSMessage) bool {
		return msg.Type == msgType
	})
}

// WaitForChatMessage waits for a chat message with specific content
func (wsc *WSClient) WaitForChatMessage(content string, timeout time.Duration) (*WSMessage, error) {
	return wsc.WaitForMessage(timeout, func(msg WSMessage) bool {
		return (msg.Type == "message" || msg.Type == "chat_message") && msg.Content == content
	})
}

// DrainMessages clears all pending messages
func (wsc *WSClient) DrainMessages() {
	for {
		select {
		case <-wsc.messages:
		default:
			return
		}
	}
}

// Close closes the WebSocket connection
func (wsc *WSClient) Close() error {
	close(wsc.done)
	wsc.mu.Lock()
	defer wsc.mu.Unlock()

	return wsc.conn.Close()
}

// Test helpers

// uniqueUsername generates a unique username for testing
func uniqueUsername(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

// uniqueEmail generates a unique email for testing
func uniqueEmail(prefix string) string {
	return fmt.Sprintf("%s_%d@test.com", prefix, time.Now().UnixNano())
}

// setupTestUser creates and logs in a test user, returning the client
func setupTestUser(t *testing.T, prefix string) *TestClient {
	t.Helper()

	client := NewTestClient(t)
	username := uniqueUsername(prefix)
	email := uniqueEmail(prefix)

	_, err := client.RegisterUser(username, email, "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	_, err = client.LoginUser(username, "password123")
	if err != nil {
		t.Fatalf("failed to login user: %v", err)
	}

	return client
}

// setupChatroomWithUser creates a user, logs them in, and creates a chatroom
func setupChatroomWithUser(t *testing.T, prefix string) (*TestClient, *ChatroomResponse) {
	t.Helper()

	client := setupTestUser(t, prefix)

	chatroom, err := client.CreateChatroom(fmt.Sprintf("%s_room", prefix))
	if err != nil {
		t.Fatalf("failed to create chatroom: %v", err)
	}

	return client, chatroom
}

// assertNoError fails the test if err is not nil
func assertNoError(t *testing.T, err error, msg string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", msg, err)
	}
}

// assertEqual checks if two values are equal
func assertEqual[T comparable](t *testing.T, got, want T, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

// createTestUser creates a test user and returns user ID
func createTestUser(t *testing.T) *RegisterResponse {
	t.Helper()

	client := NewTestClient(t)
	username := uniqueUsername("msgtest")
	email := uniqueEmail("msgtest")

	user, err := client.RegisterUser(username, email, "password123")
	if err != nil {
		t.Fatalf("failed to register user: %v", err)
	}

	_, err = client.LoginUser(username, "password123")
	if err != nil {
		t.Fatalf("failed to login user: %v", err)
	}

	// Update testClient with session token for later use
	return user
}

// createTestChatroom creates a chatroom for testing
func createTestChatroom(t *testing.T, userID string) *ChatroomResponse {
	t.Helper()

	client := setupTestUser(t, "chatroomtest")

	chatroom, err := client.CreateChatroom(fmt.Sprintf("msgtest_room_%d", time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("failed to create chatroom: %v", err)
	}

	return chatroom
}

// newTestWSClient creates a WebSocket client for a user in a chatroom
func newTestWSClient(t *testing.T, user *RegisterResponse, chatroomID string) (*WSClient, error) {
	t.Helper()

	client := setupTestUser(t, "wstest")

	// Join the chatroom
	err := client.JoinChatroom(chatroomID)
	if err != nil {
		return nil, fmt.Errorf("failed to join chatroom: %w", err)
	}

	// Connect WebSocket
	ws, err := client.ConnectWebSocket(chatroomID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect WebSocket: %w", err)
	}

	return ws, nil
}

// TestWSClient is an alias for WSClient for compatibility
type TestWSClient = WSClient
