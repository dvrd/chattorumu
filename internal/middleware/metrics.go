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

func Metrics() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(ww, r)

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

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("responsewriter does not implement http.Hijacker")
	}
	return hijacker.Hijack()
}
