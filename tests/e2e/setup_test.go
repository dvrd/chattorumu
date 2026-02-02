//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for the jobsity-chat application.
// These tests verify the complete user flow including authentication,
// chatroom management, WebSocket messaging, and stock bot integration.
package e2e

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"
	"time"

	"jobsity-chat/internal/config"
	"jobsity-chat/internal/handler"
	"jobsity-chat/internal/messaging"
	"jobsity-chat/internal/middleware"
	"jobsity-chat/internal/repository/postgres"
	"jobsity-chat/internal/service"
	"jobsity-chat/internal/websocket"

	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testServer       *http.Server
	testHub          *websocket.Hub
	testDB           *sql.DB
	rmq              *messaging.RabbitMQ
	responseConsumer *messaging.ResponseConsumer
	baseURL          string
	wsURL            string
	testClient       *http.Client
	testContext      context.Context
	cancelFunc       context.CancelFunc
)

// TestMain sets up the E2E test environment
func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	testContext = ctx
	cancelFunc = cancel

	// Setup test environment
	cleanup, err := setupTestEnvironment(ctx)
	if err != nil {
		log.Fatalf("failed to setup test environment: %v", err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	cleanup()
	cancel()

	os.Exit(code)
}

// setupTestEnvironment starts PostgreSQL, RabbitMQ, and the chat server
func setupTestEnvironment(ctx context.Context) (func(), error) {
	var cleanups []func()

	// Start PostgreSQL
	pgContainer, pgCleanup, connStr, err := startPostgres(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start PostgreSQL: %w", err)
	}
	cleanups = append(cleanups, pgCleanup)
	_ = pgContainer

	// Connect to database
	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	cleanups = append(cleanups, func() { testDB.Close() })

	// Run migrations
	if err := runMigrations(testDB); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	// Start RabbitMQ
	rmqContainer, rmqCleanup, rmqURL, err := startRabbitMQ(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start RabbitMQ: %w", err)
	}
	cleanups = append(cleanups, rmqCleanup)
	_ = rmqContainer

	// Connect to RabbitMQ with timeout
	rmqCtx, rmqCancel := context.WithTimeout(ctx, 30*time.Second)
	rmq, err = messaging.NewRabbitMQWithRetry(rmqCtx, rmqURL)
	rmqCancel()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	cleanups = append(cleanups, func() { rmq.Close() })
	// rmq is now available as global variable for tests

	// Setup chat server
	serverCleanup, err := setupChatServer(testDB, rmq)
	if err != nil {
		return nil, fmt.Errorf("failed to setup chat server: %w", err)
	}
	cleanups = append(cleanups, serverCleanup)

	// Create HTTP client with cookies
	testClient = &http.Client{
		Timeout: 30 * time.Second,
		Jar:     nil, // We'll manage cookies manually for better control
	}

	cleanup := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	return cleanup, nil
}

// streamContainerLogs starts a goroutine that streams container logs to stdout with a prefix
func streamContainerLogs(ctx context.Context, container testcontainers.Container, prefix string) {
	go func() {
		reader, err := container.Logs(ctx)
		if err != nil {
			log.Printf("[%s] failed to get logs: %v", prefix, err)
			return
		}
		defer reader.Close()

		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			log.Printf("[%s] %s", prefix, scanner.Text())
		}

		if err := scanner.Err(); err != nil && err != io.EOF {
			log.Printf("[%s] log reader error: %v", prefix, err)
		}
	}()
}

// startPostgres starts a PostgreSQL container for testing
func startPostgres(ctx context.Context) (testcontainers.Container, func(), string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "postgres:15-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "test",
			"POSTGRES_PASSWORD": "test",
			"POSTGRES_DB":       "testdb",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2),
			wait.ForListeningPort("5432/tcp"),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, "", err
	}

	// Stream container logs
	streamContainerLogs(ctx, container, "PostgreSQL")

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, nil, "", err
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		container.Terminate(ctx)
		return nil, nil, "", err
	}

	connStr := fmt.Sprintf("postgres://test:test@%s:%s/testdb?sslmode=disable", host, port.Port())

	// Wait for PostgreSQL to be fully ready
	time.Sleep(2 * time.Second)

	cleanup := func() {
		container.Terminate(ctx)
	}

	return container, cleanup, connStr, nil
}

// startRabbitMQ starts a RabbitMQ container for testing
func startRabbitMQ(ctx context.Context) (testcontainers.Container, func(), string, error) {
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3.12-management-alpine",
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "guest",
			"RABBITMQ_DEFAULT_PASS": "guest",
		},
		WaitingFor: wait.ForAll(
			wait.ForLog("Server startup complete"),
			wait.ForListeningPort("5672/tcp"),
		).WithDeadline(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, nil, "", err
	}

	// Stream container logs
	streamContainerLogs(ctx, container, "RabbitMQ")

	host, err := container.Host(ctx)
	if err != nil {
		container.Terminate(ctx)
		return nil, nil, "", err
	}

	port, err := container.MappedPort(ctx, "5672")
	if err != nil {
		container.Terminate(ctx)
		return nil, nil, "", err
	}

	url := fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())

	// Wait for RabbitMQ to be fully ready
	time.Sleep(2 * time.Second)

	cleanup := func() {
		container.Terminate(ctx)
	}

	return container, cleanup, url, nil
}

