package testutil

import (
	"fmt"
	"sync/atomic"
	"time"

	"jobsity-chat/internal/domain"
)

// Counter for generating unique IDs
var idCounter atomic.Int64

// nextID generates a unique ID for test fixtures
func nextID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, idCounter.Add(1))
}

// UserOptions allows customizing user fixture creation
type UserOptions struct {
	ID           string
	Username     string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// NewTestUser creates a test user with sensible defaults
// Pass options to override specific fields
func NewTestUser(opts ...func(*UserOptions)) *domain.User {
	o := &UserOptions{
		ID:           nextID("user"),
		Username:     fmt.Sprintf("testuser%d", idCounter.Load()),
		PasswordHash: "$2a$10$test.hash.for.testing.purposes.only", // bcrypt hash placeholder
	}

	for _, opt := range opts {
		opt(o)
	}

	// Set email based on username if not provided
	if o.Email == "" {
		o.Email = o.Username + "@example.com"
	}

	// Set created time if not provided
	if o.CreatedAt.IsZero() {
		o.CreatedAt = time.Now()
	}

	return &domain.User{
		ID:           o.ID,
		Username:     o.Username,
		Email:        o.Email,
		PasswordHash: o.PasswordHash,
		CreatedAt:    o.CreatedAt,
	}
}

// User option functions

// WithUserID sets the user ID
func WithUserID(id string) func(*UserOptions) {
	return func(o *UserOptions) {
		o.ID = id
	}
}

// WithUsername sets the username
func WithUsername(username string) func(*UserOptions) {
	return func(o *UserOptions) {
		o.Username = username
	}
}

// WithEmail sets the email
func WithEmail(email string) func(*UserOptions) {
	return func(o *UserOptions) {
		o.Email = email
	}
}

// WithPasswordHash sets the password hash
func WithPasswordHash(hash string) func(*UserOptions) {
	return func(o *UserOptions) {
		o.PasswordHash = hash
	}
}

// WithUserCreatedAt sets the user creation time
func WithUserCreatedAt(t time.Time) func(*UserOptions) {
	return func(o *UserOptions) {
		o.CreatedAt = t
	}
}

// SessionOptions allows customizing session fixture creation
type SessionOptions struct {
	ID        string
	UserID    string
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
}

// NewTestSession creates a test session with sensible defaults
func NewTestSession(opts ...func(*SessionOptions)) *domain.Session {
	o := &SessionOptions{
		ID:        nextID("session"),
		UserID:    nextID("user"),
		Token:     nextID("token"),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(o)
	}

	return &domain.Session{
		ID:        o.ID,
		UserID:    o.UserID,
		Token:     o.Token,
		ExpiresAt: o.ExpiresAt,
		CreatedAt: o.CreatedAt,
	}
}

// Session option functions

// WithSessionID sets the session ID
func WithSessionID(id string) func(*SessionOptions) {
	return func(o *SessionOptions) {
		o.ID = id
	}
}

// WithSessionUserID sets the user ID for the session
func WithSessionUserID(userID string) func(*SessionOptions) {
	return func(o *SessionOptions) {
		o.UserID = userID
	}
}

// WithToken sets the session token
func WithToken(token string) func(*SessionOptions) {
	return func(o *SessionOptions) {
		o.Token = token
	}
}

// WithExpiresAt sets the session expiration time
func WithExpiresAt(t time.Time) func(*SessionOptions) {
	return func(o *SessionOptions) {
		o.ExpiresAt = t
	}
}

// WithExpired creates an expired session
func WithExpired() func(*SessionOptions) {
	return func(o *SessionOptions) {
		o.ExpiresAt = time.Now().Add(-1 * time.Hour)
	}
}

// WithSessionCreatedAt sets the session creation time
func WithSessionCreatedAt(t time.Time) func(*SessionOptions) {
	return func(o *SessionOptions) {
		o.CreatedAt = t
	}
}

// ChatroomOptions allows customizing chatroom fixture creation
type ChatroomOptions struct {
	ID        string
	Name      string
	CreatedAt time.Time
	CreatedBy string
}

// NewTestChatroom creates a test chatroom with sensible defaults
func NewTestChatroom(opts ...func(*ChatroomOptions)) *domain.Chatroom {
	o := &ChatroomOptions{
		ID:        nextID("chatroom"),
		Name:      fmt.Sprintf("Test Room %d", idCounter.Load()),
		CreatedAt: time.Now(),
		CreatedBy: nextID("user"),
	}

	for _, opt := range opts {
		opt(o)
	}

	return &domain.Chatroom{
		ID:        o.ID,
		Name:      o.Name,
		CreatedAt: o.CreatedAt,
		CreatedBy: o.CreatedBy,
	}
}

// Chatroom option functions

// WithChatroomID sets the chatroom ID
func WithChatroomID(id string) func(*ChatroomOptions) {
	return func(o *ChatroomOptions) {
		o.ID = id
	}
}

// WithChatroomName sets the chatroom name
func WithChatroomName(name string) func(*ChatroomOptions) {
	return func(o *ChatroomOptions) {
		o.Name = name
	}
}

// WithCreatedBy sets who created the chatroom
func WithCreatedBy(userID string) func(*ChatroomOptions) {
	return func(o *ChatroomOptions) {
		o.CreatedBy = userID
	}
}

// WithChatroomCreatedAt sets the chatroom creation time
func WithChatroomCreatedAt(t time.Time) func(*ChatroomOptions) {
	return func(o *ChatroomOptions) {
		o.CreatedAt = t
	}
}

// MessageOptions allows customizing message fixture creation
type MessageOptions struct {
	ID         string
	ChatroomID string
	UserID     string
	Username   string
	Content    string
	IsBot      bool
	CreatedAt  time.Time
}

// NewTestMessage creates a test message with sensible defaults
func NewTestMessage(opts ...func(*MessageOptions)) *domain.Message {
	o := &MessageOptions{
		ID:         nextID("msg"),
		ChatroomID: nextID("chatroom"),
		UserID:     nextID("user"),
		Username:   fmt.Sprintf("testuser%d", idCounter.Load()),
		Content:    "Hello, World!",
		IsBot:      false,
		CreatedAt:  time.Now(),
	}

	for _, opt := range opts {
		opt(o)
	}

	return &domain.Message{
		ID:         o.ID,
		ChatroomID: o.ChatroomID,
		UserID:     o.UserID,
		Username:   o.Username,
		Content:    o.Content,
		IsBot:      o.IsBot,
		CreatedAt:  o.CreatedAt,
	}
}

// Message option functions

// WithMessageID sets the message ID
func WithMessageID(id string) func(*MessageOptions) {
	return func(o *MessageOptions) {
		o.ID = id
	}
}

// WithMessageChatroomID sets the chatroom ID for the message
func WithMessageChatroomID(chatroomID string) func(*MessageOptions) {
	return func(o *MessageOptions) {
		o.ChatroomID = chatroomID
	}
}

// WithMessageUserID sets the user ID for the message
func WithMessageUserID(userID string) func(*MessageOptions) {
	return func(o *MessageOptions) {
		o.UserID = userID
	}
}

// WithMessageUsername sets the username for the message
func WithMessageUsername(username string) func(*MessageOptions) {
	return func(o *MessageOptions) {
		o.Username = username
	}
}

// WithContent sets the message content
func WithContent(content string) func(*MessageOptions) {
	return func(o *MessageOptions) {
		o.Content = content
	}
}

// WithIsBot sets whether the message is from a bot
func WithIsBot(isBot bool) func(*MessageOptions) {
	return func(o *MessageOptions) {
		o.IsBot = isBot
	}
}

// WithMessageCreatedAt sets the message creation time
func WithMessageCreatedAt(t time.Time) func(*MessageOptions) {
	return func(o *MessageOptions) {
		o.CreatedAt = t
	}
}

// Batch creation helpers

// NewTestUsers creates multiple test users
func NewTestUsers(count int) []*domain.User {
	users := make([]*domain.User, count)
	for i := 0; i < count; i++ {
		users[i] = NewTestUser()
	}
	return users
}

// NewTestMessages creates multiple test messages in the same chatroom
func NewTestMessages(chatroomID string, count int) []*domain.Message {
	messages := make([]*domain.Message, count)
	for i := 0; i < count; i++ {
		messages[i] = NewTestMessage(
			WithMessageChatroomID(chatroomID),
			WithMessageCreatedAt(time.Now().Add(time.Duration(i)*time.Second)),
		)
	}
	return messages
}

// ResetIDCounter resets the ID counter (useful for deterministic tests)
func ResetIDCounter() {
	idCounter.Store(0)
}
