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

// AuthService handles authentication logic
type AuthService struct {
	userRepo    domain.UserRepository
	sessionRepo domain.SessionRepository
}

// NewAuthService creates a new authentication service
func NewAuthService(userRepo domain.UserRepository, sessionRepo domain.SessionRepository) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, username, email, password string) (*domain.User, error) {
	// Validate input
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

	// Check if username exists
	if _, err := s.userRepo.GetByUsername(ctx, username); err == nil {
		return nil, domain.ErrUsernameExists
	}

	// Check if email exists
	if _, err := s.userRepo.GetByEmail(ctx, email); err == nil {
		return nil, domain.ErrEmailExists
	}

	// Hash password with bcrypt cost 12
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return nil, err
	}

	// Create user
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

// Login authenticates a user and creates a session
func (s *AuthService) Login(ctx context.Context, username, password string) (*domain.Session, *domain.User, error) {
	// Get user
	user, err := s.userRepo.GetByUsername(ctx, username)
	if err != nil {
		return nil, nil, domain.ErrInvalidCredentials
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash), []byte(password),
	); err != nil {
		return nil, nil, domain.ErrInvalidCredentials
	}

	// Create session
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

// Logout destroys a user session
func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessionRepo.Delete(ctx, token)
}

// ValidateSession validates a session token
func (s *AuthService) ValidateSession(ctx context.Context, token string) (*domain.Session, error) {
	return s.sessionRepo.GetByToken(ctx, token)
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(ctx context.Context, userID string) (*domain.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}

// GetUserByUsername retrieves a user by username
func (s *AuthService) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	return s.userRepo.GetByUsername(ctx, username)
}
