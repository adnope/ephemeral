package httpdelivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
	"github.com/adnope/ephemeral/internal/infrastructure/sqlite"
	"github.com/adnope/ephemeral/internal/middleware"
	"github.com/adnope/ephemeral/internal/migrations"
	"github.com/adnope/ephemeral/internal/usecase"
	"golang.org/x/crypto/bcrypt"
)

func TestNewSessionCookieCanBeSecure(t *testing.T) {
	cookie := newSessionCookie("token", time.Hour, true)
	if !cookie.Secure {
		t.Fatal("Secure = false, want true")
	}
	if !cookie.HttpOnly {
		t.Fatal("HttpOnly = false, want true")
	}
}

func TestChangePassword(t *testing.T) {
	ctx := context.Background()
	schema, err := migrations.FS.ReadFile("001_initial.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.OpenDB(filepath.Join(t.TempDir(), "auth.db"), string(schema))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	users := sqlite.NewUserRepository(db)
	sessions := sqlite.NewSessionRepository(db)
	auth := usecase.NewAuthUseCase(users, sessions, time.Hour)
	login, err := auth.Login(ctx, "owner", "old password")
	if err != nil {
		t.Fatal(err)
	}
	original, err := users.GetByUsername(ctx, "owner")
	if err != nil {
		t.Fatal(err)
	}
	otherID, err := users.Create(ctx, &domain.User{Username: "other", PasswordHash: original.PasswordHash})
	if err != nil {
		t.Fatal(err)
	}
	if err := sessions.Create(ctx, &domain.Session{Token: "expired", UserID: original.ID, ExpiresAt: time.Now().Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(nil, nil, auth, nil, nil, HandlerSettings{})
	handler := middleware.SessionAuth(sessions, time.Hour, false)(http.HandlerFunc(h.ChangePassword))
	request := func(body, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, "/api/change-password", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}
	valid := `{"oldPassword":"old password","newPassword":"new password"}`
	for _, tc := range []struct {
		name, body, token, code string
		status                  int
	}{
		{"unauthenticated", valid, "", "unauthenticated", 401},
		{"expired session", valid, "expired", "unauthenticated", 401},
		{"wrong password", `{"oldPassword":"wrong","newPassword":"new password"}`, login.Token, "invalid_password", 403},
		{"missing old password", `{"newPassword":"new password"}`, login.Token, "validation_error", 400},
		{"missing new password", `{"oldPassword":"old password"}`, login.Token, "validation_error", 400},
		{"long password", `{"oldPassword":"old password","newPassword":"` + strings.Repeat("a", 73) + `"}`, login.Token, "validation_error", 400},
		{"long UTF-8 password", `{"oldPassword":"old password","newPassword":"` + strings.Repeat("é", 37) + `"}`, login.Token, "validation_error", 400},
		{"malformed JSON", `{`, login.Token, "validation_error", 400},
		{"unknown field", `{"userID":2,"oldPassword":"old password","newPassword":"new password"}`, login.Token, "validation_error", 400},
		{"oversized JSON", `{"oldPassword":"` + strings.Repeat("a", maxJSONBodyBytes) + `"}`, login.Token, "payload_too_large", 413},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := request(tc.body, tc.token)
			var body jsonErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if w.Code != tc.status || body.Code != tc.code || body.Message == "" {
				t.Fatalf("response = %d %s", w.Code, w.Body.String())
			}
			user, err := users.GetByID(ctx, original.ID)
			if err != nil || user.PasswordHash != original.PasswordHash {
				t.Fatalf("rejected request changed password: %v", err)
			}
		})
	}
	w := request(valid, login.Token)
	if w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Fatalf("success response = %d %s", w.Code, w.Body.String())
	}
	updated, err := users.GetByID(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(updated.PasswordHash), []byte("new password")); err != nil {
		t.Fatalf("new password was not hashed and persisted: %v", err)
	}
	if _, err := auth.Login(ctx, "owner", "old password"); err == nil {
		t.Fatal("old password still accepted")
	}
	if _, err := auth.Login(ctx, "owner", "new password"); err != nil {
		t.Fatalf("new password rejected: %v", err)
	}
	other, err := users.GetByID(ctx, otherID)
	if err != nil || other.PasswordHash != original.PasswordHash {
		t.Fatalf("another user's password changed: %v", err)
	}
	if changed, err := users.UpdatePassword(ctx, original.ID, original.PasswordHash, "stale update"); err != nil || changed {
		t.Fatalf("stale password update = %v, %v", changed, err)
	}
	if w := request(valid, login.Token); w.Code != http.StatusForbidden {
		t.Fatalf("existing session should stay signed in but old password should fail: %d", w.Code)
	}
	if err := auth.Logout(ctx, login.Token); err != nil {
		t.Fatal(err)
	}
	if w := request(valid, login.Token); w.Code != http.StatusUnauthorized {
		t.Fatalf("logged out session accepted: %d", w.Code)
	}
}
