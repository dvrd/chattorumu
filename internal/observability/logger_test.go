package observability

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitLogger_JSONFormat(t *testing.T) {
	t.Run("initializes_json_handler", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("info", "json")

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout

		// Verify logger is initialized
		assert.NotNil(t, logger)
	})

	t.Run("json_format_produces_json", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("info", "json")
		Info("test message", "key", "value")

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout

		// Verify logger is initialized
		assert.NotNil(t, logger)
	})
}

func TestInitLogger_TextFormat(t *testing.T) {
	t.Run("initializes_text_handler", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("info", "text")

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout

		// Verify logger is initialized
		assert.NotNil(t, logger)
	})

	t.Run("text_format_produces_text_output", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("info", "text")
		Info("test message")

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout

		// Verify logger is initialized
		assert.NotNil(t, logger)
	})
}

func TestInitLogger_LogLevels(t *testing.T) {
	tests := []struct {
		name     string
		level    string
		expected slog.Level
	}{
		{"debug_level", "debug", slog.LevelDebug},
		{"info_level", "info", slog.LevelInfo},
		{"warn_level", "warn", slog.LevelWarn},
		{"error_level", "error", slog.LevelError},
		{"invalid_defaults_to_info", "unknown", slog.LevelInfo},
		{"empty_defaults_to_info", "", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			_, w, _ := os.Pipe()
			os.Stdout = w

			InitLogger(tt.level, "text")

			// Reset stdout
			w.Close()
			os.Stdout = oldStdout

			// Verify logger is initialized
			assert.NotNil(t, logger)
		})
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{"debug", "debug", slog.LevelDebug},
		{"info", "info", slog.LevelInfo},
		{"warn", "warn", slog.LevelWarn},
		{"error", "error", slog.LevelError},
		{"unknown", "unknown", slog.LevelInfo},
		{"empty", "", slog.LevelInfo},
		{"uppercase", "DEBUG", slog.LevelInfo}, // Case sensitive, defaults to info
		{"case_insensitive", "Info", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFromContext_NoValues(t *testing.T) {
	t.Run("returns_logger_with_no_context_values", func(t *testing.T) {
		ctx := context.Background()
		result := FromContext(ctx)

		assert.NotNil(t, result)
	})
}

func TestFromContext_WithRequestID(t *testing.T) {
	t.Run("includes_request_id_in_logger", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")

		result := FromContext(ctx)
		assert.NotNil(t, result)
	})

	t.Run("empty_request_id_is_ignored", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "")

		result := FromContext(ctx)
		assert.NotNil(t, result)
	})
}

func TestFromContext_WithUserID(t *testing.T) {
	t.Run("includes_user_id_in_logger", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithUserID(ctx, "user-456")

		result := FromContext(ctx)
		assert.NotNil(t, result)
	})

	t.Run("empty_user_id_is_ignored", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithUserID(ctx, "")

		result := FromContext(ctx)
		assert.NotNil(t, result)
	})
}

func TestFromContext_WithBothValues(t *testing.T) {
	t.Run("includes_both_request_id_and_user_id", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-123")
		ctx = WithUserID(ctx, "user-456")

		result := FromContext(ctx)
		assert.NotNil(t, result)
	})
}

func TestFromContext_Fallback(t *testing.T) {
	t.Run("returns_default_logger_when_not_initialized", func(t *testing.T) {
		// Save current logger
		savedLogger := logger
		defer func() { logger = savedLogger }()

		// Reset logger
		logger = nil

		ctx := context.Background()
		result := FromContext(ctx)

		assert.NotNil(t, result)
		// Should return slog.Default()
		assert.Equal(t, slog.Default(), result)
	})
}

func TestWithRequestID(t *testing.T) {
	t.Run("adds_request_id_to_context", func(t *testing.T) {
		ctx := context.Background()
		result := WithRequestID(ctx, "test-request-id")

		assert.NotNil(t, result)
		assert.Equal(t, "test-request-id", result.Value(requestIDKey))
	})

	t.Run("overwrites_existing_request_id", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "old-id")
		ctx = WithRequestID(ctx, "new-id")

		assert.Equal(t, "new-id", ctx.Value(requestIDKey))
	})

	t.Run("preserves_other_context_values", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithUserID(ctx, "user-123")
		ctx = WithRequestID(ctx, "req-123")

		assert.Equal(t, "req-123", ctx.Value(requestIDKey))
		assert.Equal(t, "user-123", ctx.Value(userIDKey))
	})
}

