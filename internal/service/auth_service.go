package service

import (
	"context"
	"regexp"
	"time"

	"jobsity-chat/internal/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	usernameRegex = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
)

type AuthService struct {
	userRepo    domain.UserRepository
	sessionRepo domain.SessionRepository
}

func NewAuthService(userRepo domain.UserRepository, sessionRepo domain.SessionRepository) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}
}

func (s *AuthService) Register(ctx context.Context, username, email, password string) (*domain.User, error) {
	if len(username) < 3 || len(username) > 50 {
		return nil, domain.ErrInvalidInput
	}
	if !usernameRegex.MatchString(username) {
		return nil, domain.ErrInvalidInput
	}
	if !emailRegex.MatchString(email) || len(email) > 255 {
		return nil, domain.ErrInvalidInput
	}
	if len(password) < 8 || len(password) > 100 {
		return nil, domain.ErrInvalidInput
	}

	if _, err := s.userRepo.GetByUsername(ctx, username); err == nil {
		return nil, domain.ErrUsernameExists
	}

	if _, err := s.userRepo.GetByEmail(ctx, email); err == nil {
		return nil, domain.ErrEmailExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hashedPassword),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (*domain.Session, *domain.User, error) {
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, nil, domain.ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash), []byte(password),
	); err != nil {
		return nil, nil, domain.ErrInvalidCredentials
	}

	session := &domain.Session{
		UserID:    user.ID,
		Token:     uuid.New().String(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, nil, err
	}

	return session, user, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessionRepo.Delete(ctx, token)
}

func (s *AuthService) ValidateSession(ctx context.Context, token string) (*domain.Session, error) {
	return s.sessionRepo.GetByToken(ctx, token)
}

func (s *AuthService) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

func (s *AuthService) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	return s.userRepo.GetByUsername(ctx, username)
}
