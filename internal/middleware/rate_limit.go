package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	// Maximum number of limiters to keep in memory
	maxLimiters = 10000
	// Time after which an inactive limiter is removed
	cleanupInterval = 5 * time.Minute
	// Limiter is considered inactive if not used for this duration
	limiterTTL = 15 * time.Minute
)

// limiterEntry wraps a rate.Limiter with last access time
type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// RateLimiter provides per-IP rate limiting for HTTP requests with memory management
type RateLimiter struct {
	limiters map[string]*limiterEntry
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	stopCh   chan struct{}
}

// NewRateLimiter creates a new rate limiter with automatic cleanup
// requestsPerSecond: maximum average rate of requests per IP
// burst: maximum burst size (tokens bucket capacity)
func NewRateLimiter(requestsPerSecond float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*limiterEntry),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
		stopCh:   make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.cleanupLoop(context.Background())

	return rl
}

// cleanupLoop periodically removes inactive limiters to prevent memory leaks
func (rl *RateLimiter) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-rl.stopCh:
			return
		case <-ticker.C:
			rl.cleanup()
		}
	}
}

// cleanup removes limiters that haven't been used recently
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.limiters {
		if now.Sub(entry.lastAccess) > limiterTTL {
			delete(rl.limiters, key)
		}
	}

	// If still over limit, remove oldest entries (LRU eviction)
	if len(rl.limiters) > maxLimiters {
		// Find and remove oldest entries
		type keyTime struct {
			key  string
			time time.Time
		}
		entries := make([]keyTime, 0, len(rl.limiters))
		for k, e := range rl.limiters {
			entries = append(entries, keyTime{k, e.lastAccess})
		}

		// Sort by access time and remove oldest
		for i := 0; i < len(entries)-maxLimiters/2; i++ {
			oldestKey := entries[0].key
			oldestTime := entries[0].time
			oldestIdx := 0

			for j, e := range entries {
				if e.time.Before(oldestTime) {
					oldestKey = e.key
					oldestTime = e.time
					oldestIdx = j
				}
			}

			delete(rl.limiters, oldestKey)
			entries = append(entries[:oldestIdx], entries[oldestIdx+1:]...)
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// getLimiter returns the rate limiter for a given key (usually IP address)
// Creates a new limiter if one doesn't exist and updates last access time
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	// Try read lock first for better concurrency
	rl.mu.RLock()
	entry, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		// Update last access time
		rl.mu.Lock()
		entry.lastAccess = time.Now()
		rl.mu.Unlock()
		return entry.limiter
	}

	// Need to create new limiter
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	entry, exists = rl.limiters[key]
	if exists {
		entry.lastAccess = time.Now()
		return entry.limiter
	}

	// Create new limiter
	entry = &limiterEntry{
		limiter:    rate.NewLimiter(rl.rate, rl.burst),
		lastAccess: time.Now(),
	}
	rl.limiters[key] = entry
	return entry.limiter
}

// Middleware returns a chi-compatible middleware function
func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Use remote address as key for per-IP limiting
			key := r.RemoteAddr
			limiter := rl.getLimiter(key)

			if !limiter.Allow() {
				http.Error(w, `{"error":"Rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
