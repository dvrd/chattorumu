package middleware

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"jobsity-chat/internal/observability"
)

// Metrics returns a middleware that records HTTP metrics
func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap ResponseWriter to capture status code
			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Call next handler
			next.ServeHTTP(ww, r)

			// Record metrics
			duration := time.Since(start).Seconds()
			status := strconv.Itoa(ww.statusCode)

			observability.HTTPRequestDuration.WithLabelValues(
				r.Method,
				r.URL.Path,
				status,
			).Observe(duration)

			observability.HTTPRequestsTotal.WithLabelValues(
				r.Method,
				r.URL.Path,
				status,
			).Inc()
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

// Hijack implements http.Hijacker interface for WebSocket support
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("responsewriter does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}
