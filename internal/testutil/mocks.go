// Package testutil provides shared test utilities, mocks, and fixtures
// for testing the jobsity-chat application.
package testutil

import (
	"context"
	"errors"
	"sync"
	"time"

	"jobsity-chat/internal/domain"
)

// Common test errors
var (
	ErrMockNotImplemented = errors.New("mock function not implemented")
	ErrMockNotFound       = errors.New("mock: not found")
)

// MockUserRepository implements domain.UserRepository for testing
type MockUserRepository struct {
	mu sync.RWMutex

	// Function overrides - set these to customize behavior
	CreateFunc        func(ctx context.Context, user *domain.User) error
	GetByIDFunc       func(ctx context.Context, id string) (*domain.User, error)
	GetByUsernameFunc func(ctx context.Context, username string) (*domain.User, error)
	GetByEmailFunc    func(ctx context.Context, email string) (*domain.User, error)

	// In-memory storage for simple tests
	Users map[string]*domain.User
}

// NewMockUserRepository creates a new MockUserRepository with initialized maps
func NewMockUserRepository() *MockUserRepository {
	return &MockUserRepository{
		Users: make(map[string]*domain.User),
	}
}

func (m *MockUserRepository) Create(ctx context.Context, user *domain.User) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, user)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Users == nil {
		m.Users = make(map[string]*domain.User)
	}

	// Check for duplicates
	for _, u := range m.Users {
		if u.Username == user.Username {
			return domain.ErrUsernameExists
		}
		if u.Email == user.Email {
			return domain.ErrEmailExists
		}
	}

	if user.ID == "" {
		user.ID = "user-" + user.Username
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	m.Users[user.ID] = user
	return nil
}

func (m *MockUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if user, ok := m.Users[id]; ok {
		return user, nil
	}
	return nil, domain.ErrUserNotFound
}

func (m *MockUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	if m.GetByUsernameFunc != nil {
		return m.GetByUsernameFunc(ctx, username)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, user := range m.Users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

func (m *MockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.GetByEmailFunc != nil {
		return m.GetByEmailFunc(ctx, email)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, user := range m.Users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, domain.ErrUserNotFound
}

// MockSessionRepository implements domain.SessionRepository for testing
type MockSessionRepository struct {
	mu sync.RWMutex

	// Function overrides
	CreateFunc        func(ctx context.Context, session *domain.Session) error
	GetByTokenFunc    func(ctx context.Context, token string) (*domain.Session, error)
	DeleteFunc        func(ctx context.Context, token string) error
	DeleteExpiredFunc func(ctx context.Context) (int64, error)

	// In-memory storage
	Sessions map[string]*domain.Session
}

// NewMockSessionRepository creates a new MockSessionRepository with initialized maps
func NewMockSessionRepository() *MockSessionRepository {
	return &MockSessionRepository{
		Sessions: make(map[string]*domain.Session),
	}
}

func (m *MockSessionRepository) Create(ctx context.Context, session *domain.Session) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, session)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Sessions == nil {
		m.Sessions = make(map[string]*domain.Session)
	}
	m.Sessions[session.Token] = session
	return nil
}

func (m *MockSessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	if m.GetByTokenFunc != nil {
		return m.GetByTokenFunc(ctx, token)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if session, ok := m.Sessions[token]; ok {
		if session.ExpiresAt.Before(time.Now()) {
			return nil, domain.ErrSessionExpired
		}
		return session, nil
	}
	return nil, domain.ErrSessionNotFound
}

func (m *MockSessionRepository) Delete(ctx context.Context, token string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, token)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.Sessions, token)
	return nil
}

