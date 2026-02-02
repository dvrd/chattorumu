package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"jobsity-chat/internal/domain"
)

// Mock repositories for testing
type mockUserRepository struct {
	users         map[string]*domain.User
	getByUsername func(ctx context.Context, username string) (*domain.User, error)
	getByEmail    func(ctx context.Context, email string) (*domain.User, error)
	getByID       func(ctx context.Context, id string) (*domain.User, error)
	create        func(ctx context.Context, user *domain.User) error
}

func (m *mockUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	if m.getByUsername != nil {
		return m.getByUsername(ctx, username)
	}
	user, ok := m.users[username]
	if !ok {
		return nil, errors.New("user not found")
	}
	return user, nil
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.getByEmail != nil {
		return m.getByEmail(ctx, email)
	}
	for _, user := range m.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if m.getByID != nil {
		return m.getByID(ctx, id)
	}
	for _, user := range m.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *mockUserRepository) Create(ctx context.Context, user *domain.User) error {
	if m.create != nil {
		return m.create(ctx, user)
	}
	if m.users == nil {
		m.users = make(map[string]*domain.User)
	}
	if user.ID == "" {
		user.ID = "user-" + user.Username
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	m.users[user.Username] = user
	return nil
}

type mockSessionRepository struct {
	sessions        map[string]*domain.Session
	create          func(ctx context.Context, session *domain.Session) error
	getByToken      func(ctx context.Context, token string) (*domain.Session, error)
	getByCSRFToken  func(ctx context.Context, csrfToken string) (*domain.Session, error)
	updateCSRFToken func(ctx context.Context, csrfToken, sessionToken string) error
	delete          func(ctx context.Context, token string) error
	deleteByUserID  func(ctx context.Context, userID string) error
	deleteExpired   func(ctx context.Context) (int64, error)
}

func (m *mockSessionRepository) Create(ctx context.Context, session *domain.Session) error {
	if m.create != nil {
		return m.create(ctx, session)
	}
	if m.sessions == nil {
		m.sessions = make(map[string]*domain.Session)
	}
	m.sessions[session.Token] = session
	return nil
}

func (m *mockSessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	if m.getByToken != nil {
		return m.getByToken(ctx, token)
	}
	session, ok := m.sessions[token]
	if !ok {
		return nil, errors.New("session not found")
	}
	return session, nil
}

func (m *mockSessionRepository) GetByCSRFToken(ctx context.Context, csrfToken string) (*domain.Session, error) {
	if m.getByCSRFToken != nil {
		return m.getByCSRFToken(ctx, csrfToken)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSessionRepository) UpdateCSRFToken(ctx context.Context, csrfToken, sessionToken string) error {
	if m.updateCSRFToken != nil {
		return m.updateCSRFToken(ctx, csrfToken, sessionToken)
	}
	return nil
}

func (m *mockSessionRepository) Delete(ctx context.Context, token string) error {
	if m.delete != nil {
		return m.delete(ctx, token)
	}
	delete(m.sessions, token)
	return nil
}

func (m *mockSessionRepository) DeleteByUserID(ctx context.Context, userID string) error {
	if m.deleteByUserID != nil {
		return m.deleteByUserID(ctx, userID)
	}
	return nil
}

func (m *mockSessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	if m.deleteExpired != nil {
		return m.deleteExpired(ctx)
	}
	return 0, nil
}

func TestAuthService_Register_Success(t *testing.T) {
	userRepo := &mockUserRepository{
		users: make(map[string]*domain.User),
	}
	sessionRepo := &mockSessionRepository{}
	authService := NewAuthService(userRepo, sessionRepo)

	ctx := context.Background()
	user, err := authService.Register(ctx, "alice", "alice@example.com", "password123")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if user == nil {
		t.Fatal("Expected non-nil user")
	}

	if user.Username != "alice" {
		t.Errorf("Expected username 'alice', got %s", user.Username)
	}

	if user.Email != "alice@example.com" {
		t.Errorf("Expected email 'alice@example.com', got %s", user.Email)
	}

	if user.ID == "" {
		t.Error("Expected user ID to be set")
	}

	if user.PasswordHash == "" {
		t.Error("Expected password hash to be set")
	}

	if user.PasswordHash == "password123" {
		t.Error("Password should be hashed, not stored in plain text")
	}

	if user.CreatedAt.IsZero() {
		t.Error("Expected CreatedAt to be set")
	}
}

func TestAuthService_Register_DuplicateUsername(t *testing.T) {
	userRepo := &mockUserRepository{
		users: map[string]*domain.User{
			"alice": {
				ID:       "user1",
				Username: "alice",
				Email:    "alice@example.com",
			},
		},
	}
	sessionRepo := &mockSessionRepository{}
	authService := NewAuthService(userRepo, sessionRepo)

	ctx := context.Background()
	user, err := authService.Register(ctx, "alice", "newalice@example.com", "password123")

	if err == nil {
		t.Error("Expected error for duplicate username")
	}

	if user != nil {
		t.Errorf("Expected nil user, got: %+v", user)
	}

	if !errors.Is(err, domain.ErrUsernameExists) {
		t.Errorf("Expected ErrUsernameExists, got: %v", err)
	}
}

func TestAuthService_Register_DuplicateEmail(t *testing.T) {
	userRepo := &mockUserRepository{
		getByEmail: func(ctx context.Context, email string) (*domain.User, error) {
			if email == "alice@example.com" {
				return &domain.User{
					ID:       "user1",
					Username: "alice",
					Email:    "alice@example.com",
				}, nil
			}
			return nil, errors.New("user not found")
		},
		getByUsername: func(ctx context.Context, username string) (*domain.User, error) {
			return nil, errors.New("user not found")
		},
	}
	sessionRepo := &mockSessionRepository{}
	authService := NewAuthService(userRepo, sessionRepo)

	ctx := context.Background()
	user, err := authService.Register(ctx, "bob", "alice@example.com", "password123")

	if err == nil {
		t.Error("Expected error for duplicate email")
	}

	if user != nil {
		t.Errorf("Expected nil user, got: %+v", user)
	}

	if !errors.Is(err, domain.ErrEmailExists) {
		t.Errorf("Expected ErrEmailExists, got: %v", err)
	}
}

func TestAuthService_Register_InvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		username string
		email    string
		password string
	}{
		{
			name:     "empty username",
			username: "",
			email:    "alice@example.com",
			password: "password123",
		},
		{
			name:     "empty email",
			username: "alice",
			email:    "",
			password: "password123",
		},
		{
			name:     "empty password",
			username: "alice",
			email:    "alice@example.com",
			password: "",
		},
		{
			name:     "short username",
			username: "ab",
			email:    "alice@example.com",
			password: "password123",
		},
		{
			name:     "short password",
			username: "alice",
			email:    "alice@example.com",
			password: "12345",
		},
		{
			name:     "invalid email format",
			username: "alice",
			email:    "not-an-email",
			password: "password123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepository{users: make(map[string]*domain.User)}
			sessionRepo := &mockSessionRepository{}
			authService := NewAuthService(userRepo, sessionRepo)

			ctx := context.Background()
			user, err := authService.Register(ctx, tt.username, tt.email, tt.password)

			if err == nil {
				t.Error("Expected error for invalid input")
			}

			if user != nil {
				t.Errorf("Expected nil user, got: %+v", user)
			}

			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Errorf("Expected ErrInvalidInput, got: %v", err)
			}
		})
	}
}

