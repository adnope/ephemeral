package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

type contextKey string

const ctxKeySession contextKey = "session"

var publicPaths = map[string]struct{}{
	"/login":          {},
	"/api/auth/state": {},
	"/api/login":      {},
	"/favicon.ico":    {},
	"/manifest.json":  {},
}

func SessionAuth(repo domain.SessionRepository, sessionTTL time.Duration, cookieSecure bool) func(http.Handler) http.Handler {
	if sessionTTL < time.Minute {
		sessionTTL = time.Minute
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("session_token")
			if err != nil {
				unauthenticated(w, r)
				return
			}

			session, err := repo.GetByToken(r.Context(), cookie.Value)
			if err != nil {
				http.SetCookie(w, expiredCookie())
				unauthenticated(w, r)
				return
			}

			now := time.Now()
			if session.ExpiresAt.Before(now) {
				http.SetCookie(w, expiredCookie())
				unauthenticated(w, r)
				return
			}

			refreshWindow := max(sessionTTL/3, time.Minute)

			if time.Until(session.ExpiresAt) <= refreshWindow {
				newExpiresAt := now.Add(sessionTTL)

				if err := repo.Refresh(r.Context(), session.Token, newExpiresAt); err != nil {
					http.SetCookie(w, expiredCookie())
					unauthenticated(w, r)
					return
				}

				session.ExpiresAt = newExpiresAt
				http.SetCookie(w, sessionCookie(session.Token, sessionTTL, cookieSecure))
			}

			ctx := context.WithValue(r.Context(), ctxKeySession, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func isPublicPath(path string) bool {
	if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/share/") {
		return true
	}
	_, ok := publicPaths[path]
	return ok
}

func unauthenticated(w http.ResponseWriter, r *http.Request) {
	if wantsJSONResponse(r) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"code":    "unauthenticated",
			"message": "authentication required",
		})
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func wantsJSONResponse(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json") ||
		hasJSONRequestBody(r)
}

func hasJSONRequestBody(r *http.Request) bool {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	return strings.TrimSpace(contentType) == "application/json"
}

func GetSession(ctx context.Context) *domain.Session {
	s, _ := ctx.Value(ctxKeySession).(*domain.Session)
	return s
}

func sessionCookie(token string, ttl time.Duration, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	}
}

func expiredCookie() *http.Cookie {
	return &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}
