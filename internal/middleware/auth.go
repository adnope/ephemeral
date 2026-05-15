package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

type contextKey string

const ctxKeySession contextKey = "session"

var publicPaths = map[string]struct{}{
	"/login":         {},
	"/api/login":     {},
	"/favicon.ico":   {},
	"/manifest.json": {},
}

func SessionAuth(repo domain.SessionRepository, sessionTTL time.Duration) func(http.Handler) http.Handler {
	if sessionTTL < time.Minute {
		sessionTTL = time.Minute
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			if err != nil {
				http.SetCookie(w, expiredCookie())
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			now := time.Now()
			if session.ExpiresAt.Before(now) {
				http.SetCookie(w, expiredCookie())
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}

			refreshWindow := sessionTTL / 3
			if refreshWindow < time.Minute {
				refreshWindow = time.Minute
			}

			if time.Until(session.ExpiresAt) <= refreshWindow {
				newExpiresAt := now.Add(sessionTTL)

				if err := repo.Refresh(r.Context(), session.Token, newExpiresAt); err != nil {
					http.SetCookie(w, expiredCookie())
					http.Redirect(w, r, "/login", http.StatusSeeOther)
					return
				}

				session.ExpiresAt = newExpiresAt
				http.SetCookie(w, sessionCookie(session.Token, sessionTTL))
			}

			ctx := context.WithValue(r.Context(), ctxKeySession, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetSession(ctx context.Context) *domain.Session {
	s, _ := ctx.Value(ctxKeySession).(*domain.Session)
	return s
}

func sessionCookie(token string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
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
