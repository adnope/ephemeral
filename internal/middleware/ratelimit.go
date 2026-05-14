package middleware

import (
	"net/http"
	"sync"
	"time"
)

// RateLimit implements a simple token-bucket rate limiter per IP.
// Uses a map protected by sync.Mutex; sufficient for single-server deployment.
func RateLimit(maxTokens int, window time.Duration) func(http.Handler) http.Handler {
	type bucket struct {
		tokens   int
		lastFill time.Time
	}

	var (
		mu      sync.Mutex
		buckets = make(map[string]*bucket)
	)

	// Background cleanup: purge stale buckets every 5 minutes
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for ip, b := range buckets {
				if now.Sub(b.lastFill) > 10*time.Minute {
					delete(buckets, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			mu.Lock()
			b, ok := buckets[ip]
			if !ok {
				b = &bucket{tokens: maxTokens, lastFill: time.Now()}
				buckets[ip] = b
			}

			// Refill tokens based on elapsed time
			elapsed := time.Since(b.lastFill)
			if elapsed >= window {
				b.tokens = maxTokens
				b.lastFill = time.Now()
			}

			if b.tokens <= 0 {
				mu.Unlock()
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}

			b.tokens--
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}
