package auth

import (
	"crypto/subtle"
	"net/http"
	"os"
	"sync"
	"time"
)

// RateLimiter implements a simple per-IP token-bucket rate limiter.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // requests per window
	window  time.Duration // window duration
}

type bucket struct {
	count    int
	resetAt  time.Time
}

// NewRateLimiter creates a limiter allowing `rate` requests per `window`.
func NewRateLimiter(rate int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		buckets: make(map[string]*bucket),
		rate:    rate,
		window:  window,
	}
	// Periodic cleanup goroutine
	go func() {
		for range time.Tick(window * 2) {
			rl.mu.Lock()
			now := time.Now()
			for k, b := range rl.buckets {
				if now.After(b.resetAt) {
					delete(rl.buckets, k)
				}
			}
			rl.mu.Unlock()
		}
	}()
	return rl
}

// Allow returns true if the IP is within rate limits.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok || now.After(b.resetAt) {
		rl.buckets[ip] = &bucket{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	if b.count >= rl.rate {
		return false
	}
	b.count++
	return true
}

// Middleware returns an HTTP middleware enforcing rate limits and optional
// admin API key authentication.
func Middleware(rl *RateLimiter) func(http.Handler) http.Handler {
	adminKey := os.Getenv("TOKENSIEVE_ADMIN_KEY")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
				ip = xff
			}

			if !rl.Allow(ip) {
				http.Error(w, `{"error":"rate_limit_exceeded"}`, http.StatusTooManyRequests)
				return
			}

			// Admin endpoints require API key
			if adminKey != "" && r.URL.Path == "/api/admin" {
				provided := r.Header.Get("X-Admin-Key")
				if subtle.ConstantTimeCompare([]byte(provided), []byte(adminKey)) != 1 {
					http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
					return
				}
			}

			// CORS preflight pass-through
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Session-ID, X-Admin-Key")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
