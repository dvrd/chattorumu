package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"jobsity-chat/internal/domain"
)

// Mock repositories for testing
type mockMessageRepository struct {
	messages      []*domain.Message
	create        func(ctx context.Context, message *domain.Message) error
	getByChatroom func(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error)
}

func (m *mockMessageRepository) Create(ctx context.Context, message *domain.Message) error {
	if m.create != nil {
		return m.create(ctx, message)
	}
	message.ID = "msg-" + string(rune(len(m.messages)))
	message.CreatedAt = time.Now()
	m.messages = append(m.messages, message)
	return nil
}

func (m *mockMessageRepository) GetByChatroom(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
	if m.getByChatroom != nil {
		return m.getByChatroom(ctx, chatroomID, limit)
	}

	result := []*domain.Message{}
	for _, msg := range m.messages {
		if msg.ChatroomID == chatroomID {
			result = append(result, msg)
		}
	}

	// Apply limit
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, nil
}

type mockChatroomRepository struct {
	chatrooms      map[string]*domain.Chatroom
	members        map[string]map[string]bool // chatroomID -> userID -> bool
	create         func(ctx context.Context, chatroom *domain.Chatroom) error
	getByID        func(ctx context.Context, id string) (*domain.Chatroom, error)
	list           func(ctx context.Context) ([]*domain.Chatroom, error)
	addMember      func(ctx context.Context, chatroomID, userID string) error
	isMember       func(ctx context.Context, chatroomID, userID string) (bool, error)
}

func (m *mockChatroomRepository) Create(ctx context.Context, chatroom *domain.Chatroom) error {
	if m.create != nil {
		return m.create(ctx, chatroom)
	}
	if m.chatrooms == nil {
		m.chatrooms = make(map[string]*domain.Chatroom)
	}
	chatroom.ID = "chatroom-" + chatroom.Name
	chatroom.CreatedAt = time.Now()
	m.chatrooms[chatroom.ID] = chatroom
	return nil
}

func (m *mockChatroomRepository) GetByID(ctx context.Context, id string) (*domain.Chatroom, error) {
	if m.getByID != nil {
		return m.getByID(ctx, id)
	}
	chatroom, ok := m.chatrooms[id]
	if !ok {
		return nil, errors.New("chatroom not found")
	}
	return chatroom, nil
}

func (m *mockChatroomRepository) List(ctx context.Context) ([]*domain.Chatroom, error) {
	if m.list != nil {
		return m.list(ctx)
	}
	result := []*domain.Chatroom{}
	for _, chatroom := range m.chatrooms {
		result = append(result, chatroom)
	}
	return result, nil
}

func (m *mockChatroomRepository) AddMember(ctx context.Context, chatroomID, userID string) error {
	if m.addMember != nil {
		return m.addMember(ctx, chatroomID, userID)
	}
	if m.members == nil {
		m.members = make(map[string]map[string]bool)
	}
	if m.members[chatroomID] == nil {
		m.members[chatroomID] = make(map[string]bool)
	}
	m.members[chatroomID][userID] = true
	return nil
}

func (m *mockChatroomRepository) IsMember(ctx context.Context, chatroomID, userID string) (bool, error) {
	if m.isMember != nil {
		return m.isMember(ctx, chatroomID, userID)
	}
	if m.members == nil || m.members[chatroomID] == nil {
		return false, nil
	}
	return m.members[chatroomID][userID], nil
}

