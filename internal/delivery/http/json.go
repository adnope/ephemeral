package httpdelivery

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

type jsonErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

const maxJSONBodyBytes = 64 << 10

var errJSONBodyTooLarge = errors.New("json body too large")

type itemMetadataResponse struct {
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Duration     string `json:"duration"`
	MIME         string `json:"mime"`
	ThumbnailURL string `json:"thumbnailUrl"`
}

type itemResponse struct {
	ID                   int64                `json:"id"`
	Type                 string               `json:"type"`
	Text                 string               `json:"text"`
	Filename             string               `json:"filename"`
	FilesizeBytes        int64                `json:"filesizeBytes"`
	ContentURL           string               `json:"contentUrl"`
	DownloadURL          string               `json:"downloadUrl"`
	CreatedAtEpochMillis int64                `json:"createdAtEpochMillis"`
	Metadata             itemMetadataResponse `json:"metadata"`
}

type pageResponse struct {
	Items      []itemResponse `json:"items"`
	NextCursor int64          `json:"nextCursor"`
	HasMore    bool           `json:"hasMore"`
}

func wantsJSON(r *http.Request) bool {
	return acceptsJSON(r) || hasJSONContentType(r)
}

func acceptsJSON(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Accept")), "application/json")
}

func hasJSONContentType(r *http.Request) bool {
	contentType := strings.ToLower(r.Header.Get("Content-Type"))
	if i := strings.IndexByte(contentType, ';'); i >= 0 {
		contentType = contentType[:i]
	}
	return strings.TrimSpace(contentType) == "application/json"
}

func decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return errJSONBodyTooLarge
		}
		return err
	}

	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return errJSONBodyTooLarge
		}
		return err
	}

	return nil
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if data == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, jsonErrorResponse{
		Code:    code,
		Message: message,
	})
}

func pageToResponse(items []*domain.Item, nextCursor int64) pageResponse {
	responses := make([]itemResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, itemToResponse(item))
	}
	return pageResponse{
		Items:      responses,
		NextCursor: nextCursor,
		HasMore:    nextCursor != 0,
	}
}

func itemToResponse(item *domain.Item) itemResponse {
	if item == nil {
		return itemResponse{}
	}

	var text string
	var contentURL string
	var downloadURL string
	if item.Type == domain.ItemTypeText {
		text = item.Content
	} else if item.Content != "" {
		contentURL = apiFileURL(item.Content)
		downloadURL = contentURL
	}

	var thumbnailURL string
	if item.Metadata.Thumb != "" {
		thumbnailURL = apiFileURL(item.Metadata.Thumb)
	}

	return itemResponse{
		ID:                   item.ID,
		Type:                 item.Type,
		Text:                 text,
		Filename:             item.Filename,
		FilesizeBytes:        item.Filesize,
		ContentURL:           contentURL,
		DownloadURL:          downloadURL,
		CreatedAtEpochMillis: unixMillis(item.CreatedAt),
		Metadata: itemMetadataResponse{
			Width:        item.Metadata.Width,
			Height:       item.Metadata.Height,
			Duration:     item.Metadata.Duration,
			MIME:         item.Metadata.MIME,
			ThumbnailURL: thumbnailURL,
		},
	}
}

func apiFileURL(name string) string {
	return "/api/files/" + url.PathEscape(name)
}

func unixMillis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixNano() / int64(time.Millisecond)
}
