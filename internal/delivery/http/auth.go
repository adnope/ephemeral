package httpdelivery

import (
	"errors"
	"net/http"
	"time"

	"github.com/adnope/ephemeral/internal/usecase"
)

const sessionCookieName = "session_token"

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type authStateResponse struct {
	SetupRequired bool `json:"setupRequired"`
}

type loginResponse struct {
	Authenticated bool `json:"authenticated"`
}

// AuthState handles GET /api/auth/state.
func (h *Handler) AuthState(w http.ResponseWriter, r *http.Request) {
	page, err := h.auth.LoginPage(r.Context())
	if err != nil {
		h.log.Error("auth state: page data", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, authStateResponse{SetupRequired: page.IsSetup})
}

// Login handles POST /api/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var username string
	var password string

	if hasJSONContentType(r) {
		var req loginRequest
		if err := decodeJSON(w, r, &req); err != nil {
			if errors.Is(err, errJSONBodyTooLarge) {
				writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "JSON body too large")
				return
			}
			writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid JSON body")
			return
		}
		username = req.Username
		password = req.Password
	} else {
		if err := r.ParseForm(); err != nil {
			if wantsJSON(r) {
				writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid form")
			} else {
				http.Error(w, "invalid form", http.StatusBadRequest)
			}
			return
		}
		username = r.FormValue("username")
		password = r.FormValue("password")
	}

	result, err := h.auth.Login(r.Context(), username, password)
	if err != nil {
		if wantsJSON(r) {
			switch {
			case errors.Is(err, usecase.ErrMissingCredentials):
				writeJSONError(w, http.StatusBadRequest, "validation_error", "missing credentials")
			case errors.Is(err, usecase.ErrInvalidCredentials):
				writeJSONError(w, http.StatusUnauthorized, "unauthenticated", "invalid credentials")
			case errors.Is(err, usecase.ErrUserCreationFailed):
				h.log.Error("login: create initial user", "err", err)
				writeJSONError(w, http.StatusInternalServerError, "server_error", "user creation failed")
			default:
				h.log.Error("login: usecase", "err", err)
				writeJSONError(w, http.StatusInternalServerError, "server_error", "internal error")
			}
			return
		}

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

	http.SetCookie(w, newSessionCookie(result.Token, result.TTL, h.settings.CookieSecure))
	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, loginResponse{Authenticated: true})
		return
	}
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
	if wantsJSON(r) {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func newSessionCookie(token string, ttl time.Duration, secure bool) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   secure,
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
