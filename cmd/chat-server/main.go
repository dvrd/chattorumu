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

	// Start hub with context
	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()
	go func() {
		if err := hub.Run(hubCtx); err != nil && err != context.Canceled {
			log.Printf("Hub error: %v", err)
		}
	}()
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

	// Clean URL routes for auth pages
	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/login.html")
	})
	r.Get("/register", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/register.html")
	})
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	// Redirect legacy .html URLs to clean URLs
	r.Get("/login.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusMovedPermanently)
	})
	r.Get("/register.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/register", http.StatusMovedPermanently)
	})
	r.Get("/index.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	})

	// Block all other routes - no static file server
	// This prevents access to any files we're not explicitly serving
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Rate limit auth routes to prevent brute force attacks
		authLimiter := middleware.NewRateLimiter(5, 10) // 5 req/sec, burst 10

		// Public auth routes with rate limiting
		r.Group(func(r chi.Router) {
			r.Use(authLimiter.Middleware())
			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
		})

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

	// Stop accepting new HTTP connections
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// Cancel background tasks (response consumer)
	cancel()

	// Stop WebSocket hub
	hubCancel()

	// Give hub time to close connections gracefully
	time.Sleep(100 * time.Millisecond)

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
