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

func TestSetUploadFileHeadersSetsHLSContentTypes(t *testing.T) {
	t.Run("playlist", func(t *testing.T) {
		res := httptest.NewRecorder()

		setUploadFileHeaders(res, "hls/sample/index.m3u8")

		if got := res.Header().Get("Content-Type"); got != "application/vnd.apple.mpegurl" {
			t.Fatalf("Content-Type = %q, want application/vnd.apple.mpegurl", got)
		}
	})

	t.Run("segment", func(t *testing.T) {
		res := httptest.NewRecorder()

		setUploadFileHeaders(res, "hls/sample/segment_00000.ts")

		if got := res.Header().Get("Content-Type"); got != "video/mp2t" {
			t.Fatalf("Content-Type = %q, want video/mp2t", got)
		}
	})
}