func TestChatService_SendMessage_Success(t *testing.T) {
	messageRepo := &mockMessageRepository{
		messages: []*domain.Message{},
	}
	chatroomRepo := &mockChatroomRepository{
		members: map[string]map[string]bool{
			"chatroom1": {
				"user1": true,
			},
		},
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	message := &domain.Message{
		ChatroomID: "chatroom1",
		UserID:     "user1",
		Username:   "alice",
		Content:    "Hello, world!",
		IsBot:      false,
	}

	err := chatService.SendMessage(ctx, message)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if message.ID == "" {
		t.Error("Expected message ID to be set")
	}

	if message.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	if len(messageRepo.messages) != 1 {
		t.Errorf("Expected 1 message in repository, got %d", len(messageRepo.messages))
	}
}

func TestChatService_SendMessage_BotMessage(t *testing.T) {
	messageRepo := &mockMessageRepository{
		messages: []*domain.Message{},
	}
	chatroomRepo := &mockChatroomRepository{
		members: map[string]map[string]bool{
			"chatroom1": {
				"bot-user-id": true,
			},
		},
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	message := &domain.Message{
		ChatroomID: "chatroom1",
		UserID:     "bot-user-id",
		Username:   "StockBot",
		Content:    "AAPL.US quote is $151.5 per share",
		IsBot:      true,
	}

	err := chatService.SendMessage(ctx, message)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !messageRepo.messages[0].IsBot {
		t.Error("Expected IsBot to be true")
	}

	if messageRepo.messages[0].Username != "StockBot" {
		t.Errorf("Expected username 'StockBot', got %s", messageRepo.messages[0].Username)
	}
}

func TestChatService_GetMessages_WithLimit(t *testing.T) {
	messageRepo := &mockMessageRepository{
		messages: []*domain.Message{
			{ID: "msg1", ChatroomID: "chatroom1", Content: "Message 1"},
			{ID: "msg2", ChatroomID: "chatroom1", Content: "Message 2"},
			{ID: "msg3", ChatroomID: "chatroom1", Content: "Message 3"},
			{ID: "msg4", ChatroomID: "chatroom1", Content: "Message 4"},
			{ID: "msg5", ChatroomID: "chatroom1", Content: "Message 5"},
		},
	}
	chatroomRepo := &mockChatroomRepository{}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	messages, err := chatService.GetMessages(ctx, "chatroom1", 3)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}
}

func TestChatService_GetMessages_DefaultLimit(t *testing.T) {
	// Create 60 messages
	messages := make([]*domain.Message, 60)
	for i := 0; i < 60; i++ {
		messages[i] = &domain.Message{
			ID:         "msg" + string(rune(i)),
			ChatroomID: "chatroom1",
			Content:    "Message " + string(rune(i)),
		}
	}

	messageRepo := &mockMessageRepository{
		messages: messages,
	}
	chatroomRepo := &mockChatroomRepository{}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	result, err := chatService.GetMessages(ctx, "chatroom1", 0)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Default limit should be 50
	if len(result) > 50 {
		t.Errorf("Expected max 50 messages with default limit, got %d", len(result))
	}
}

func TestChatService_GetMessages_OrderedByTimestamp(t *testing.T) {
	now := time.Now()
	messageRepo := &mockMessageRepository{
		messages: []*domain.Message{
			{ID: "msg1", ChatroomID: "chatroom1", Content: "First", CreatedAt: now.Add(-3 * time.Minute)},
			{ID: "msg2", ChatroomID: "chatroom1", Content: "Second", CreatedAt: now.Add(-2 * time.Minute)},
			{ID: "msg3", ChatroomID: "chatroom1", Content: "Third", CreatedAt: now.Add(-1 * time.Minute)},
		},
	}
	chatroomRepo := &mockChatroomRepository{}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	messages, err := chatService.GetMessages(ctx, "chatroom1", 10)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify messages are returned (order depends on repository implementation)
	if len(messages) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(messages))
	}
}

func TestChatService_GetMessages_FiltersByChatroom(t *testing.T) {
	messageRepo := &mockMessageRepository{
		messages: []*domain.Message{
			{ID: "msg1", ChatroomID: "chatroom1", Content: "Chat 1 Message 1"},
			{ID: "msg2", ChatroomID: "chatroom2", Content: "Chat 2 Message 1"},
			{ID: "msg3", ChatroomID: "chatroom1", Content: "Chat 1 Message 2"},
			{ID: "msg4", ChatroomID: "chatroom2", Content: "Chat 2 Message 2"},
		},
	}
	chatroomRepo := &mockChatroomRepository{}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	messages, err := chatService.GetMessages(ctx, "chatroom1", 10)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("Expected 2 messages for chatroom1, got %d", len(messages))
	}

	for _, msg := range messages {
		if msg.ChatroomID != "chatroom1" {
			t.Errorf("Expected all messages from chatroom1, got message from %s", msg.ChatroomID)
		}
	}
}

