package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	maxLimiters     = 10000
	cleanupInterval = 5 * time.Minute
	limiterTTL      = 15 * time.Minute
)

type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

type RateLimiter struct {
	limiters map[string]*limiterEntry
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	stopCh   chan struct{}
}

func NewRateLimiter(ctx context.Context, requestsPerSecond float64, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*limiterEntry),
		rate:     rate.Limit(requestsPerSecond),
		burst:    burst,
		stopCh:   make(chan struct{}),
	}

	go rl.cleanupLoop(ctx)

	return rl
}

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

func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	for key, entry := range rl.limiters {
		if now.Sub(entry.lastAccess) > limiterTTL {
			delete(rl.limiters, key)
		}
	}

	if len(rl.limiters) > maxLimiters {
		type keyTime struct {
			key  string
			time time.Time
		}
		entries := make([]keyTime, 0, len(rl.limiters))
		for k, e := range rl.limiters {
			entries = append(entries, keyTime{k, e.lastAccess})
		}

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

func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	entry, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		rl.mu.Lock()
		entry.lastAccess = time.Now()
		rl.mu.Unlock()
		return entry.limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists = rl.limiters[key]
	if exists {
		entry.lastAccess = time.Now()
		return entry.limiter
	}

	entry = &limiterEntry{
		limiter:    rate.NewLimiter(rl.rate, rl.burst),
		lastAccess: time.Now(),
	}
	rl.limiters[key] = entry
	return entry.limiter
}

func (rl *RateLimiter) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
