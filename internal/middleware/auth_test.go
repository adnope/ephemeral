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

func TestPublicSharePathsSkipSessionAuth(t *testing.T) {
	if !isPublicPath("/share/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatal("/share/{token} should be public")
	}
	if !isPublicPath("/share/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/file") {
		t.Fatal("/share/{token}/file should be public")
	}
	if !isPublicPath("/api/share/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa") {
		t.Fatal("/api/share/{token} should be public")
	}
	if !isPublicPath("/assets/app-hash.js") {
		t.Fatal("/assets/* should be public")
	}
}
