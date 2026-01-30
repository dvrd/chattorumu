package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/middleware"
	"jobsity-chat/internal/service"

	"golang.org/x/crypto/bcrypt"
)

// mockUserRepository implements domain.UserRepository for testing
type mockUserRepository struct {
	createFunc   func(ctx context.Context, user *domain.User) error
	getByIDFunc  func(ctx context.Context, id string) (*domain.User, error)
	getUsernameFunc func(ctx context.Context, username string) (*domain.User, error)
	getEmailFunc func(ctx context.Context, email string) (*domain.User, error)
}

func (m *mockUserRepository) Create(ctx context.Context, user *domain.User) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, user)
	}
	return errors.New("not implemented")
}

func (m *mockUserRepository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	if m.getByIDFunc != nil {
		return m.getByIDFunc(ctx, id)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	if m.getUsernameFunc != nil {
		return m.getUsernameFunc(ctx, username)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	if m.getEmailFunc != nil {
		return m.getEmailFunc(ctx, email)
	}
	return nil, errors.New("not implemented")
}

// mockSessionRepository implements domain.SessionRepository for testing
type mockSessionRepository struct {
	createFunc        func(ctx context.Context, session *domain.Session) error
	getByTokenFunc    func(ctx context.Context, token string) (*domain.Session, error)
	deleteFunc        func(ctx context.Context, token string) error
	deleteExpiredFunc func(ctx context.Context) (int64, error)
}

func (m *mockSessionRepository) Create(ctx context.Context, session *domain.Session) error {
	if m.createFunc != nil {
		return m.createFunc(ctx, session)
	}
	return errors.New("not implemented")
}

func (m *mockSessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	if m.getByTokenFunc != nil {
		return m.getByTokenFunc(ctx, token)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSessionRepository) Delete(ctx context.Context, token string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, token)
	}
	return errors.New("not implemented")
}

func (m *mockSessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	if m.deleteExpiredFunc != nil {
		return m.deleteExpiredFunc(ctx)
	}
	return 0, nil
}

func TestAuthHandler_Register_Success(t *testing.T) {
	userRepo := &mockUserRepository{
		createFunc: func(ctx context.Context, user *domain.User) error {
			user.ID = "user-123"
			user.CreatedAt = time.Now()
			return nil
		},
	}
	sessionRepo := &mockSessionRepository{}

	authService := service.NewAuthService(userRepo, sessionRepo)
	handler := NewAuthHandler(authService)

	reqBody := `{"username":"testuser","email":"test@example.com","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status %d, got %d, body: %s", http.StatusCreated, w.Code, w.Body.String())
	}

	var resp RegisterResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "user-123" {
		t.Errorf("expected ID 'user-123', got '%s'", resp.ID)
	}
	if resp.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", resp.Username)
	}
	if resp.Email != "test@example.com" {
		t.Errorf("expected email 'test@example.com', got '%s'", resp.Email)
	}
}

func TestAuthHandler_Register_InvalidJSON(t *testing.T) {
	authService := service.NewAuthService(&mockUserRepository{}, &mockSessionRepository{})
	handler := NewAuthHandler(authService)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`invalid json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Invalid request body") {
		t.Errorf("expected error message about invalid request body, got: %s", w.Body.String())
	}
}

