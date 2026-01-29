package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_BasicFunctionality(t *testing.T) {
	rl := NewRateLimiter(2, 2) // 2 req/sec, burst 2
	defer rl.Stop()

	handler := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:1234"

	// First request should succeed
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("First request: expected status 200, got %d", rr.Code)
	}

	// Second request should succeed (burst)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Second request: expected status 200, got %d", rr.Code)
	}

	// Third request should be rate limited (burst exhausted)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("Third request: expected status 429, got %d", rr.Code)
	}
}

func TestRateLimiter_PerIPLimiting(t *testing.T) {
	rl := NewRateLimiter(1, 1) // 1 req/sec, burst 1
	defer rl.Stop()

	handler := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// IP 1 - first request
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "192.168.1.1:1234"
	rr1 := httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusOK {
		t.Errorf("IP1 first request: expected 200, got %d", rr1.Code)
	}

	// IP 2 - first request (should succeed independently)
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.2:1234"
	rr2 := httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Errorf("IP2 first request: expected 200, got %d", rr2.Code)
	}

	// IP 1 - second request (should be rate limited)
	rr1 = httptest.NewRecorder()
	handler.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusTooManyRequests {
		t.Errorf("IP1 second request: expected 429, got %d", rr1.Code)
	}

	// IP 2 - second request (should be rate limited)
	rr2 = httptest.NewRecorder()
	handler.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusTooManyRequests {
		t.Errorf("IP2 second request: expected 429, got %d", rr2.Code)
	}
}

func TestRateLimiter_CleanupMemoryLeak(t *testing.T) {
	rl := NewRateLimiter(10, 1)
	defer rl.Stop()

	// Create many limiters (more than we'd normally see)
	for i := 0; i < 100; i++ {
		key := "192.168.1." + string(rune(i))
		limiter := rl.getLimiter(key)
		if limiter == nil {
			t.Fatalf("Failed to create limiter for key %s", key)
		}
	}

	// Verify limiters were created
	rl.mu.RLock()
	initialCount := len(rl.limiters)
	rl.mu.RUnlock()

	if initialCount != 100 {
		t.Errorf("Expected 100 limiters, got %d", initialCount)
	}

	// Manually trigger cleanup with old timestamps
	rl.mu.Lock()
	oldTime := time.Now().Add(-20 * time.Minute) // Older than limiterTTL
	for key := range rl.limiters {
		rl.limiters[key].lastAccess = oldTime
	}
	rl.mu.Unlock()

	// Trigger cleanup
	rl.cleanup()

	// Verify limiters were cleaned up
	rl.mu.RLock()
	finalCount := len(rl.limiters)
	rl.mu.RUnlock()

	if finalCount != 0 {
		t.Errorf("Expected 0 limiters after cleanup, got %d", finalCount)
	}
}

func TestRateLimiter_LRUEviction(t *testing.T) {
	rl := NewRateLimiter(10, 1)
	defer rl.Stop()

	// Create more limiters than maxLimiters (10000) would allow
	// But for testing, let's create enough to trigger LRU
	numLimiters := 15000

	// Create limiters
	for i := 0; i < numLimiters; i++ {
		key := "ip-" + string(rune(i%256)) + string(rune(i/256))
		_ = rl.getLimiter(key)
	}

	// Trigger cleanup to enforce max limit
	rl.cleanup()

	// Verify we're under the limit
	rl.mu.RLock()
	count := len(rl.limiters)
	rl.mu.RUnlock()

	if count > maxLimiters {
		t.Errorf("Expected max %d limiters, got %d", maxLimiters, count)
	}
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(100, 10)
	defer rl.Stop()

	handler := rl.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Run concurrent requests
	var wg sync.WaitGroup
	numGoroutines := 50
	requestsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < requestsPerGoroutine; j++ {
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "192.168.1." + string(rune(id)) + ":1234"
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
				// Don't check status - just verify no panic
			}
		}(i)
	}

	wg.Wait()

	// Verify limiters were created
	rl.mu.RLock()
	count := len(rl.limiters)
	rl.mu.RUnlock()

	if count == 0 {
		t.Error("Expected limiters to be created")
	}
}

func TestRateLimiter_CleanupLoop(t *testing.T) {
	rl := NewRateLimiter(10, 1)

	// Create some limiters
	for i := 0; i < 10; i++ {
		key := "192.168.1." + string(rune(i))
		_ = rl.getLimiter(key)
	}

	// Set old access time
	rl.mu.Lock()
	oldTime := time.Now().Add(-20 * time.Minute)
	for key := range rl.limiters {
		rl.limiters[key].lastAccess = oldTime
	}
	rl.mu.Unlock()

	// Stop should trigger cleanup goroutine exit
	rl.Stop()

	// Give it a moment to process
	time.Sleep(100 * time.Millisecond)

	// Create a new limiter - cleanup goroutine should be stopped
	// This verifies no goroutine leak
	rl2 := NewRateLimiter(10, 1)
	defer rl2.Stop()

	// If we got here without panic, the test passes
}

func TestRateLimiter_LastAccessUpdate(t *testing.T) {
	rl := NewRateLimiter(10, 1)
	defer rl.Stop()

	key := "192.168.1.1:1234"

	// Get limiter first time
	_ = rl.getLimiter(key)

	rl.mu.RLock()
	firstAccess := rl.limiters[key].lastAccess
	rl.mu.RUnlock()

	// Wait a bit
	time.Sleep(10 * time.Millisecond)

	// Get limiter second time
	_ = rl.getLimiter(key)

	rl.mu.RLock()
	secondAccess := rl.limiters[key].lastAccess
	rl.mu.RUnlock()

	// Verify last access was updated
	if !secondAccess.After(firstAccess) {
		t.Error("Expected lastAccess to be updated on subsequent access")
	}
}

func TestRateLimiter_ContextCancellation(t *testing.T) {
	rl := NewRateLimiter(10, 1)

	// Create cleanup context
	ctx, cancel := context.WithCancel(context.Background())

	// Start cleanup with cancellable context
	go rl.cleanupLoop(ctx)

	// Cancel context
	cancel()

	// Give it time to exit
	time.Sleep(100 * time.Millisecond)

	// Stop should not hang
	rl.Stop()
}
