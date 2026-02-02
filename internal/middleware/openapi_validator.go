package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// OpenAPIValidatorConfig holds configuration for OpenAPI validation middleware
type OpenAPIValidatorConfig struct {
	// Enabled controls whether validation is active
	Enabled bool
	// SpecPath is the path to the OpenAPI specification file
	SpecPath string
	// ValidateRequests enables request validation
	ValidateRequests bool
	// ValidateResponses enables response validation (impacts performance)
	ValidateResponses bool
	// SkipPaths are paths to skip validation (e.g., /health, /metrics, static files)
	SkipPaths []string
}

// DefaultOpenAPIValidatorConfig returns a sensible default configuration
func DefaultOpenAPIValidatorConfig() *OpenAPIValidatorConfig {
	// Enable validation in development, disable in production by default
	env := os.Getenv("ENVIRONMENT")
	isDev := env != "production" && env != "prod"

	return &OpenAPIValidatorConfig{
		Enabled:           isDev,
		SpecPath:          "artifacts/openapi.yaml",
		ValidateRequests:  true,
		ValidateResponses: false, // Disabled by default for performance
		SkipPaths: []string{
			"/health",
			"/health/ready",
			"/metrics",
			"/login",
			"/register",
			"/",
			"/login.html",
			"/register.html",
			"/index.html",
		},
	}
}

// OpenAPIValidator creates a middleware that validates HTTP requests and responses
// against an OpenAPI 3.0 specification
func OpenAPIValidator(config *OpenAPIValidatorConfig) func(next http.Handler) http.Handler {
	if config == nil {
		config = DefaultOpenAPIValidatorConfig()
	}

	// If validation is disabled, return a no-op middleware
	if !config.Enabled {
		slog.Info("OpenAPI validation disabled")
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Load OpenAPI specification
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(config.SpecPath)
	if err != nil {
		slog.Error("failed to load OpenAPI spec",
			slog.String("path", config.SpecPath),
			slog.String("error", err.Error()))
		// Return no-op middleware on error to avoid breaking the app
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Validate the OpenAPI document itself
	if validErr := doc.Validate(loader.Context); validErr != nil {
		slog.Error("OpenAPI spec validation failed", slog.String("error", validErr.Error()))
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	// Create router for matching requests to operations
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		slog.Error("failed to create OpenAPI router", slog.String("error", err.Error()))
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	slog.Info("OpenAPI validation enabled",
		slog.Bool("validate_requests", config.ValidateRequests),
		slog.Bool("validate_responses", config.ValidateResponses),
		slog.String("spec_path", config.SpecPath))

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip validation for certain paths
			if shouldSkipPath(r.URL.Path, config.SkipPaths) {
				next.ServeHTTP(w, r)
				return
			}

			// Find the route that matches the request
			route, pathParams, err := router.FindRoute(r)
			if err != nil {
				// Route not found in OpenAPI spec
				if config.ValidateRequests {
					slog.Warn("request path not found in OpenAPI spec",
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path))
					writeValidationError(w, fmt.Sprintf("Path not found in OpenAPI spec: %s %s", r.Method, r.URL.Path))
					return
				}
				// If validation is disabled, just pass through
				next.ServeHTTP(w, r)
				return
			}

			// Validate request if enabled
			if config.ValidateRequests {
				requestValidationInput := &openapi3filter.RequestValidationInput{
					Request:    r,
					PathParams: pathParams,
					Route:      route,
					Options: &openapi3filter.Options{
						AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
					},
				}

				if err := openapi3filter.ValidateRequest(context.Background(), requestValidationInput); err != nil {
					slog.Warn("request validation failed",
						slog.String("method", r.Method),
						slog.String("path", r.URL.Path),
						slog.String("error", err.Error()))
					writeValidationError(w, fmt.Sprintf("Request validation failed: %s", err.Error()))
					return
				}

				slog.Debug("request validated successfully",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path))
			}

			// If response validation is disabled, just call the next handler
			if !config.ValidateResponses {
				next.ServeHTTP(w, r)
				return
			}

			// Wrap response writer to capture status and body for validation
			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default status
				body:           []byte{},
			}

			// Call the next handler
			next.ServeHTTP(recorder, r)

			// Validate response
			responseValidationInput := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: &openapi3filter.RequestValidationInput{
					Request:    r,
					PathParams: pathParams,
					Route:      route,
				},
				Status: recorder.statusCode,
				Header: recorder.Header(),
				Body:   io.NopCloser(bytes.NewReader(recorder.body)),
				Options: &openapi3filter.Options{
					AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
				},
			}

			if err := openapi3filter.ValidateResponse(context.Background(), responseValidationInput); err != nil {
				slog.Warn("response validation failed",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", recorder.statusCode),
					slog.String("error", err.Error()))
				// Note: We don't return error to client here since response is already sent
				// This is logged for debugging purposes
			} else {
				slog.Debug("response validated successfully",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", recorder.statusCode))
			}
		})
	}
}

// shouldSkipPath checks if a path should skip validation
func shouldSkipPath(path string, skipPaths []string) bool {
	for _, skipPath := range skipPaths {
		if strings.HasPrefix(path, skipPath) || path == skipPath {
			return true
		}
	}
	return false
}

// writeValidationError writes a JSON error response
func writeValidationError(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// responseRecorder wraps http.ResponseWriter to capture response data
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       []byte
}

// WriteHeader captures the status code
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body
func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body = append(r.body, b...)
	return r.ResponseWriter.Write(b)
}
