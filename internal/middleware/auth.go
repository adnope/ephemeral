package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/store"
)

type contextKey string

const ctxKeySession contextKey = "session"

var publicPaths = map[string]struct{}{
	"/login":         {},
	"/api/login":     {},
	"/favicon.ico":   {},
	"/manifest.json": {},
}

// SessionAuth enforces cookie-based session authentication.
// Skips static assets and explicitly public paths.
func SessionAuth(repo store.SessionRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Static assets bypass auth entirely
			if strings.HasPrefix(r.URL.Path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := publicPaths[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}

			cookie, err := r.Cookie("session_token")
			if err != nil {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			session, err := repo.GetByToken(r.Context(), cookie.Value)
			if err != nil || session.ExpiresAt.Before(time.Now()) {
				http.SetCookie(w, expiredCookie())
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			ctx := context.WithValue(r.Context(), ctxKeySession, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetSession extracts the session from context. Returns nil if not authenticated.
func GetSession(ctx context.Context) *store.Session {
	s, _ := ctx.Value(ctxKeySession).(*store.Session)
	return s
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
