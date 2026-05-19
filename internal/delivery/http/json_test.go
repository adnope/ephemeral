package httpdelivery

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

func TestItemToResponseFileURLs(t *testing.T) {
	createdAt := time.Date(2026, 5, 16, 12, 30, 0, 0, time.UTC)
	item := &domain.Item{
		ID:       42,
		Type:     domain.ItemTypeVideo,
		Content:  "uploads/sample video.mp4",
		Filename: "sample video.mp4",
		Filesize: 2048,
		Metadata: domain.Metadata{
			Width:  640,
			Height: 480,
			MIME:   "video/mp4",
			Thumb:  "thumbs/sample video.jpg",
		},
		CreatedAt: createdAt,
	}

	got := itemToResponse(item)

	if got.ID != 42 || got.Type != domain.ItemTypeVideo {
		t.Fatalf("unexpected identity fields: %#v", got)
	}
	if got.Text != "" {
		t.Fatalf("file item text = %q, want empty", got.Text)
	}
	if got.ContentURL != "/api/files/uploads%2Fsample%20video.mp4" {
		t.Fatalf("ContentURL = %q", got.ContentURL)
	}
	if got.DownloadURL != got.ContentURL {
		t.Fatalf("DownloadURL = %q, want %q", got.DownloadURL, got.ContentURL)
	}
	if got.Metadata.ThumbnailURL != "/api/files/thumbs%2Fsample%20video.jpg" {
		t.Fatalf("ThumbnailURL = %q", got.Metadata.ThumbnailURL)
	}
	if got.CreatedAtEpochMillis != createdAt.UnixMilli() {
		t.Fatalf("CreatedAtEpochMillis = %d, want %d", got.CreatedAtEpochMillis, createdAt.UnixMilli())
	}
}

func TestItemToResponseText(t *testing.T) {
	item := &domain.Item{
		ID:      7,
		Type:    domain.ItemTypeText,
		Content: "hello",
	}

	got := itemToResponse(item)

	if got.Text != "hello" {
		t.Fatalf("Text = %q, want hello", got.Text)
	}
	if got.ContentURL != "" || got.DownloadURL != "" {
		t.Fatalf("text item URLs should be empty: %#v", got)
	}
}

func TestDecodeJSONRejectsBodyAboveLimit(t *testing.T) {
	var reqBody struct {
		Text string `json:"text"`
	}

	body := bytes.NewBufferString(`{"text":"`)
	body.Write(bytes.Repeat([]byte("a"), maxJSONBodyBytes))
	body.WriteString(`"}`)

	req := httptest.NewRequest(http.MethodPost, "/api/message", body)
	res := httptest.NewRecorder()

	err := decodeJSON(res, req, &reqBody)
	if !errors.Is(err, errJSONBodyTooLarge) {
		t.Fatalf("decodeJSON error = %v, want errJSONBodyTooLarge", err)
	}
}

func TestBoundedUploadConcurrency(t *testing.T) {
	tests := []struct {
		name string
		raw  int
		want int
	}{
		{name: "default minimum", raw: 0, want: 1},
		{name: "configured value", raw: 5, want: 5},
		{name: "maximum cap", raw: 11, want: 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := boundedUploadConcurrency(tt.raw); got != tt.want {
				t.Fatalf("boundedUploadConcurrency(%d) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}