func (m *MockSessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	if m.DeleteExpiredFunc != nil {
		return m.DeleteExpiredFunc(ctx)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var count int64
	now := time.Now()
	for token, session := range m.Sessions {
		if session.ExpiresAt.Before(now) {
			delete(m.Sessions, token)
			count++
		}
	}
	return count, nil
}

// MockChatroomRepository implements domain.ChatroomRepository for testing
type MockChatroomRepository struct {
	mu sync.RWMutex

	// Function overrides
	CreateFunc           func(ctx context.Context, chatroom *domain.Chatroom) error
	CreateWithMemberFunc func(ctx context.Context, chatroom *domain.Chatroom, userID string) error
	GetByIDFunc          func(ctx context.Context, id string) (*domain.Chatroom, error)
	ListFunc             func(ctx context.Context) ([]*domain.Chatroom, error)
	ListPaginatedFunc    func(ctx context.Context, limit int, cursor string) ([]*domain.Chatroom, string, error)
	AddMemberFunc        func(ctx context.Context, chatroomID, userID string) error
	IsMemberFunc         func(ctx context.Context, chatroomID, userID string) (bool, error)

	// In-memory storage
	Chatrooms map[string]*domain.Chatroom
	Members   map[string]map[string]bool // chatroomID -> userID -> isMember
}

// NewMockChatroomRepository creates a new MockChatroomRepository with initialized maps
func NewMockChatroomRepository() *MockChatroomRepository {
	return &MockChatroomRepository{
		Chatrooms: make(map[string]*domain.Chatroom),
		Members:   make(map[string]map[string]bool),
	}
}

func (m *MockChatroomRepository) Create(ctx context.Context, chatroom *domain.Chatroom) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, chatroom)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if chatroom.ID == "" {
		chatroom.ID = "chatroom-" + chatroom.Name
	}
	if chatroom.CreatedAt.IsZero() {
		chatroom.CreatedAt = time.Now()
	}
	m.Chatrooms[chatroom.ID] = chatroom
	return nil
}

func (m *MockChatroomRepository) CreateWithMember(ctx context.Context, chatroom *domain.Chatroom, userID string) error {
	if m.CreateWithMemberFunc != nil {
		return m.CreateWithMemberFunc(ctx, chatroom, userID)
	}
	if err := m.Create(ctx, chatroom); err != nil {
		return err
	}
	return m.AddMember(ctx, chatroom.ID, userID)
}

func (m *MockChatroomRepository) GetByID(ctx context.Context, id string) (*domain.Chatroom, error) {
	if m.GetByIDFunc != nil {
		return m.GetByIDFunc(ctx, id)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if chatroom, ok := m.Chatrooms[id]; ok {
		return chatroom, nil
	}
	return nil, domain.ErrChatroomNotFound
}

func (m *MockChatroomRepository) List(ctx context.Context) ([]*domain.Chatroom, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*domain.Chatroom, 0, len(m.Chatrooms))
	for _, chatroom := range m.Chatrooms {
		result = append(result, chatroom)
	}
	return result, nil
}

func (m *MockChatroomRepository) ListPaginated(ctx context.Context, limit int, cursor string) ([]*domain.Chatroom, string, error) {
	if m.ListPaginatedFunc != nil {
		return m.ListPaginatedFunc(ctx, limit, cursor)
	}
	chatrooms, err := m.List(ctx)
	if err != nil {
		return nil, "", err
	}

	// Simple pagination: return up to limit items
	if len(chatrooms) > limit {
		return chatrooms[:limit], chatrooms[limit-1].ID, nil
	}
	return chatrooms, "", nil
}

func (m *MockChatroomRepository) AddMember(ctx context.Context, chatroomID, userID string) error {
	if m.AddMemberFunc != nil {
		return m.AddMemberFunc(ctx, chatroomID, userID)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Members == nil {
		m.Members = make(map[string]map[string]bool)
	}
	if m.Members[chatroomID] == nil {
		m.Members[chatroomID] = make(map[string]bool)
	}
	m.Members[chatroomID][userID] = true
	return nil
}

func (m *MockChatroomRepository) IsMember(ctx context.Context, chatroomID, userID string) (bool, error) {
	if m.IsMemberFunc != nil {
		return m.IsMemberFunc(ctx, chatroomID, userID)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if members, ok := m.Members[chatroomID]; ok {
		return members[userID], nil
	}
	return false, nil
}

// MockMessageRepository implements domain.MessageRepository for testing
type MockMessageRepository struct {
	mu sync.RWMutex

	// Function overrides
	CreateFunc              func(ctx context.Context, message *domain.Message) error
	GetByChatroomFunc       func(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error)
	GetByChatroomBeforeFunc func(ctx context.Context, chatroomID string, before string, limit int) ([]*domain.Message, error)

	// In-memory storage
	Messages []*domain.Message
}

// NewMockMessageRepository creates a new MockMessageRepository with initialized slices
func NewMockMessageRepository() *MockMessageRepository {
	return &MockMessageRepository{
		Messages: make([]*domain.Message, 0),
	}
}

func (m *MockMessageRepository) Create(ctx context.Context, message *domain.Message) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, message)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if message.ID == "" {
		message.ID = "msg-" + time.Now().Format("20060102150405.000")
	}
	if message.CreatedAt.IsZero() {
		message.CreatedAt = time.Now()
	}
	m.Messages = append(m.Messages, message)
	return nil
}

