package service

import (
	"context"

	"jobsity-chat/internal/domain"
)

type ChatService struct {
	messageRepo  domain.MessageRepository
	chatroomRepo domain.ChatroomRepository
}

func NewChatService(messageRepo domain.MessageRepository, chatroomRepo domain.ChatroomRepository) *ChatService {
	return &ChatService{
		messageRepo:  messageRepo,
		chatroomRepo: chatroomRepo,
	}
}

func (s *ChatService) SendMessage(ctx context.Context, msg *domain.Message) error {
	if !msg.IsBot {
		isMember, err := s.chatroomRepo.IsMember(ctx, msg.ChatroomID, msg.UserID)
		if err != nil {
			return err
		}
		if !isMember {
			return domain.ErrNotMember
		}
	}

	if len(msg.Content) == 0 || len(msg.Content) > 1000 {
		return domain.ErrInvalidInput
	}

	return s.messageRepo.Create(ctx, msg)
}

func (s *ChatService) GetMessages(ctx context.Context, chatroomID string, limit int) ([]*domain.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.messageRepo.GetByChatroom(ctx, chatroomID, limit)
}

func (s *ChatService) GetMessagesBefore(ctx context.Context, chatroomID string, before string, limit int) ([]*domain.Message, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.messageRepo.GetByChatroomBefore(ctx, chatroomID, before, limit)
}

func (s *ChatService) CreateChatroom(ctx context.Context, name, createdBy string) (*domain.Chatroom, error) {
	if len(name) == 0 || len(name) > 100 {
		return nil, domain.ErrInvalidInput
	}

	chatroom := &domain.Chatroom{
		Name:      name,
		CreatedBy: createdBy,
	}

	if err := s.chatroomRepo.CreateWithMember(ctx, chatroom, createdBy); err != nil {
		return nil, err
	}

	return chatroom, nil
}

func (s *ChatService) ListChatrooms(ctx context.Context) ([]*domain.Chatroom, error) {
	return s.chatroomRepo.List(ctx)
}

func (s *ChatService) JoinChatroom(ctx context.Context, chatroomID, userID string) error {
	if _, err := s.chatroomRepo.GetByID(ctx, chatroomID); err != nil {
		return err
	}

	return s.chatroomRepo.AddMember(ctx, chatroomID, userID)
}

func (s *ChatService) IsMember(ctx context.Context, chatroomID, userID string) (bool, error) {
	return s.chatroomRepo.IsMember(ctx, chatroomID, userID)
}