func TestAuthService_Login_Success(t *testing.T) {
	// Create a user with a known password hash
	userRepo := &mockUserRepository{
		users: make(map[string]*domain.User),
	}
	sessionRepo := &mockSessionRepository{
		sessions: make(map[string]*domain.Session),
	}
	authService := NewAuthService(userRepo, sessionRepo)

	// Register a user first
	ctx := context.Background()
	_, err := authService.Register(ctx, "alice", "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	// Now try to login
	session, user, err := authService.Login(ctx, "alice", "password123")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if session == nil {
		t.Fatal("Expected non-nil session")
	}

	if user == nil {
		t.Fatal("Expected non-nil user")
	}

	if user.Username != "alice" {
		t.Errorf("Expected username 'alice', got %s", user.Username)
	}

	if session.Token == "" {
		t.Error("Expected session token to be set")
	}

	if session.UserID == "" {
		t.Error("Expected session user ID to be set")
	}

	if session.ExpiresAt.Before(time.Now()) {
		t.Error("Expected session to not be expired")
	}

	// Verify session is 24 hours in the future
	expectedExpiry := time.Now().Add(24 * time.Hour)
	diff := session.ExpiresAt.Sub(expectedExpiry).Abs()
	if diff > time.Minute {
		t.Errorf("Expected session to expire in ~24 hours, but difference is %v", diff)
	}
}

func TestAuthService_Login_InvalidCredentials(t *testing.T) {
	userRepo := &mockUserRepository{
		users: make(map[string]*domain.User),
	}
	sessionRepo := &mockSessionRepository{}
	authService := NewAuthService(userRepo, sessionRepo)

	// Register a user
	ctx := context.Background()
	_, err := authService.Register(ctx, "alice", "alice@example.com", "password123")
	if err != nil {
		t.Fatalf("Failed to register user: %v", err)
	}

	// Try to login with wrong password
	session, user, err := authService.Login(ctx, "alice", "wrongpassword")

	if err == nil {
		t.Error("Expected error for invalid credentials")
	}

	if session != nil {
		t.Errorf("Expected nil session, got: %+v", session)
	}

	if user != nil {
		t.Errorf("Expected nil user, got: %+v", user)
	}

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("Expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestAuthService_Login_UserNotFound(t *testing.T) {
	userRepo := &mockUserRepository{
		users: make(map[string]*domain.User),
	}
	sessionRepo := &mockSessionRepository{}
	authService := NewAuthService(userRepo, sessionRepo)

	ctx := context.Background()
	session, user, err := authService.Login(ctx, "nonexistent", "password123")

	if err == nil {
		t.Error("Expected error for user not found")
	}

	if session != nil {
		t.Errorf("Expected nil session, got: %+v", session)
	}

	if user != nil {
		t.Errorf("Expected nil user, got: %+v", user)
	}

	if !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("Expected ErrInvalidCredentials, got: %v", err)
	}
}

func TestAuthService_Logout_Success(t *testing.T) {
	userRepo := &mockUserRepository{}
	sessionRepo := &mockSessionRepository{
		sessions: make(map[string]*domain.Session),
	}
	authService := NewAuthService(userRepo, sessionRepo)

	// Create a session
	ctx := context.Background()
	token := "test-token-123"
	sessionRepo.sessions[token] = &domain.Session{
		ID:        "session1",
		UserID:    "user1",
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	// Logout
	err := authService.Logout(ctx, token)

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify session was deleted
	_, exists := sessionRepo.sessions[token]
	if exists {
		t.Error("Expected session to be deleted")
	}
}

func TestAuthService_GetUserByID_Success(t *testing.T) {
	userRepo := &mockUserRepository{
		users: map[string]*domain.User{
			"alice": {
				ID:       "user1",
				Username: "alice",
				Email:    "alice@example.com",
			},
		},
	}
	sessionRepo := &mockSessionRepository{}
	authService := NewAuthService(userRepo, sessionRepo)

	ctx := context.Background()
	user, err := authService.GetUserByID(ctx, "user1")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if user == nil {
		t.Fatal("Expected non-nil user")
	}

	if user.Username != "alice" {
		t.Errorf("Expected username 'alice', got %s", user.Username)
	}
}

func TestAuthService_GetUserByID_NotFound(t *testing.T) {
	userRepo := &mockUserRepository{
		users: make(map[string]*domain.User),
	}
	sessionRepo := &mockSessionRepository{}
	authService := NewAuthService(userRepo, sessionRepo)

	ctx := context.Background()
	user, err := authService.GetUserByID(ctx, "nonexistent")

	if err == nil {
		t.Error("Expected error for user not found")
	}

	if user != nil {
		t.Errorf("Expected nil user, got: %+v", user)
	}
}

func TestAuthService_PasswordHashing(t *testing.T) {
	userRepo := &mockUserRepository{
		users: make(map[string]*domain.User),
	}
	sessionRepo := &mockSessionRepository{}
	authService := NewAuthService(userRepo, sessionRepo)

	ctx := context.Background()

	// Register two users with the same password
	user1, _ := authService.Register(ctx, "alice", "alice@example.com", "samepassword")
	user2, _ := authService.Register(ctx, "bob", "bob@example.com", "samepassword")

	// Password hashes should be different (due to salt)
	if user1.PasswordHash == user2.PasswordHash {
		t.Error("Expected different password hashes for same password (salt should differ)")
	}

	// Both should be able to login with the same password
	_, _, err1 := authService.Login(ctx, "alice", "samepassword")
	_, _, err2 := authService.Login(ctx, "bob", "samepassword")

	if err1 != nil || err2 != nil {
		t.Error("Expected both users to login successfully with the same password")
	}
}

func TestAuthService_SessionTokenUniqueness(t *testing.T) {
	userRepo := &mockUserRepository{
		users: make(map[string]*domain.User),
	}
	sessionRepo := &mockSessionRepository{
		sessions: make(map[string]*domain.Session),
	}
	authService := NewAuthService(userRepo, sessionRepo)

	ctx := context.Background()

	// Register a user
	authService.Register(ctx, "alice", "alice@example.com", "password123")

	// Create multiple sessions
	session1, _, _ := authService.Login(ctx, "alice", "password123")
	session2, _, _ := authService.Login(ctx, "alice", "password123")

	// Tokens should be unique
	if session1.Token == session2.Token {
		t.Error("Expected unique session tokens")
	}
}

func TestAuthService_EmailValidation(t *testing.T) {
	tests := []struct {
		name  string
		email string
		valid bool
	}{
		{"valid email", "alice@example.com", true},
		{"valid with subdomain", "alice@mail.example.com", true},
		{"valid with plus", "alice+tag@example.com", true},
		{"no at sign", "aliceexample.com", false},
		{"no domain", "alice@", false},
		{"no local part", "@example.com", false},
		{"multiple at signs", "alice@@example.com", false},
		{"no TLD", "alice@example", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepository{users: make(map[string]*domain.User)}
			sessionRepo := &mockSessionRepository{}
			authService := NewAuthService(userRepo, sessionRepo)

			ctx := context.Background()
			_, err := authService.Register(ctx, "alice", tt.email, "password123")

			if tt.valid && err != nil {
				t.Errorf("Expected valid email %s to be accepted, got error: %v", tt.email, err)
			}

			if !tt.valid && err == nil {
				t.Errorf("Expected invalid email %s to be rejected", tt.email)
			}
		})
	}
}

func TestAuthService_UsernameValidation(t *testing.T) {
	tests := []struct {
		name     string
		username string
		valid    bool
	}{
		{"valid username", "alice", true},
		{"valid with numbers", "alice123", true},
		{"valid with underscore", "alice_bob", true},
		{"minimum length (3 chars)", "abc", true},
		{"too short (2 chars)", "ab", false},
		{"empty", "", false},
		{"with spaces", "alice bob", false},
		{"with special chars", "alice@bob", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := &mockUserRepository{users: make(map[string]*domain.User)}
			sessionRepo := &mockSessionRepository{}
			authService := NewAuthService(userRepo, sessionRepo)

			ctx := context.Background()
			_, err := authService.Register(ctx, tt.username, "test@example.com", "password123")

			if tt.valid && err != nil {
				t.Errorf("Expected valid username %q to be accepted, got error: %v", tt.username, err)
			}

			if !tt.valid && err == nil {
				t.Errorf("Expected invalid username %q to be rejected", tt.username)
			}
		})
	}
}

// Benchmark tests
func BenchmarkRegister(b *testing.B) {
	userRepo := &mockUserRepository{users: make(map[string]*domain.User)}
	sessionRepo := &mockSessionRepository{}
	authService := NewAuthService(userRepo, sessionRepo)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		username := "user" + string(rune(i))
		authService.Register(ctx, username, username+"@example.com", "password123")
	}
}

func BenchmarkLogin(b *testing.B) {
	userRepo := &mockUserRepository{users: make(map[string]*domain.User)}
	sessionRepo := &mockSessionRepository{sessions: make(map[string]*domain.Session)}
	authService := NewAuthService(userRepo, sessionRepo)
	ctx := context.Background()

	// Register a user
	authService.Register(ctx, "alice", "alice@example.com", "password123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		authService.Login(ctx, "alice", "password123")
	}
}
