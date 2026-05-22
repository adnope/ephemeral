package httpdelivery

import (
	"testing"
	"time"
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
