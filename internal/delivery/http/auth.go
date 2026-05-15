package httpdelivery

import (
	"errors"
	"net/http"
	"time"

	"github.com/adnope/ephemeral/internal/usecase"
)

const sessionCookieName = "session_token"

// LoginPage handles GET /login.
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	page, err := h.auth.LoginPage(r.Context())
	if err != nil {
		h.log.Error("login: page data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"IsSetup": page.IsSetup,
		"Error":   r.URL.Query().Get("error"),
	}

	if err := h.tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
		h.log.Error("login: render", "err", err)
	}
}

// Login handles POST /api/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	result, err := h.auth.Login(r.Context(), r.FormValue("username"), r.FormValue("password"))
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrMissingCredentials):
			redirectLoginError(w, r, "missing+credentials")
		case errors.Is(err, usecase.ErrInvalidCredentials):
			redirectLoginError(w, r, "invalid+credentials")
		case errors.Is(err, usecase.ErrUserCreationFailed):
			h.log.Error("login: create initial user", "err", err)
			redirectLoginError(w, r, "user+creation+failed")
		default:
			h.log.Error("login: usecase", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	http.SetCookie(w, newSessionCookie(result.Token, result.TTL))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout handles POST /api/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		if logoutErr := h.auth.Logout(r.Context(), cookie.Value); logoutErr != nil {
			h.log.Error("logout: delete session", "err", logoutErr)
		}
	}

	http.SetCookie(w, expiredSessionCookie())
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func newSessionCookie(token string, ttl time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func expiredSessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
}

func redirectLoginError(w http.ResponseWriter, r *http.Request, message string) {
	http.Redirect(w, r, "/login?error="+message, http.StatusSeeOther)
}
