package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jobsity-chat/internal/config"
	"jobsity-chat/internal/domain"
	"jobsity-chat/internal/handler"
	"jobsity-chat/internal/messaging"
	"jobsity-chat/internal/middleware"
	"jobsity-chat/internal/observability"
	"jobsity-chat/internal/repository/postgres"
	"jobsity-chat/internal/service"
	"jobsity-chat/internal/websocket"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.Load()

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logFormat := os.Getenv("LOG_FORMAT")
	if logFormat == "" {
		logFormat = "json"
	}
	observability.InitLogger(logLevel, logFormat)

	slog.Info("starting chat server")

	connCtx, connCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connCancel()

	db, err := config.NewPostgresConnection(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	if err := db.PingContext(connCtx); err != nil {
		slog.Error("database ping failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("connected to postgresql")

	rmqCtx, rmqCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer rmqCancel()

	rmq, err := messaging.NewRabbitMQWithRetry(rmqCtx, cfg.RabbitMQURL)
	if err != nil {
		slog.Error("failed to connect to rabbitmq", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer rmq.Close()

	userRepo := postgres.NewUserRepository(db)
	sessionRepo := postgres.NewSessionRepository(db)
	messageRepo := postgres.NewMessageRepository(db)
	chatroomRepo := postgres.NewChatroomRepository(db)

	authService := service.NewAuthService(userRepo, sessionRepo)
	chatService := service.NewChatService(messageRepo, chatroomRepo)

	hub := websocket.NewHub()

	hubCtx, hubCancel := context.WithCancel(context.Background())
	defer hubCancel()
	go func() {
		if err := hub.Run(hubCtx); err != nil && err != context.Canceled {
			slog.Error("hub error", slog.String("error", err.Error()))
		}
	}()
	slog.Info("websocket hub started")

	botUserID := ensureBotUser(authService)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	responseConsumer := messaging.NewResponseConsumer(rmq, hub, chatService, botUserID)
	if err := responseConsumer.Start(ctx); err != nil {
		slog.Error("failed to start response consumer", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("response consumer started")

	go startSessionCleanup(ctx, sessionRepo)
	slog.Info("session cleanup task started")

	authHandler := handler.NewAuthHandler(authService)
	chatroomHandler := handler.NewChatroomHandler(chatService, hub)
	wsHandler := handler.NewWebSocketHandler(hub, chatService, authService, rmq, sessionRepo, cfg.AllowedOrigins)

	r := chi.NewRouter()

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.CORS(middleware.ParseOrigins(cfg.AllowedOrigins)))
	r.Use(middleware.Metrics())
	// r.Use(middleware.OpenAPIValidator(middleware.DefaultOpenAPIValidatorConfig()))

	r.Get("/health", handler.Health)
	r.Get("/health/ready", handler.Ready(db, rmq))
	r.Handle("/metrics", promhttp.Handler())

	r.Get("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/login.html")
	})
	r.Get("/register", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/register.html")
	})
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
	})

	r.Get("/login.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusMovedPermanently)
	})
	r.Get("/register.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/register", http.StatusMovedPermanently)
	})
	r.Get("/index.html", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	})

	// Block all other routes to prevent access to files we're not explicitly serving
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Not Found", http.StatusNotFound)
	})

	r.Route("/api/v1", func(r chi.Router) {
		authLimiter := middleware.NewRateLimiter(ctx, 5, 10)
		apiLimiter := middleware.NewRateLimiter(ctx, 20, 50)

		r.Group(func(r chi.Router) {
			r.Use(authLimiter.Middleware())
			r.Post("/auth/register", authHandler.Register)
			r.Post("/auth/login", authHandler.Login)
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(sessionRepo))
			r.Use(apiLimiter.Middleware())

			r.Get("/auth/me", authHandler.Me)
			r.Post("/auth/logout", authHandler.Logout)
			r.Get("/chatrooms", chatroomHandler.List)
			r.Post("/chatrooms", chatroomHandler.Create)
			r.Post("/chatrooms/{id}/join", chatroomHandler.Join)
			r.Get("/chatrooms/{id}/messages", chatroomHandler.GetMessages)
		})
	})

	// Auth handled internally to support query param tokens
	r.Get("/ws/chat/{chatroom_id}", wsHandler.HandleConnection)

	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("chat server listening", slog.String("port", cfg.Port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	slog.Info("shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", slog.String("error", err.Error()))
	}

	cancel()
	hubCancel()

	time.Sleep(100 * time.Millisecond)

	slog.Info("server stopped gracefully")
}

// ensureBotUser creates a bot user if it doesn't exist (idempotent)
func ensureBotUser(authService *service.AuthService) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	botUser, err := authService.Register(ctx, "StockBot", "bot@jobsity.com", "bot-password-not-used")

	switch {
	case err == nil:
		slog.Info("created bot user",
			slog.String("username", botUser.Username),
			slog.String("id", botUser.ID))
		return botUser.ID

	case errors.Is(err, domain.ErrUsernameExists):
		slog.Info("bot user already exists, fetching")
		botUser, err := authService.GetUserByUsername(ctx, "StockBot")
		if err != nil {
			slog.Error("bot user exists but cannot fetch",
				slog.String("error", err.Error()))
			panic("could not initialize stock bot user: " + err.Error())
		}
		slog.Info("using existing bot user",
			slog.String("username", botUser.Username),
			slog.String("id", botUser.ID))
		return botUser.ID

	default:
		slog.Error("failed to ensure bot user", slog.String("error", err.Error()))
		panic("could not initialize stock bot user: " + err.Error())
	}
}

// startSessionCleanup runs a background task to delete expired sessions
func startSessionCleanup(ctx context.Context, repo domain.SessionRepository) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("stopping session cleanup task")
			return
		case <-ticker.C:
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			count, err := repo.DeleteExpired(cleanupCtx)
			if err != nil {
				slog.Error("session cleanup failed", slog.String("error", err.Error()))
			} else {
				slog.Info("session cleanup completed",
					slog.Int64("sessions_deleted", count))
			}
			cancel()
		}
	}
}
