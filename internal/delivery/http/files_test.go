package httpdelivery

import (
	"net/http/httptest"
	"testing"
)

func TestSetUploadFileHeadersSandboxesActiveDocuments(t *testing.T) {
	res := httptest.NewRecorder()

	setUploadFileHeaders(res, "example.svg")

	if got := res.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := res.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("Content-Security-Policy header missing")
	}
}

func TestSetUploadFileHeadersDoesNotSandboxImages(t *testing.T) {
	res := httptest.NewRecorder()

	setUploadFileHeaders(res, "example.jpg")

	if got := res.Header().Get("Content-Security-Policy"); got != "" {
		t.Fatalf("Content-Security-Policy = %q, want empty", got)
	}
}
