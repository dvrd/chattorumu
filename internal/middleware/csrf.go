package middleware

import (
	"crypto/hmac"
	"log/slog"
	"net/http"
	"strings"

	"jobsity-chat/internal/domain"
)

// CSRF middleware validates CSRF tokens for state-changing requests.
// It implements the Synchronizer Token Pattern using server-side session storage.
//
// Token Validation Flow:
// 1. Skip for safe HTTP methods (GET, HEAD, OPTIONS)
// 2. Skip for endpoints that don't require CSRF protection (health, metrics, websocket)
// 3. Extract CSRF token from request (form data or headers)
// 4. Retrieve session using session cookie
// 5. Verify token against session.CSRFToken using constant-time comparison
// 6. Log security events on validation failure
// 7. Reject with 403 Forbidden if invalid
//
// Token sources (checked in order):
// - Form field: csrf_token
// - Header: X-CSRF-Token
// - Header: X-XSRF-Token (alternate)
func CSRF(sessionRepo domain.SessionRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip CSRF validation for safe methods
			if isSafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Skip CSRF validation for exempt endpoints
			if isExemptPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Extract session from context (set by Auth middleware)
			session, ok := GetSession(r.Context())
			if !ok {
				// No session in context means user is not authenticated
				// Auth middleware should have caught this, but return 401 to be safe
				http.Error(w, `{"error":"Not authenticated"}`, http.StatusUnauthorized)
				return
			}

			// Extract CSRF token from request
			submittedToken := extractCSRFToken(r)

			// Validate token presence
			if submittedToken == "" {
				logCSRFFailure(r, session.UserID, "missing token")
				http.Error(w, `{"error":"Forbidden"}`, http.StatusForbidden)
				return
			}

			// Validate token using constant-time comparison
			if !hmac.Equal([]byte(session.CSRFToken), []byte(submittedToken)) {
				logCSRFFailure(r, session.UserID, "invalid token")
				http.Error(w, `{"error":"Forbidden"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// isSafeMethod returns true if the HTTP method is idempotent and cacheable.
// These methods should not modify state and don't require CSRF tokens.
func isSafeMethod(method string) bool {
	return method == http.MethodGet ||
		method == http.MethodHead ||
		method == http.MethodOptions
}

// isExemptPath returns true if the request path should skip CSRF validation.
// Exempted paths include health checks, metrics, and websocket upgrades.
func isExemptPath(path string) bool {
	exemptPaths := []string{
		"/health",
		"/metrics",
		"/ws/",
	}

	for _, exemptPath := range exemptPaths {
		if strings.HasPrefix(path, exemptPath) {
			return true
		}
	}
	return false
}

// extractCSRFToken extracts the CSRF token from the request.
// Checks sources in order: form data, X-CSRF-Token header, X-XSRF-Token header.
func extractCSRFToken(r *http.Request) string {
	// Check form data (for traditional HTML form submissions)
	token := r.FormValue("csrf_token")
	if token != "" {
		return token
	}

	// Check X-CSRF-Token header (for AJAX/API requests)
	token = r.Header.Get("X-CSRF-Token")
	if token != "" {
		return token
	}

	// Check X-XSRF-Token header (alternate header name)
	token = r.Header.Get("X-XSRF-Token")
	return token
}

// logCSRFFailure logs a security event when CSRF validation fails.
// Useful for monitoring and detecting potential CSRF attacks.
func logCSRFFailure(r *http.Request, userID, reason string) {
	slog.Warn("CSRF validation failed",
		slog.String("user_id", userID),
		slog.String("reason", reason),
		slog.String("method", r.Method),
		slog.String("path", r.RequestURI),
		slog.String("remote_addr", r.RemoteAddr),
	)
}
