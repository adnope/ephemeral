package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/adnope/ephemeral/internal/store"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "session_token"

// GET /login
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	count, err := h.users.Count(r.Context())
	if err != nil {
		h.log.Error("login: user count", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"IsSetup": count == 0,
		"Error":   r.URL.Query().Get("error"),
	}

	if err := h.tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
		h.log.Error("login: render", "err", err)
	}
}

// POST /login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		redirectLoginError(w, r, "missing+credentials")
		return
	}

	count, err := h.users.Count(r.Context())
	if err != nil {
		h.log.Error("login: user count", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if count == 0 {
		if err := h.createInitialUser(r, username, password); err != nil {
			h.log.Error("login: create initial user", "err", err)
			redirectLoginError(w, r, "user+creation+failed")
			return
		}
	}

	user, err := h.users.GetByUsername(r.Context(), username)
	if err != nil {
		redirectLoginError(w, r, "invalid+credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		redirectLoginError(w, r, "invalid+credentials")
		return
	}

	token, err := generateSessionToken()
	if err != nil {
		h.log.Error("login: generate token", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	session := &store.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(h.sessionTTL),
	}

	if err := h.sessions.Create(r.Context(), session); err != nil {
		h.log.Error("login: create session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, newSessionCookie(token, h.sessionTTL))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// POST /logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		if delErr := h.sessions.Delete(r.Context(), cookie.Value); delErr != nil {
			h.log.Error("logout: delete session", "err", delErr)
		}
	}

	http.SetCookie(w, expiredSessionCookie())
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) createInitialUser(r *http.Request, username string, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return err
	}

	user := &store.User{
		Username:     username,
		PasswordHash: string(hash),
	}

	_, err = h.users.Create(r.Context(), user)
	return err
}

func generateSessionToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(tokenBytes), nil
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
