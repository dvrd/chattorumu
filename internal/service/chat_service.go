package service

import (
	"context"

	"jobsity-chat/internal/domain"
)

// ChatService handles chat business logic
type ChatService struct {
	messageRepo  domain.MessageRepository
	chatroomRepo domain.ChatroomRepository
}

// NewChatService creates a new chat service
func NewChatService(messageRepo domain.MessageRepository, chatroomRepo domain.ChatroomRepository) *ChatService {
	return &ChatService{
		messageRepo:  messageRepo,
		chatroomRepo: chatroomRepo,
	}
}

// SendMessage saves a message to the database
func (s *ChatService) SendMessage(ctx context.Context, msg *domain.Message) error {
	// Validate chatroom membership
	isMember, err := s.chatroomRepo.IsMember(ctx, msg.ChatroomID, msg.UserID)
	if err != nil {
		return err
	}
	if !isMember {
		return domain.ErrNotMember
	}

	// Validate message content
	if len(msg.Content) == 0 || len(msg.Content) > 1000 {
		return domain.ErrInvalidInput
	}

	// Save message
	return s.messageRepo.Create(ctx, msg)
}

// GetMessages retrieves the last N messages for a chatroom
func (s *ChatService) GetMessages(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.messageRepo.GetByChatroom(ctx, chatroomID, limit)
}

// CreateChatroom creates a new chatroom
func (s *ChatService) CreateChatroom(ctx context.Context, name, createdBy string) (*domain.Chatroom, error) {
	if len(name) == 0 || len(name) > 100 {
		return nil, domain.ErrInvalidInput
	}

	chatroom := &domain.Chatroom{
		Name:      name,
		CreatedBy: createdBy,
	}

	if err := s.chatroomRepo.Create(ctx, chatroom); err != nil {
		return nil, err
	}

	// Automatically add creator as member
	if err := s.chatroomRepo.AddMember(ctx, chatroom.ID, createdBy); err != nil {
		return nil, err
	}

	return chatroom, nil
}

// ListChatrooms retrieves all chatrooms
func (s *ChatService) ListChatrooms(ctx context.Context) ([]*domain.Chatroom, error) {
	return s.chatroomRepo.List(ctx)
}

// JoinChatroom adds a user to a chatroom
func (s *ChatService) JoinChatroom(ctx context.Context, chatroomID, userID string) error {
	// Verify chatroom exists
	if _, err := s.chatroomRepo.GetByID(ctx, chatroomID); err != nil {
		return err
	}

	return s.chatroomRepo.AddMember(ctx, chatroomID, userID)
}

// IsMember checks if a user is a member of a chatroom
func (s *ChatService) IsMember(ctx context.Context, chatroomID, userID string) (bool, error) {
	return s.chatroomRepo.IsMember(ctx, chatroomID, userID)
}