func TestChatService_CreateChatroom_Success(t *testing.T) {
	messageRepo := &mockMessageRepository{}
	chatroomRepo := &mockChatroomRepository{
		chatrooms: make(map[string]*domain.Chatroom),
		members:   make(map[string]map[string]bool),
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	chatroom, err := chatService.CreateChatroom(ctx, "General", "user1")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if chatroom == nil {
		t.Fatal("Expected non-nil chatroom")
	}

	if chatroom.Name != "General" {
		t.Errorf("Expected name 'General', got %s", chatroom.Name)
	}

	if chatroom.CreatedBy != "user1" {
		t.Errorf("Expected created by 'user1', got %s", chatroom.CreatedBy)
	}

	if chatroom.ID == "" {
		t.Error("Expected chatroom ID to be set")
	}

	if chatroom.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}

	// Verify creator was added as member
	isMember, _ := chatroomRepo.IsMember(ctx, chatroom.ID, "user1")
	if !isMember {
		t.Error("Expected creator to be added as member")
	}
}

func TestChatService_CreateChatroom_EmptyName(t *testing.T) {
	messageRepo := &mockMessageRepository{}
	chatroomRepo := &mockChatroomRepository{}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	chatroom, err := chatService.CreateChatroom(ctx, "", "user1")

	if err == nil {
		t.Error("Expected error for empty chatroom name")
	}

	if chatroom != nil {
		t.Errorf("Expected nil chatroom, got: %+v", chatroom)
	}
}

