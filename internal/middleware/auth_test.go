package middleware

import (
	"testing"
	"time"
)

func TestSessionCookieCanBeSecure(t *testing.T) {
	cookie := sessionCookie("token", time.Hour, true)
	if !cookie.Secure {
		t.Fatal("Secure = false, want true")
	}
	if !cookie.HttpOnly {
		t.Fatal("HttpOnly = false, want true")
	}
}
