package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"jobsity-chat/internal/testutil"
)

func TestCORS_AllowedOrigin(t *testing.T) {
	tests := []struct {
		name           string
		allowedOrigins []string
		requestOrigin  string
		shouldAllow    bool
	}{
		{
			name:           "allowed origin",
			allowedOrigins: []string{"http://localhost:3000", "http://example.com"},
			requestOrigin:  "http://localhost:3000",
			shouldAllow:    true,
		},
		{
			name:           "allowed second origin",
			allowedOrigins: []string{"http://localhost:3000", "http://example.com"},
			requestOrigin:  "http://example.com",
			shouldAllow:    true,
		},
		{
			name:           "disallowed origin",
			allowedOrigins: []string{"http://localhost:3000"},
			requestOrigin:  "http://malicious.com",
			shouldAllow:    false,
		},
		{
			name:           "empty origin allowed",
			allowedOrigins: []string{"http://localhost:3000"},
			requestOrigin:  "",
			shouldAllow:    false, // Headers won't be set for empty origin
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := CORS(tt.allowedOrigins)

			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			accessControlHeader := w.Header().Get("Access-Control-Allow-Origin")
			if tt.shouldAllow {
				testutil.AssertEqual(t, accessControlHeader, tt.requestOrigin)
			} else {
				testutil.AssertEqual(t, accessControlHeader, "")
			}
		})
	}
}

func TestCORS_WildcardOrigin(t *testing.T) {
	middleware := CORS([]string{"*"})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	origins := []string{
		"http://localhost:3000",
		"http://example.com",
		"http://any-origin.test",
		"https://secure.site.com",
	}

	for _, origin := range origins {
		t.Run(origin, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
			req.Header.Set("Origin", origin)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			accessControlHeader := w.Header().Get("Access-Control-Allow-Origin")
			testutil.AssertEqual(t, accessControlHeader, origin)
		})
	}
}

func TestCORS_PreflightRequest(t *testing.T) {
	middleware := CORS([]string{"http://localhost:3000"})

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Preflight should return 200 OK
	testutil.AssertStatusCode(t, w, http.StatusOK)

	// Preflight should NOT call the next handler
	testutil.AssertFalse(t, nextHandlerCalled, "preflight should not call next handler")

	// Should set CORS headers
	testutil.AssertEqual(t, w.Header().Get("Access-Control-Allow-Origin"), "http://localhost:3000")
	testutil.AssertEqual(t, w.Header().Get("Access-Control-Allow-Credentials"), "true")
}

func TestCORS_CredentialsHeader(t *testing.T) {
	middleware := CORS([]string{"http://localhost:3000"})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	testutil.AssertEqual(t, w.Header().Get("Access-Control-Allow-Credentials"), "true")
}

func TestCORS_MethodsHeader(t *testing.T) {
	middleware := CORS([]string{"http://localhost:3000"})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	methodsHeader := w.Header().Get("Access-Control-Allow-Methods")
	testutil.AssertContains(t, methodsHeader, "GET")
	testutil.AssertContains(t, methodsHeader, "POST")
	testutil.AssertContains(t, methodsHeader, "PUT")
	testutil.AssertContains(t, methodsHeader, "DELETE")
	testutil.AssertContains(t, methodsHeader, "OPTIONS")
}

func TestCORS_HeadersAllowed(t *testing.T) {
	middleware := CORS([]string{"http://localhost:3000"})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	headersHeader := w.Header().Get("Access-Control-Allow-Headers")
	testutil.AssertContains(t, headersHeader, "Content-Type")
	testutil.AssertContains(t, headersHeader, "Authorization")
}

func TestCORS_RegularRequestPassesThrough(t *testing.T) {
	middleware := CORS([]string{"http://localhost:3000"})

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("response body"))
	})

	handler := middleware(nextHandler)

	// Non-OPTIONS request should pass through to next handler
	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			nextHandlerCalled = false
			req := httptest.NewRequest(method, "/api/test", nil)
			req.Header.Set("Origin", "http://localhost:3000")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			testutil.AssertTrue(t, nextHandlerCalled, "next handler should be called for "+method)
		})
	}
}

func TestCORS_DisallowedOriginNoHeaders(t *testing.T) {
	middleware := CORS([]string{"http://localhost:3000"})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "http://malicious.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// CORS headers should NOT be set for disallowed origin
	testutil.AssertEqual(t, w.Header().Get("Access-Control-Allow-Origin"), "")
	testutil.AssertEqual(t, w.Header().Get("Access-Control-Allow-Credentials"), "")
	testutil.AssertEqual(t, w.Header().Get("Access-Control-Allow-Methods"), "")
	testutil.AssertEqual(t, w.Header().Get("Access-Control-Allow-Headers"), "")
}

func TestParseOrigins_SingleOrigin(t *testing.T) {
	origins := ParseOrigins("http://localhost:3000")

	testutil.AssertLen(t, origins, 1)
	testutil.AssertEqual(t, origins[0], "http://localhost:3000")
}

func TestParseOrigins_MultipleOrigins(t *testing.T) {
	origins := ParseOrigins("http://localhost:3000,http://example.com,http://test.com")

	testutil.AssertLen(t, origins, 3)
	testutil.AssertEqual(t, origins[0], "http://localhost:3000")
	testutil.AssertEqual(t, origins[1], "http://example.com")
	testutil.AssertEqual(t, origins[2], "http://test.com")
}

func TestParseOrigins_TrimSpaces(t *testing.T) {
	origins := ParseOrigins("  http://localhost:3000  ,  http://example.com  ")

	testutil.AssertLen(t, origins, 2)
	testutil.AssertEqual(t, origins[0], "http://localhost:3000")
	testutil.AssertEqual(t, origins[1], "http://example.com")
}

func TestParseOrigins_Wildcard(t *testing.T) {
	origins := ParseOrigins("*")

	testutil.AssertLen(t, origins, 1)
	testutil.AssertEqual(t, origins[0], "*")
}

func TestParseOrigins_EmptyString(t *testing.T) {
	origins := ParseOrigins("")

	// Should return slice with one empty string
	testutil.AssertLen(t, origins, 1)
}

func TestCORS_PreflightWithDisallowedOrigin(t *testing.T) {
	middleware := CORS([]string{"http://localhost:3000"})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
	req.Header.Set("Origin", "http://malicious.com")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Should still return 200 for OPTIONS
	testutil.AssertStatusCode(t, w, http.StatusOK)

	// But CORS headers should NOT be set
	testutil.AssertEqual(t, w.Header().Get("Access-Control-Allow-Origin"), "")
}

func TestCORS_NoOriginHeader(t *testing.T) {
	middleware := CORS([]string{"http://localhost:3000"})

	nextHandlerCalled := false
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextHandlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	// No Origin header
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	// Request should still pass through
	testutil.AssertTrue(t, nextHandlerCalled, "request without Origin should pass through")

	// No CORS headers should be set
	testutil.AssertEqual(t, w.Header().Get("Access-Control-Allow-Origin"), "")
}

// Benchmark CORS middleware
func BenchmarkCORS(b *testing.B) {
	middleware := CORS([]string{"http://localhost:3000", "http://example.com"})

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}
}

func BenchmarkParseOrigins(b *testing.B) {
	originsStr := "http://localhost:3000, http://example.com, http://test.com, http://dev.local"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseOrigins(originsStr)
	}
}