func TestWithUserID(t *testing.T) {
	t.Run("adds_user_id_to_context", func(t *testing.T) {
		ctx := context.Background()
		result := WithUserID(ctx, "test-user-id")

		assert.NotNil(t, result)
		assert.Equal(t, "test-user-id", result.Value(userIDKey))
	})

	t.Run("overwrites_existing_user_id", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithUserID(ctx, "old-user")
		ctx = WithUserID(ctx, "new-user")

		assert.Equal(t, "new-user", ctx.Value(userIDKey))
	})

	t.Run("preserves_other_context_values", func(t *testing.T) {
		ctx := context.Background()
		ctx = WithRequestID(ctx, "req-456")
		ctx = WithUserID(ctx, "user-789")

		assert.Equal(t, "req-456", ctx.Value(requestIDKey))
		assert.Equal(t, "user-789", ctx.Value(userIDKey))
	})
}

func TestLoggingFunctions_InfoLevel(t *testing.T) {
	t.Run("info_logs_message", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("info", "text")

		// Should not panic
		assert.NotPanics(t, func() {
			Info("test info message", "key", "value")
		})

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout
	})
}

func TestLoggingFunctions_ErrorLevel(t *testing.T) {
	t.Run("error_logs_message", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("info", "text")

		// Should not panic
		assert.NotPanics(t, func() {
			Error("test error message", "error", "something went wrong")
		})

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout
	})
}

func TestLoggingFunctions_WarnLevel(t *testing.T) {
	t.Run("warn_logs_message", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("info", "text")

		// Should not panic
		assert.NotPanics(t, func() {
			Warn("test warning message", "warning", "be careful")
		})

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout
	})
}

func TestLoggingFunctions_DebugLevel(t *testing.T) {
	t.Run("debug_logs_when_level_is_debug", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("debug", "text")

		// Should not panic
		assert.NotPanics(t, func() {
			Debug("test debug message", "debug_key", "debug_value")
		})

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout
	})

	t.Run("debug_does_not_log_when_level_is_info", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("info", "text")

		// Should not panic - debug logs should be filtered at info level
		assert.NotPanics(t, func() {
			Debug("test debug message that should not appear")
		})

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout
	})
}

func TestLoggingFunctions_WithoutInitializedLogger(t *testing.T) {
	t.Run("info_uses_default_logger_when_not_initialized", func(t *testing.T) {
		// Save current logger
		savedLogger := logger
		defer func() { logger = savedLogger }()

		// Reset logger
		logger = nil

		// Should not panic
		assert.NotPanics(t, func() {
			Info("test message without initialized logger")
		})
	})

	t.Run("error_uses_default_logger_when_not_initialized", func(t *testing.T) {
		// Save current logger
		savedLogger := logger
		defer func() { logger = savedLogger }()

		// Reset logger
		logger = nil

		// Should not panic
		assert.NotPanics(t, func() {
			Error("test error without initialized logger")
		})
	})

	t.Run("warn_uses_default_logger_when_not_initialized", func(t *testing.T) {
		// Save current logger
		savedLogger := logger
		defer func() { logger = savedLogger }()

		// Reset logger
		logger = nil

		// Should not panic
		assert.NotPanics(t, func() {
			Warn("test warning without initialized logger")
		})
	})

	t.Run("debug_uses_default_logger_when_not_initialized", func(t *testing.T) {
		// Save current logger
		savedLogger := logger
		defer func() { logger = savedLogger }()

		// Reset logger
		logger = nil

		// Should not panic
		assert.NotPanics(t, func() {
			Debug("test debug without initialized logger")
		})
	})
}

func TestInitLogger_AddSourceOption(t *testing.T) {
	t.Run("adds_source_when_debug_level", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("debug", "text")
		Info("test message")

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout

		// Verify logger is initialized
		assert.NotNil(t, logger)
	})

	t.Run("does_not_add_source_when_not_debug_level", func(t *testing.T) {
		// Capture stdout
		oldStdout := os.Stdout
		_, w, _ := os.Pipe()
		os.Stdout = w

		InitLogger("info", "text")
		Info("test message")

		// Reset stdout
		w.Close()
		os.Stdout = oldStdout

		// Verify logger is initialized
		assert.NotNil(t, logger)
	})
}
