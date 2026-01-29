package config

import (
	"os"
	"testing"
)

func TestConfig_IsProduction(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		expected    bool
	}{
		{"production", "production", true},
		{"prod", "prod", true},
		{"development", "development", false},
		{"dev", "dev", false},
		{"staging", "staging", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Environment: tt.environment}
			if got := cfg.IsProduction(); got != tt.expected {
				t.Errorf("IsProduction() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_IsDevelopment(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		expected    bool
	}{
		{"development", "development", true},
		{"dev", "dev", true},
		{"empty", "", true},
		{"production", "production", false},
		{"prod", "prod", false},
		{"staging", "staging", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Environment: tt.environment}
			if got := cfg.IsDevelopment(); got != tt.expected {
				t.Errorf("IsDevelopment() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_Validate_Production(t *testing.T) {
	tests := []struct {
		name          string
		sessionSecret string
		wantError     bool
		errorContains string
	}{
		{
			name:          "valid_secret",
			sessionSecret: "this-is-a-very-secure-secret-with-32-plus-characters",
			wantError:     false,
		},
		{
			name:          "empty_secret",
			sessionSecret: "",
			wantError:     true,
			errorContains: "SESSION_SECRET must be set",
		},
		{
			name:          "default_secret",
			sessionSecret: "change-this-in-production",
			wantError:     true,
			errorContains: "SESSION_SECRET must be set",
		},
		{
			name:          "short_secret",
			sessionSecret: "short",
			wantError:     true,
			errorContains: "at least 32 characters",
		},
		{
			name:          "exactly_32_chars",
			sessionSecret: "12345678901234567890123456789012",
			wantError:     false,
		},
		{
			name:          "31_chars",
			sessionSecret: "1234567890123456789012345678901",
			wantError:     true,
			errorContains: "at least 32 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Environment:   "production",
				SessionSecret: tt.sessionSecret,
			}

			err := cfg.Validate()

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorContains != "" && !contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestConfig_Validate_Development(t *testing.T) {
	tests := []struct {
		name          string
		sessionSecret string
		wantError     bool
	}{
		{"empty_secret_gets_default", "", false},
		{"short_secret_allowed", "short", false},
		{"any_secret_allowed", "any-value", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Environment:   "development",
				SessionSecret: tt.sessionSecret,
			}

			err := cfg.Validate()

			if tt.wantError && err == nil {
				t.Error("Expected error, got nil")
			} else if !tt.wantError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			// Verify default was set if secret was empty
			if tt.sessionSecret == "" && cfg.SessionSecret == "" {
				t.Error("Expected default secret to be set for development")
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{"env_set", "TEST_KEY", "default", "custom", "custom"},
		{"env_not_set", "TEST_KEY_NOT_SET", "default", "", "default"},
		{"empty_default", "TEST_KEY_EMPTY", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable if provided
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.expected {
				t.Errorf("getEnv() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConfig_Validate_AllowedOrigins_Production_Warning(t *testing.T) {
	// This test verifies that a warning is logged for ALLOWED_ORIGINS in production
	// Since we can't easily capture log output in tests, we just verify no error
	cfg := &Config{
		Environment:    "production",
		SessionSecret:  "this-is-a-very-secure-secret-with-32-plus-characters",
		AllowedOrigins: "http://localhost:3000",
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for ALLOWED_ORIGINS warning, got %v", err)
	}
}

func TestConfig_Validate_Staging(t *testing.T) {
	cfg := &Config{
		Environment:   "staging",
		SessionSecret: "",
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Expected no error for staging environment, got %v", err)
	}

	// Verify default was set
	if cfg.SessionSecret == "" {
		t.Error("Expected default secret to be set for staging")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		   len(s) > len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
