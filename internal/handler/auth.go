package handler

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/adnope/leandrop/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// LoginPage renders the login form.
// GET /login
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Check if setup is needed (no users exist yet)
	count, err := h.users.Count(r.Context())
	if err != nil {
		h.log.Error("login: user count", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"IsSetup": count == 0,
		"Error":   r.URL.Query().Get("error"),
	}

	if err := h.tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
		h.log.Error("login: render", "err", err)
	}
}

// Login handles credential validation and session creation.
// POST /login
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		http.Redirect(w, r, "/login?error=missing+credentials", http.StatusSeeOther)
		return
	}

	// Check if this is first-time setup
	count, err := h.users.Count(r.Context())
	if err != nil {
		h.log.Error("login: user count", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if count == 0 {
		// First run: create the user account
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			h.log.Error("login: hash password", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		user := &store.User{
			Username:     username,
			PasswordHash: string(hash),
		}
		if _, err := h.users.Create(r.Context(), user); err != nil {
			h.log.Error("login: create user", "err", err)
			http.Redirect(w, r, "/login?error=user+creation+failed", http.StatusSeeOther)
			return
		}
	}

	// Authenticate
	user, err := h.users.GetByUsername(r.Context(), username)
	if err != nil {
		http.Redirect(w, r, "/login?error=invalid+credentials", http.StatusSeeOther)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		http.Redirect(w, r, "/login?error=invalid+credentials", http.StatusSeeOther)
		return
	}

	// Generate session token: 32 bytes of crypto/rand
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		h.log.Error("login: generate token", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	token := hex.EncodeToString(tokenBytes)

	session := &store.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour), // 30 days
	}

	if err := h.sessions.Create(r.Context(), session); err != nil {
		h.log.Error("login: create session", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout invalidates the current session.
// POST /logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		if delErr := h.sessions.Delete(r.Context(), cookie.Value); delErr != nil {
			h.log.Error("logout: delete session", "err", delErr)
		}
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