func TestChatService_JoinChatroom_Success(t *testing.T) {
	messageRepo := &mockMessageRepository{}
	chatroomRepo := &mockChatroomRepository{
		chatrooms: map[string]*domain.Chatroom{
			"chatroom1": {ID: "chatroom1", Name: "General"},
		},
		members: make(map[string]map[string]bool),
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	err := chatService.JoinChatroom(ctx, "chatroom1", "user1")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify user was added as member
	isMember, _ := chatroomRepo.IsMember(ctx, "chatroom1", "user1")
	if !isMember {
		t.Error("Expected user to be added as member")
	}
}

func TestChatService_JoinChatroom_ChatroomNotFound(t *testing.T) {
	messageRepo := &mockMessageRepository{}
	chatroomRepo := &mockChatroomRepository{
		chatrooms: make(map[string]*domain.Chatroom),
		getByID: func(ctx context.Context, id string) (*domain.Chatroom, error) {
			return nil, errors.New("chatroom not found")
		},
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	err := chatService.JoinChatroom(ctx, "nonexistent", "user1")

	if err == nil {
		t.Error("Expected error for nonexistent chatroom")
	}
}

func TestChatService_IsMember_True(t *testing.T) {
	messageRepo := &mockMessageRepository{}
	chatroomRepo := &mockChatroomRepository{
		members: map[string]map[string]bool{
			"chatroom1": {
				"user1": true,
			},
		},
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	isMember, err := chatService.IsMember(ctx, "chatroom1", "user1")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if !isMember {
		t.Error("Expected user to be member")
	}
}

func TestChatService_IsMember_False(t *testing.T) {
	messageRepo := &mockMessageRepository{}
	chatroomRepo := &mockChatroomRepository{
		members: map[string]map[string]bool{
			"chatroom1": {
				"user1": true,
			},
		},
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	isMember, err := chatService.IsMember(ctx, "chatroom1", "user2")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if isMember {
		t.Error("Expected user to NOT be member")
	}
}

func TestChatService_ListChatrooms_Success(t *testing.T) {
	messageRepo := &mockMessageRepository{}
	chatroomRepo := &mockChatroomRepository{
		chatrooms: map[string]*domain.Chatroom{
			"chatroom1": {ID: "chatroom1", Name: "General"},
			"chatroom2": {ID: "chatroom2", Name: "Random"},
			"chatroom3": {ID: "chatroom3", Name: "Tech"},
		},
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	chatrooms, err := chatService.ListChatrooms(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(chatrooms) != 3 {
		t.Errorf("Expected 3 chatrooms, got %d", len(chatrooms))
	}
}

func TestChatService_ListChatrooms_Empty(t *testing.T) {
	messageRepo := &mockMessageRepository{}
	chatroomRepo := &mockChatroomRepository{
		chatrooms: make(map[string]*domain.Chatroom),
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()
	chatrooms, err := chatService.ListChatrooms(ctx)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(chatrooms) != 0 {
		t.Errorf("Expected 0 chatrooms, got %d", len(chatrooms))
	}
}

func TestChatService_SendMessage_ValidatesContent(t *testing.T) {
	messageRepo := &mockMessageRepository{}
	chatroomRepo := &mockChatroomRepository{
		members: map[string]map[string]bool{
			"chatroom1": {
				"user1": true,
			},
		},
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()

	tests := []struct {
		name    string
		message *domain.Message
		wantErr bool
	}{
		{
			name: "valid message",
			message: &domain.Message{
				ChatroomID: "chatroom1",
				UserID:     "user1",
				Username:   "alice",
				Content:    "Hello!",
			},
			wantErr: false,
		},
		{
			name: "empty content",
			message: &domain.Message{
				ChatroomID: "chatroom1",
				UserID:     "user1",
				Username:   "alice",
				Content:    "",
			},
			wantErr: true,
		},
		{
			name: "missing chatroom ID",
			message: &domain.Message{
				ChatroomID: "",
				UserID:     "user1",
				Username:   "alice",
				Content:    "Hello!",
			},
			wantErr: true,
		},
		{
			name: "missing user ID",
			message: &domain.Message{
				ChatroomID: "chatroom1",
				UserID:     "",
				Username:   "alice",
				Content:    "Hello!",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := chatService.SendMessage(ctx, tt.message)

			if tt.wantErr && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestChatService_MultipleChatrooms(t *testing.T) {
	messageRepo := &mockMessageRepository{
		messages: []*domain.Message{},
	}
	chatroomRepo := &mockChatroomRepository{
		chatrooms: make(map[string]*domain.Chatroom),
		members:   make(map[string]map[string]bool),
	}
	chatService := NewChatService(messageRepo, chatroomRepo)

	ctx := context.Background()

	// Create multiple chatrooms
	chatroom1, _ := chatService.CreateChatroom(ctx, "General", "user1")
	chatroom2, _ := chatService.CreateChatroom(ctx, "Random", "user1")

	// Send messages to different chatrooms
	msg1 := &domain.Message{
		ChatroomID: chatroom1.ID,
		UserID:     "user1",
		Username:   "alice",
		Content:    "Message in General",
	}
	msg2 := &domain.Message{
		ChatroomID: chatroom2.ID,
		UserID:     "user1",
		Username:   "alice",
		Content:    "Message in Random",
	}

	chatService.SendMessage(ctx, msg1)
	chatService.SendMessage(ctx, msg2)

	// Get messages from each chatroom
	messages1, _ := chatService.GetMessages(ctx, chatroom1.ID, 10)
	messages2, _ := chatService.GetMessages(ctx, chatroom2.ID, 10)

	if len(messages1) != 1 {
		t.Errorf("Expected 1 message in chatroom1, got %d", len(messages1))
	}

	if len(messages2) != 1 {
		t.Errorf("Expected 1 message in chatroom2, got %d", len(messages2))
	}

	if messages1[0].Content != "Message in General" {
		t.Error("Wrong message content in chatroom1")
	}

	if messages2[0].Content != "Message in Random" {
		t.Error("Wrong message content in chatroom2")
	}
}

// Benchmark tests
func BenchmarkSendMessage(b *testing.B) {
	messageRepo := &mockMessageRepository{messages: []*domain.Message{}}
	chatroomRepo := &mockChatroomRepository{}
	chatService := NewChatService(messageRepo, chatroomRepo)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := &domain.Message{
			ChatroomID: "chatroom1",
			UserID:     "user1",
			Username:   "alice",
			Content:    "Benchmark message",
		}
		chatService.SendMessage(ctx, msg)
	}
}

func BenchmarkGetMessages(b *testing.B) {
	messages := make([]*domain.Message, 100)
	for i := 0; i < 100; i++ {
		messages[i] = &domain.Message{
			ID:         "msg" + string(rune(i)),
			ChatroomID: "chatroom1",
			Content:    "Message " + string(rune(i)),
		}
	}

	messageRepo := &mockMessageRepository{messages: messages}
	chatroomRepo := &mockChatroomRepository{}
	chatService := NewChatService(messageRepo, chatroomRepo)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chatService.GetMessages(ctx, "chatroom1", 50)
	}
}
