package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	Port           string
	DatabaseURL    string
	RabbitMQURL    string
	SessionSecret  string
	StooqAPIURL    string
	AllowedOrigins string
	Environment    string // development, staging, production
}

// Load loads configuration from environment variables and validates for production
func Load() *Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		DatabaseURL:    getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/jobsity_chat?sslmode=disable"),
		RabbitMQURL:    getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		SessionSecret:  getEnv("SESSION_SECRET", ""),
		StooqAPIURL:    getEnv("STOOQ_API_URL", "https://stooq.com"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:8080"),
		Environment:    getEnv("ENVIRONMENT", "development"),
	}

	// Validate production configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	return cfg
}

// Validate checks configuration for security and correctness
func (c *Config) Validate() error {
	// Production environment requires strong secrets
	if c.IsProduction() {
		if c.SessionSecret == "" || c.SessionSecret == "change-this-in-production" {
			return fmt.Errorf("SESSION_SECRET must be set to a strong random value in production")
		}

		if len(c.SessionSecret) < 32 {
			return fmt.Errorf("SESSION_SECRET must be at least 32 characters in production (got %d)", len(c.SessionSecret))
		}

		// Warn about non-HTTPS origins in production
		if c.AllowedOrigins != "" {
			log.Println("WARNING: Ensure ALLOWED_ORIGINS uses HTTPS in production")
		}
	} else if c.SessionSecret == "" {
		// Development/staging: provide default if not set
		c.SessionSecret = "dev-secret-not-for-production"
		log.Println("Using default SESSION_SECRET for development")
	}

	return nil
}

// IsProduction returns true if running in production environment
func (c *Config) IsProduction() bool {
	return c.Environment == "production" || c.Environment == "prod"
}

// IsDevelopment returns true if running in development environment
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development" || c.Environment == "dev" || c.Environment == ""
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
