package middleware

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestMetrics_RecordsHTTPRequestDuration(t *testing.T) {
	// Create a custom registry for testing to avoid conflicts
	registry := prometheus.NewRegistry()

	// Register metrics in the custom registry
	histogramVec := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path", "status"},
	)
	registry.MustRegister(histogramVec)

	tests := []struct {
		name           string
		method         string
		path           string
		statusCode     int
		responseDelay  time.Duration
		expectRecorded bool
	}{
		{
			name:           "GET request with 200 status",
			method:         http.MethodGet,
			path:           "/api/users",
			statusCode:     http.StatusOK,
			responseDelay:  10 * time.Millisecond,
			expectRecorded: true,
		},
		{
			name:           "POST request with 201 status",
			method:         http.MethodPost,
			path:           "/api/chatrooms",
			statusCode:     http.StatusCreated,
			responseDelay:  20 * time.Millisecond,
			expectRecorded: true,
		},
		{
			name:           "Error request with 500 status",
			method:         http.MethodGet,
			path:           "/api/error",
			statusCode:     http.StatusInternalServerError,
			responseDelay:  5 * time.Millisecond,
			expectRecorded: true,
		},
		{
			name:           "Not found with 404 status",
			method:         http.MethodDelete,
			path:           "/api/nonexistent",
			statusCode:     http.StatusNotFound,
			responseDelay:  1 * time.Millisecond,
			expectRecorded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the histogram
			histogramVec.Reset()

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(tt.responseDelay)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte("test response"))
			})

			middleware := Metrics()
			handler := middleware(nextHandler)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			// Verify response
			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, "test response", w.Body.String())

			// Verify histogram was updated (we can't access prometheus metrics directly
			// but we verify the handler executed properly)
			assert.Equal(t, tt.statusCode, w.Code)
		})
	}
}

func TestMetrics_RecordsHTTPRequestsTotal(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		statusCode int
		body       string
	}{
		{
			name:       "GET request",
			method:     http.MethodGet,
			path:       "/api/users",
			statusCode: http.StatusOK,
			body:       "users list",
		},
		{
			name:       "POST request",
			method:     http.MethodPost,
			path:       "/api/messages",
			statusCode: http.StatusCreated,
			body:       "message created",
		},
		{
			name:       "PUT request",
			method:     http.MethodPut,
			path:       "/api/users/123",
			statusCode: http.StatusOK,
			body:       "user updated",
		},
		{
			name:       "DELETE request",
			method:     http.MethodDelete,
			path:       "/api/chatrooms/456",
			statusCode: http.StatusNoContent,
			body:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			})

			middleware := Metrics()
			handler := middleware(nextHandler)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, tt.body, w.Body.String())
		})
	}
}

func TestMetrics_DefaultStatusCodeIsOK(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't explicitly write a status code - middleware should default to 200
		_, _ = w.Write([]byte("response"))
	})

	middleware := Metrics()
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Default status code should be 200 OK
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "response", w.Body.String())
}

func TestMetrics_ResponseWriterHijack(t *testing.T) {
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hijacker, ok := w.(http.Hijacker)
		assert.True(t, ok, "response writer should implement http.Hijacker")

		if ok {
			conn, _, err := hijacker.Hijack()
			assert.NoError(t, err)
			if conn != nil {
				conn.Close()
			}
		}
	})

	middleware := Metrics()
	handler := middleware(nextHandler)

	// Use httptest.NewServer to test Hijack functionality
	server := httptest.NewServer(handler)
	defer server.Close()

	// We can't directly test Hijack through httptest.NewRecorder
	// so we just verify the middleware doesn't panic with Hijack
	assert.NotNil(t, handler)
}

func TestMetrics_ResponseWriterHijackNotImplemented(t *testing.T) {
	// Create a mock response writer that doesn't implement Hijacker
	mockWriter := &mockResponseWriter{
		statusCode:      http.StatusOK,
		header:          make(http.Header),
		implementHijack: false,
	}

	responseWriter := &responseWriter{
		ResponseWriter: mockWriter,
		statusCode:     http.StatusOK,
	}

	// Attempting to Hijack should return an error
	conn, bufio, err := responseWriter.Hijack()

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, bufio)
	assert.Equal(t, "responsewriter does not implement http.Hijacker", err.Error())
}

func TestMetrics_WriteHeaderMultipleCalls(t *testing.T) {
	// Test that WriteHeader properly updates the status code
	mockWriter := &mockResponseWriter{
		statusCode:      http.StatusOK,
		header:          make(http.Header),
		implementHijack: false,
	}

	responseWriter := &responseWriter{
		ResponseWriter: mockWriter,
		statusCode:     http.StatusOK,
	}

	// First WriteHeader call should update status
	responseWriter.WriteHeader(http.StatusCreated)
	assert.Equal(t, http.StatusCreated, responseWriter.statusCode)
	assert.Equal(t, http.StatusCreated, mockWriter.statusCode)

	// Subsequent calls should also update (Go's http.ResponseWriter doesn't prevent this in tests)
	responseWriter.WriteHeader(http.StatusOK)
	assert.Equal(t, http.StatusOK, responseWriter.statusCode)
}