func TestAuthHandler_Register_ValidationErrors(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		userRepoSetup  func() *mockUserRepository
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:        "invalid input - short username",
			requestBody: `{"username":"ab","email":"test@test.com","password":"password123"}`,
			userRepoSetup: func() *mockUserRepository {
				return &mockUserRepository{}
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Invalid input",
		},
		{
			name:        "invalid input - short password",
			requestBody: `{"username":"testuser","email":"test@test.com","password":"short"}`,
			userRepoSetup: func() *mockUserRepository {
				return &mockUserRepository{}
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Invalid input",
		},
		{
			name:        "invalid input - invalid email",
			requestBody: `{"username":"testuser","email":"notanemail","password":"password123"}`,
			userRepoSetup: func() *mockUserRepository {
				return &mockUserRepository{}
			},
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Invalid input",
		},
		{
			name:        "username exists",
			requestBody: `{"username":"existing","email":"test@test.com","password":"password123"}`,
			userRepoSetup: func() *mockUserRepository {
				return &mockUserRepository{
					createFunc: func(ctx context.Context, user *domain.User) error {
						return domain.ErrUsernameExists
					},
				}
			},
			expectedStatus: http.StatusConflict,
			expectedMsg:    "User already exists",
		},
		{
			name:        "email exists",
			requestBody: `{"username":"testuser","email":"existing@test.com","password":"password123"}`,
			userRepoSetup: func() *mockUserRepository {
				return &mockUserRepository{
					createFunc: func(ctx context.Context, user *domain.User) error {
						return domain.ErrEmailExists
					},
				}
			},
			expectedStatus: http.StatusConflict,
			expectedMsg:    "User already exists",
		},
		{
			name:        "internal error",
			requestBody: `{"username":"testuser","email":"test@test.com","password":"password123"}`,
			userRepoSetup: func() *mockUserRepository {
				return &mockUserRepository{
					createFunc: func(ctx context.Context, user *domain.User) error {
						return errors.New("database error")
					},
				}
			},
			expectedStatus: http.StatusInternalServerError,
			expectedMsg:    "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userRepo := tt.userRepoSetup()
			sessionRepo := &mockSessionRepository{}
			authService := service.NewAuthService(userRepo, sessionRepo)
			handler := NewAuthHandler(authService)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Register(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body: %s", tt.expectedStatus, w.Code, w.Body.String())
			}

			if !strings.Contains(w.Body.String(), tt.expectedMsg) {
				t.Errorf("expected error message '%s', got: %s", tt.expectedMsg, w.Body.String())
			}
		})
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	// Save original env and restore after test
	originalEnv := os.Getenv("ENVIRONMENT")
	defer os.Setenv("ENVIRONMENT", originalEnv)

	// Hash a password for testing
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	tests := []struct {
		name             string
		environment      string
		expectedSecure   bool
		expectedSameSite http.SameSite
	}{
		{
			name:             "production environment",
			environment:      "production",
			expectedSecure:   true,
			expectedSameSite: http.SameSiteLaxMode,
		},
		{
			name:             "development environment",
			environment:      "development",
			expectedSecure:   false,
			expectedSameSite: http.SameSiteLaxMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("ENVIRONMENT", tt.environment)

			userRepo := &mockUserRepository{
				getUsernameFunc: func(ctx context.Context, username string) (*domain.User, error) {
					return &domain.User{
						ID:           "user-123",
						Username:     "testuser",
						Email:        "test@example.com",
						PasswordHash: string(hashedPassword),
					}, nil
				},
			}

			sessionRepo := &mockSessionRepository{
				createFunc: func(ctx context.Context, session *domain.Session) error {
					// Session token is generated by service
					return nil
				},
			}

			authService := service.NewAuthService(userRepo, sessionRepo)
			handler := NewAuthHandler(authService)

			reqBody := `{"username":"testuser","password":"password123"}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.Login(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("expected status %d, got %d, body: %s", http.StatusOK, w.Code, w.Body.String())
			}

			// Check response body
			var resp LoginResponse
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if !resp.Success {
				t.Error("expected success to be true")
			}
			if resp.User.Username != "testuser" {
				t.Errorf("expected username 'testuser', got '%s'", resp.User.Username)
			}
			if resp.SessionToken == "" {
				t.Error("expected non-empty session token")
			}

			// Check cookie
			cookies := w.Result().Cookies()
			if len(cookies) != 1 {
				t.Fatalf("expected 1 cookie, got %d", len(cookies))
			}

			cookie := cookies[0]
			if cookie.Name != "session_id" {
				t.Errorf("expected cookie name 'session_id', got '%s'", cookie.Name)
			}
			if cookie.Value == "" {
				t.Error("expected non-empty cookie value")
			}
			if cookie.HttpOnly != true {
				t.Error("expected HttpOnly to be true")
			}
			if cookie.Secure != tt.expectedSecure {
				t.Errorf("expected Secure to be %v, got %v", tt.expectedSecure, cookie.Secure)
			}
			if cookie.SameSite != tt.expectedSameSite {
				t.Errorf("expected SameSite to be %v, got %v", tt.expectedSameSite, cookie.SameSite)
			}
		})
	}
}

func TestAuthHandler_Login_InvalidJSON(t *testing.T) {
	authService := service.NewAuthService(&mockUserRepository{}, &mockSessionRepository{})
	handler := NewAuthHandler(authService)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`invalid json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	userRepo := &mockUserRepository{
		getUsernameFunc: func(ctx context.Context, username string) (*domain.User, error) {
			return nil, errors.New("user not found")
		},
	}

	sessionRepo := &mockSessionRepository{}
	authService := service.NewAuthService(userRepo, sessionRepo)
	handler := NewAuthHandler(authService)

	reqBody := `{"username":"testuser","password":"wrongpassword"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Invalid credentials") {
		t.Errorf("expected error message about invalid credentials, got: %s", w.Body.String())
	}
}

func TestAuthHandler_Login_InternalError(t *testing.T) {
	// Hash a password for testing
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	// Internal error during session creation (after successful auth)
	userRepo := &mockUserRepository{
		getUsernameFunc: func(ctx context.Context, username string) (*domain.User, error) {
			return &domain.User{
				ID:           "user-123",
				Username:     "testuser",
				Email:        "test@example.com",
				PasswordHash: string(hashedPassword),
			}, nil
		},
	}

	sessionRepo := &mockSessionRepository{
		createFunc: func(ctx context.Context, session *domain.Session) error {
			return errors.New("database connection failed")
		},
	}

	authService := service.NewAuthService(userRepo, sessionRepo)
	handler := NewAuthHandler(authService)

	reqBody := `{"username":"testuser","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Internal server error") {
		t.Errorf("expected generic error message, got: %s", w.Body.String())
	}
}

func TestAuthHandler_Me_Success(t *testing.T) {
	userRepo := &mockUserRepository{
		getByIDFunc: func(ctx context.Context, userID string) (*domain.User, error) {
			return &domain.User{
				ID:       userID,
				Username: "testuser",
				Email:    "test@example.com",
			}, nil
		},
	}

	sessionRepo := &mockSessionRepository{}
	authService := service.NewAuthService(userRepo, sessionRepo)
	handler := NewAuthHandler(authService)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	// Add user ID to context (simulating auth middleware)
	ctx := middleware.WithUserID(req.Context(), "user-123")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Me(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp RegisterResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "user-123" {
		t.Errorf("expected ID 'user-123', got '%s'", resp.ID)
	}
	if resp.Username != "testuser" {
		t.Errorf("expected username 'testuser', got '%s'", resp.Username)
	}
}

func TestAuthHandler_Me_NoUserIDInContext(t *testing.T) {
	authService := service.NewAuthService(&mockUserRepository{}, &mockSessionRepository{})
	handler := NewAuthHandler(authService)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()

	handler.Me(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Unauthorized") {
		t.Errorf("expected unauthorized error, got: %s", w.Body.String())
	}
}

func TestAuthHandler_Me_UserNotFound(t *testing.T) {
	userRepo := &mockUserRepository{
		getByIDFunc: func(ctx context.Context, userID string) (*domain.User, error) {
			return nil, errors.New("user not found")
		},
	}

	sessionRepo := &mockSessionRepository{}
	authService := service.NewAuthService(userRepo, sessionRepo)
	handler := NewAuthHandler(authService)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	ctx := middleware.WithUserID(req.Context(), "nonexistent-user")
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Me(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestAuthHandler_Logout_Success(t *testing.T) {
	// Save original env and restore after test
	originalEnv := os.Getenv("ENVIRONMENT")
	defer os.Setenv("ENVIRONMENT", originalEnv)
	os.Setenv("ENVIRONMENT", "production")

	sessionRepo := &mockSessionRepository{
		deleteFunc: func(ctx context.Context, token string) error {
			if token == "session-token-123" {
				return nil
			}
			return errors.New("invalid token")
		},
	}

	authService := service.NewAuthService(&mockUserRepository{}, sessionRepo)
	handler := NewAuthHandler(authService)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	// Add session to context (simulating auth middleware)
	ctx := middleware.WithSession(req.Context(), &domain.Session{
		Token:  "session-token-123",
		UserID: "user-123",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// Check response
	var resp map[string]bool
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp["success"] {
		t.Error("expected success to be true")
	}

	// Check cookie is cleared
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	cookie := cookies[0]
	if cookie.Name != "session_id" {
		t.Errorf("expected cookie name 'session_id', got '%s'", cookie.Name)
	}
	if cookie.Value != "" {
		t.Errorf("expected empty cookie value, got '%s'", cookie.Value)
	}
	if cookie.MaxAge != -1 {
		t.Errorf("expected MaxAge -1, got %d", cookie.MaxAge)
	}
	if !cookie.Secure {
		t.Error("expected Secure to be true in production")
	}
}

func TestAuthHandler_Logout_NoSessionInContext(t *testing.T) {
	authService := service.NewAuthService(&mockUserRepository{}, &mockSessionRepository{})
	handler := NewAuthHandler(authService)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthHandler_Logout_ServiceError(t *testing.T) {
	sessionRepo := &mockSessionRepository{
		deleteFunc: func(ctx context.Context, token string) error {
			return errors.New("database error")
		},
	}

	authService := service.NewAuthService(&mockUserRepository{}, sessionRepo)
	handler := NewAuthHandler(authService)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	ctx := middleware.WithSession(req.Context(), &domain.Session{
		Token:  "session-token-123",
		UserID: "user-123",
	})
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	if !strings.Contains(w.Body.String(), "Failed to logout") {
		t.Errorf("expected error message about logout failure, got: %s", w.Body.String())
	}
}
