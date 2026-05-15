package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitSkipsFilesAndEvents(t *testing.T) {
	t.Parallel()

	var hits int
	handler := RateLimit(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, path := range []string{"/api/files/example.jpg", "/api/events"} {
		for i := 0; i < 3; i++ {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			req.RemoteAddr = "192.0.2.10:1234"
			res := httptest.NewRecorder()

			handler.ServeHTTP(res, req)
			if res.Code != http.StatusNoContent {
				t.Fatalf("%s status = %d, want %d", path, res.Code, http.StatusNoContent)
			}
		}
	}

	if hits != 6 {
		t.Fatalf("hits = %d, want 6", hits)
	}
}

func TestRateLimitSeparatesAuthAndDefaultBuckets(t *testing.T) {
	t.Parallel()

	handler := RateLimit(1, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	defaultReq := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	defaultReq.RemoteAddr = "192.0.2.10:1234"
	defaultRes := httptest.NewRecorder()
	handler.ServeHTTP(defaultRes, defaultReq)
	if defaultRes.Code != http.StatusNoContent {
		t.Fatalf("first default status = %d", defaultRes.Code)
	}

	defaultRes = httptest.NewRecorder()
	handler.ServeHTTP(defaultRes, defaultReq)
	if defaultRes.Code != http.StatusTooManyRequests {
		t.Fatalf("second default status = %d, want 429", defaultRes.Code)
	}

	authReq := httptest.NewRequest(http.MethodPost, "/api/login", nil)
	authReq.RemoteAddr = "192.0.2.10:5678"
	authReq.Header.Set("Accept", "application/json")
	authRes := httptest.NewRecorder()
	handler.ServeHTTP(authRes, authReq)
	if authRes.Code != http.StatusNoContent {
		t.Fatalf("auth status = %d, want %d", authRes.Code, http.StatusNoContent)
	}
}

func TestRateLimitJSONResponse(t *testing.T) {
	t.Parallel()

	handler := RateLimit(0, time.Minute)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("Accept", "application/json")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", res.Code)
	}
	if res.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header missing")
	}
	if got := res.Body.String(); got != "{\"code\":\"rate_limited\",\"message\":\"rate limit exceeded\"}\n" {
		t.Fatalf("body = %q", got)
	}
}
