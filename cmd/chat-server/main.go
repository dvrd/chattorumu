package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jobsity-chat/internal/config"
	"jobsity-chat/internal/handler"
	"jobsity-chat/internal/messaging"
	"jobsity-chat/internal/middleware"
	"jobsity-chat/internal/repository/postgres"
	"jobsity-chat/internal/service"
	"jobsity-chat/internal/websocket"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	log.Println("Starting Chat Server...")

	// Load configuration
	cfg := config.Load()

	// Connect to database
	db, err := config.NewPostgresConnection(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	// Connect to RabbitMQ
	rmq, err := messaging.NewRabbitMQ(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rmq.Close()
	log.Println("Connected to RabbitMQ")

	// Initialize repositories
	userRepo := postgres.NewUserRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	messageRepo := postgres.NewMessageRepository(db)
	chatroomRepo := postgres.NewChatroomRepository(db)

	// Initialize services
	authService := service.NewAuthService(userRepo, sessionRepo)
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()
	log.Println("WebSocket Hub started")

	// Get or create bot user
	botUserID := ensureBotUser(authService)

	// Start response consumer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	responseConsumer := messaging.NewResponseConsumer(rmq, hub, chatService, botUserID)
	if err := responseConsumer.Start(ctx); err != nil {
		log.Fatalf("Failed to start response consumer: %v", err)
	}
	log.Println("Response consumer started")

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authService)
	chatroomHandler := handler.NewChatroomHandler(chatService)
	wsHandler := handler.NewWebSocketHandler(hub, chatService, authService, rmq)

	// Setup router
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.CORS(middleware.ParseOrigins(cfg.AllowedOrigins)))

	// Health checks
	r.Get("/health", handler.Health)
	r.Get("/health/ready", handler.Ready(db))

	// Serve static files
	r.Handle("/*", http.FileServer(http.Dir("./static")))

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public auth routes
		r.Post("/auth/register", authHandler.Register)
		r.Post("/auth/login", authHandler.Login)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(sessionRepo))

			r.Post("/auth/logout", authHandler.Logout)
			r.Get("/chatrooms", chatroomHandler.List)
			r.Post("/chatrooms", chatroomHandler.Create)
			r.Post("/chatrooms/{id}/join", chatroomHandler.Join)
			r.Get("/chatrooms/{id}/messages", chatroomHandler.GetMessages)
		})
	})

	// WebSocket routes (protected)
	r.Group(func(r chi.Router) {
		r.Use(middleware.Auth(sessionRepo))
		r.Get("/ws/chat/{chatroom_id}", wsHandler.HandleConnection)
	})

	// Setup HTTP server
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Chat Server listening on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down server...")

	// Shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	cancel() // Cancel background tasks
	log.Println("Server stopped gracefully")
}

// ensureBotUser creates a bot user if it doesn't exist
func ensureBotUser(authService *service.AuthService) string {
	ctx := context.Background()

	// Try to get existing bot user
	botUser, err := authService.GetUserByID(ctx, "00000000-0000-0000-0000-000000000001")
	if err == nil {
		log.Printf("Bot user already exists: %s", botUser.Username)
		return botUser.ID
	}

	// Create bot user
	botUser, err = authService.Register(ctx, "StockBot", "bot@jobsity.com", "bot-password-not-used")
	if err != nil {
		log.Printf("Warning: Failed to create bot user: %v", err)
		return "00000000-0000-0000-0000-000000000001" // Fallback to hardcoded ID
	}

	log.Printf("Created bot user: %s (ID: %s)", botUser.Username, botUser.ID)
	return botUser.ID
}