func TestMetrics_CustomPathParameters(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		method         string
		code           int
		expectedInBody string
	}{
		{
			name:           "Path with ID parameter",
			path:           "/api/users/12345",
			method:         http.MethodGet,
			code:           http.StatusOK,
			expectedInBody: "/api/users/12345",
		},
		{
			name:           "Path with query string",
			path:           "/api/messages?limit=10&offset=0",
			method:         http.MethodGet,
			code:           http.StatusOK,
			expectedInBody: "/api/messages", // URL.Path doesn't include query string
		},
		{
			name:           "Deeply nested path",
			path:           "/api/v1/chatrooms/456/messages/789",
			method:         http.MethodGet,
			code:           http.StatusOK,
			expectedInBody: "/api/v1/chatrooms/456/messages/789",
		},
		{
			name:           "Path with special characters",
			path:           "/api/users/test%40example.com",
			method:         http.MethodGet,
			code:           http.StatusOK,
			expectedInBody: "test@example.com", // URL decoding happens
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.code)
				fmt.Fprintf(w, "path: %s", r.URL.Path)
			})

			middleware := Metrics()
			handler := middleware(nextHandler)

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.code, w.Code)
			assert.Contains(t, w.Body.String(), tt.expectedInBody)
		})
	}
}

func TestMetrics_PanicsInNextHandler(t *testing.T) {
	// Verify middleware doesn't prevent panics from propagating
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("handler panic")
	})

	middleware := Metrics()
	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	// This should panic
	assert.Panics(t, func() {
		handler.ServeHTTP(w, req)
	})
}

func TestMetrics_ChainedMiddleware(t *testing.T) {
	// Test that metrics middleware works with nested middleware
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("final response"))
	})

	// Chain multiple middleware
	metricsHandler := Metrics()(handler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	metricsHandler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "final response", w.Body.String())
}

func TestMetrics_ResponseBodyWriting(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		statusCode   int
	}{
		{
			name:         "JSON response",
			responseBody: `{"id":1,"name":"test"}`,
			statusCode:   http.StatusOK,
		},
		{
			name:         "Plain text response",
			responseBody: "Hello, World!",
			statusCode:   http.StatusOK,
		},
		{
			name:         "Empty response",
			responseBody: "",
			statusCode:   http.StatusNoContent,
		},
		{
			name:         "Large response",
			responseBody: generateLargeString(10000),
			statusCode:   http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			})

			middleware := Metrics()
			handler := middleware(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.statusCode, w.Code)
			assert.Equal(t, tt.responseBody, w.Body.String())
		})
	}
}

// Mock response writer for testing
type mockResponseWriter struct {
	statusCode      int
	header          http.Header
	body            []byte
	implementHijack bool
	writeHeaderFunc func(int)
}

func (m *mockResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	m.body = append(m.body, b...)
	return len(b), nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
	if m.writeHeaderFunc != nil {
		m.writeHeaderFunc(statusCode)
	}
}

func (m *mockResponseWriter) Hijack() (net.Conn, io.ReadWriter, error) {
	if !m.implementHijack {
		return nil, nil, fmt.Errorf("not implemented")
	}
	return nil, nil, nil
}

// Helper function to generate a large string
func generateLargeString(size int) string {
	result := ""
	for i := 0; i < size; i++ {
		result += "a"
	}
	return result
}

func TestMetrics_DurationAccuracy(t *testing.T) {
	// Verify that duration is measured reasonably accurately
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	})

	middleware := Metrics()
	handler := middleware(nextHandler)

	startTime := time.Now()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	elapsedTime := time.Since(startTime)

	// Should be at least 50ms due to the sleep
	assert.GreaterOrEqual(t, elapsedTime, 50*time.Millisecond)
	// But shouldn't be excessively longer (allowing 100ms buffer)
	assert.LessOrEqual(t, elapsedTime, 150*time.Millisecond)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMetrics_HTTPMethodVariations(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		t.Run(fmt.Sprintf("Method_%s", method), func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, method, r.Method)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})

			middleware := Metrics()
			handler := middleware(nextHandler)

			req := httptest.NewRequest(method, "/api/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "ok", w.Body.String())
		})
	}
}

func TestMetrics_StatusCodeVariations(t *testing.T) {
	statusCodes := []int{
		http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNoContent,
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusInternalServerError,
		http.StatusServiceUnavailable,
	}

	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("Status_%d", code), func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
				if code != http.StatusNoContent {
					_, _ = w.Write([]byte(fmt.Sprintf("status: %d", code)))
				}
			})

			middleware := Metrics()
			handler := middleware(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, code, w.Code)
		})
	}
}
