package middleware

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

func RateLimit(maxTokens int, window time.Duration) func(http.Handler) http.Handler {
	type bucket struct {
		tokens   int
		lastFill time.Time
	}

	var (
		mu      sync.Mutex
		buckets = make(map[string]*bucket)
	)

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
			if skipRateLimit(r) {
				next.ServeHTTP(w, r)
				return
			}

			key := clientRateLimitKey(r)

			mu.Lock()
			b, ok := buckets[key]
			if !ok {
				b = &bucket{tokens: maxTokens, lastFill: time.Now()}
				buckets[key] = b
			}

			elapsed := time.Since(b.lastFill)
			if elapsed >= window {
				b.tokens = maxTokens
				b.lastFill = time.Now()
			}

			if b.tokens <= 0 {
				retryAfter := window - elapsed
				if retryAfter < time.Second {
					retryAfter = time.Second
				}
				mu.Unlock()
				writeRateLimitExceeded(w, r, retryAfter)
				return
			}

			b.tokens--
			mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

func skipRateLimit(r *http.Request) bool {
	if strings.HasPrefix(r.URL.Path, "/static/") {
		return true
	}

	if r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/files/") {
		return true
	}

	switch r.URL.Path {
	case "/api/events", "/favicon.ico", "/manifest.json":
		return true
	default:
		return false
	}
}

func clientRateLimitKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || host == "" {
		host = r.RemoteAddr
	}
	if host == "" {
		host = "unknown"
	}

	return host + ":" + rateLimitClass(r)
}

func rateLimitClass(r *http.Request) string {
	switch r.URL.Path {
	case "/api/auth/state", "/api/login", "/api/logout", "/login":
		return "auth"
	default:
		return "default"
	}
}

func writeRateLimitExceeded(w http.ResponseWriter, r *http.Request, retryAfter time.Duration) {
	seconds := int(retryAfter.Round(time.Second).Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))

	if wantsJSONResponse(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":"rate_limited","message":"rate limit exceeded"}` + "\n"))
		return
	}

	http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
}
