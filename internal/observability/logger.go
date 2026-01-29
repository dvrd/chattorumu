package observability

import (
	"context"
	"log/slog"
	"os"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	userIDKey    contextKey = "user_id"
)

var logger *slog.Logger

// InitLogger initializes the global structured logger
func InitLogger(level, format string) {
	var handler slog.Handler

	opts := &slog.HandlerOptions{
		Level:     parseLevel(level),
		AddSource: level == "debug",
	}

	if format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger = slog.New(handler)
	slog.SetDefault(logger)
}

// FromContext returns a logger with context values attached
func FromContext(ctx context.Context) *slog.Logger {
	if logger == nil {
		// Fallback to default logger if not initialized
		return slog.Default()
	}

	attrs := make([]any, 0, 4)

	if reqID, ok := ctx.Value(requestIDKey).(string); ok && reqID != "" {
		attrs = append(attrs, slog.String("request_id", reqID))
	}

	if userID, ok := ctx.Value(userIDKey).(string); ok && userID != "" {
		attrs = append(attrs, slog.String("user_id", userID))
	}

	if len(attrs) > 0 {
		return logger.With(attrs...)
	}
	return logger
}

// WithRequestID adds request ID to context
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey, requestID)
}

// WithUserID adds user ID to context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// parseLevel converts string level to slog.Level
func parseLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Info logs at info level
func Info(msg string, args ...any) {
	if logger != nil {
		logger.Info(msg, args...)
	} else {
		slog.Info(msg, args...)
	}
}

// Error logs at error level
func Error(msg string, args ...any) {
	if logger != nil {
		logger.Error(msg, args...)
	} else {
		slog.Error(msg, args...)
	}
}

// Warn logs at warn level
func Warn(msg string, args ...any) {
	if logger != nil {
		logger.Warn(msg, args...)
	} else {
		slog.Warn(msg, args...)
	}
}

// Debug logs at debug level
func Debug(msg string, args ...any) {
	if logger != nil {
		logger.Debug(msg, args...)
	} else {
		slog.Debug(msg, args...)
	}
}
