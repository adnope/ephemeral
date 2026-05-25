package httpdelivery

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adnope/ephemeral/internal/usecase"
)

func TestSetPublicShareFileHeadersUsesAttachmentForDownloads(t *testing.T) {
	res := httptest.NewRecorder()

	setPublicShareFileHeaders(res, usecase.PublicSharedFile{
		Filename: "report.pdf",
		MIME:     "application/pdf",
		Inline:   false,
	})

	if got := res.Header().Get("Content-Type"); got != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", got)
	}
	if got := res.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment;") {
		t.Fatalf("Content-Disposition = %q, want attachment", got)
	}
	if got := res.Header().Get("Cache-Control"); got != "private, no-store" {
		t.Fatalf("Cache-Control = %q, want private, no-store", got)
	}
}

func TestSetPublicShareFileHeadersSandboxesInlineActiveDocuments(t *testing.T) {
	res := httptest.NewRecorder()

	setPublicShareFileHeaders(res, usecase.PublicSharedFile{
		RelPath:  "diagram.svg",
		Filename: "diagram.svg",
		MIME:     "image/svg+xml",
		Inline:   true,
	})

	if got := res.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "inline;") {
		t.Fatalf("Content-Disposition = %q, want inline", got)
	}
	if got := res.Header().Get("Content-Security-Policy"); got == "" {
		t.Fatal("Content-Security-Policy header missing")
	}
}