// runMigrations creates the database schema
func runMigrations(db *sql.DB) error {
	schema := `
		CREATE EXTENSION IF NOT EXISTS "pgcrypto";

		CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			username VARCHAR(50) UNIQUE NOT NULL CHECK (length(username) >= 3),
			email VARCHAR(255) UNIQUE NOT NULL CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$'),
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS sessions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token VARCHAR(255) UNIQUE NOT NULL,
			expires_at TIMESTAMP NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
		);

		CREATE TABLE IF NOT EXISTS chatrooms (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(100) NOT NULL CHECK (length(name) >= 1),
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
			created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS chatroom_members (
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			chatroom_id UUID NOT NULL REFERENCES chatrooms(id) ON DELETE CASCADE,
			joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
			PRIMARY KEY (user_id, chatroom_id)
		);

		CREATE TABLE IF NOT EXISTS messages (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			chatroom_id UUID NOT NULL REFERENCES chatrooms(id) ON DELETE CASCADE,
			user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			content TEXT NOT NULL CHECK (length(content) > 0 AND length(content) <= 1000),
			is_bot BOOLEAN DEFAULT FALSE NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL
		);
	`
	_, err := db.Exec(schema)
	return err
}

// setupChatServer creates and starts the chat server
func setupChatServer(db *sql.DB, rmq *messaging.RabbitMQ) (func(), error) {
	// Create repositories
	userRepo, err := postgres.NewUserRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create user repository: %w", err)
	}

	sessionRepo, err := postgres.NewSessionRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create session repository: %w", err)
	}

	chatroomRepo, err := postgres.NewChatroomRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create chatroom repository: %w", err)
	}

	messageRepo, err := postgres.NewMessageRepository(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create message repository: %w", err)
	}

	// Create services
	authService := service.NewAuthService(userRepo, sessionRepo)
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	// Create WebSocket hub
	testHub = websocket.NewHub()
	hubCtx, hubCancel := context.WithCancel(context.Background())
	go testHub.Run(hubCtx)

	// Create and start response consumer
	responseConsumer = messaging.NewResponseConsumer(rmq, testHub, chatService, "test-bot-001")
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	err = responseConsumer.Start(consumerCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to start response consumer: %w", err)
	}

	// Create handlers
	authHandler := handler.NewAuthHandler(authService)
	chatroomHandler := handler.NewChatroomHandler(chatService, testHub)
	wsHandler := handler.NewWebSocketHandler(
		testHub,
		chatService,
		authService,
		rmq,
		sessionRepo,
		"*", // Allow all origins for testing
	)

	// Setup router
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.CORS([]string{"*"}))

	// Health endpoints (public)
	r.Get("/health", handler.Health)
	r.Get("/health/ready", handler.Ready(db, rmq))

	// Auth routes (public)
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)

		// Protected auth routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(sessionRepo))
			r.Get("/me", authHandler.Me)
			r.Post("/logout", authHandler.Logout)
		})
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(sessionRepo))

		// Chatroom routes
		r.Route("/api/v1/chatrooms", func(r chi.Router) {
			r.Get("/", chatroomHandler.List)
			r.Post("/", chatroomHandler.Create)
			r.Post("/{id}/join", chatroomHandler.Join)
			r.Get("/{id}/messages", chatroomHandler.GetMessages)
		})
	})

	// WebSocket route
	r.Get("/ws/chat/{chatroom_id}", wsHandler.HandleConnection)

	// Find an available port
	testPort := 18080
	baseURL = fmt.Sprintf("http://localhost:%d", testPort)
	wsURL = fmt.Sprintf("ws://localhost:%d", testPort)

	// Start server
	testServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", testPort),
		Handler: r,
	}

	go func() {
		if err := testServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	// Wait for server to be ready
	time.Sleep(500 * time.Millisecond)

	// Verify server is running with improved error logging
	maxRetries := 20
	for i := 0; i < maxRetries; i++ {
		resp, err := http.Get(baseURL + "/health")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			log.Printf("server started successfully after %d retries", i)
			break
		}
		if err != nil {
			log.Printf("health check attempt %d failed: %v", i+1, err)
		} else {
			log.Printf("health check attempt %d failed with status %d", i+1, resp.StatusCode)
			resp.Body.Close()
		}
		if i == maxRetries-1 {
			return nil, fmt.Errorf("server did not start in time after %d attempts", maxRetries)
		}
		time.Sleep(500 * time.Millisecond)
	}

	cleanup := func() {
		consumerCancel()
		hubCancel()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		testServer.Shutdown(ctx)
	}

	return cleanup, nil
}

// getConfig returns a test configuration
func getConfig() *config.Config {
	return &config.Config{
		Environment:    "test",
		SessionSecret:  "test-secret-key-32-characters-long!",
		AllowedOrigins: "*",
	}
}