func (m *MockMessageRepository) GetByChatroom(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
	if m.GetByChatroomFunc != nil {
		return m.GetByChatroomFunc(ctx, chatroomID, limit)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*domain.Message, 0)
	for _, msg := range m.Messages {
		if msg.ChatroomID == chatroomID {
			result = append(result, msg)
		}
	}

	// Return last 'limit' messages (most recent)
	if len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result, nil
}

func (m *MockMessageRepository) GetByChatroomBefore(ctx context.Context, chatroomID string, before string, limit int) ([]*domain.Message, error) {
	if m.GetByChatroomBeforeFunc != nil {
		return m.GetByChatroomBeforeFunc(ctx, chatroomID, before, limit)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*domain.Message, 0)
	foundBefore := false

	for i := len(m.Messages) - 1; i >= 0; i-- {
		msg := m.Messages[i]
		if msg.ChatroomID != chatroomID {
			continue
		}
		if msg.ID == before {
			foundBefore = true
			continue
		}
		if foundBefore || before == "" {
			result = append([]*domain.Message{msg}, result...)
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

// MockMessagePublisher implements websocket.MessagePublisher for testing
type MockMessagePublisher struct {
	mu sync.RWMutex

	// Function overrides
	PublishStockCommandFunc func(ctx context.Context, chatroomID, stockCode, requestedBy string) error
	PublishHelloCommandFunc func(ctx context.Context, chatroomID, requestedBy string) error

	// Call tracking
	StockCommands []StockCommandCall
	HelloCommands []HelloCommandCall
}

// StockCommandCall records a call to PublishStockCommand
type StockCommandCall struct {
	ChatroomID  string
	StockCode   string
	RequestedBy string
}

// HelloCommandCall records a call to PublishHelloCommand
type HelloCommandCall struct {
	ChatroomID  string
	RequestedBy string
}

// NewMockMessagePublisher creates a new MockMessagePublisher
func NewMockMessagePublisher() *MockMessagePublisher {
	return &MockMessagePublisher{
		StockCommands: make([]StockCommandCall, 0),
		HelloCommands: make([]HelloCommandCall, 0),
	}
}

func (m *MockMessagePublisher) PublishStockCommand(ctx context.Context, chatroomID, stockCode, requestedBy string) error {
	if m.PublishStockCommandFunc != nil {
		return m.PublishStockCommandFunc(ctx, chatroomID, stockCode, requestedBy)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.StockCommands = append(m.StockCommands, StockCommandCall{
		ChatroomID:  chatroomID,
		StockCode:   stockCode,
		RequestedBy: requestedBy,
	})
	return nil
}

func (m *MockMessagePublisher) PublishHelloCommand(ctx context.Context, chatroomID, requestedBy string) error {
	if m.PublishHelloCommandFunc != nil {
		return m.PublishHelloCommandFunc(ctx, chatroomID, requestedBy)
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.HelloCommands = append(m.HelloCommands, HelloCommandCall{
		ChatroomID:  chatroomID,
		RequestedBy: requestedBy,
	})
	return nil
}

// GetStockCommandCalls returns all recorded stock command calls
func (m *MockMessagePublisher) GetStockCommandCalls() []StockCommandCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]StockCommandCall{}, m.StockCommands...)
}

// GetHelloCommandCalls returns all recorded hello command calls
func (m *MockMessagePublisher) GetHelloCommandCalls() []HelloCommandCall {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]HelloCommandCall{}, m.HelloCommands...)
}

// Reset clears all recorded calls
func (m *MockMessagePublisher) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StockCommands = make([]StockCommandCall, 0)
	m.HelloCommands = make([]HelloCommandCall, 0)
}
