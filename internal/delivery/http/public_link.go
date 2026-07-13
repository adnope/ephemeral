package httpdelivery

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/adnope/ephemeral/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type publicLinkRequest struct {
	ExpiresInSeconds *int64 `json:"expires_in_seconds"`
}

type publicLinkResponse struct {
	URL       string     `json:"url"`
	Token     string     `json:"token"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type publicLinkStatusResponse struct {
	Status    string     `json:"status"`
	URL       string     `json:"url,omitempty"`
	Token     string     `json:"token,omitempty"`
	ExpiresAt *time.Time `json:"expires_at"`
}

type publicShareResponse struct {
	Filename      string     `json:"filename"`
	FilesizeBytes int64      `json:"filesizeBytes"`
	ItemType      string     `json:"itemType"`
	MIME          string     `json:"mime"`
	SourceURL     string     `json:"sourceUrl"`
	PosterURL     string     `json:"posterUrl"`
	DownloadURL   string     `json:"downloadUrl"`
	ExpiresAt     *time.Time `json:"expiresAt"`
	Processing    bool       `json:"processing"`
}

// PublicLinkStatus handles GET /api/items/{id}/public-link.
func (h *Handler) PublicLinkStatus(w http.ResponseWriter, r *http.Request) {
	itemID, ok := parseItemIDParam(w, r)
	if !ok {
		return
	}

	status, err := h.items.PublicLinkStatus(r.Context(), itemID)
	if err != nil {
		h.writePublicLinkUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, publicLinkStatusResponse{
		Status:    string(status.State),
		URL:       status.URL,
		Token:     status.Token,
		ExpiresAt: status.ExpiresAt,
	})
}

// CreatePublicLink handles POST /api/items/{id}/public-link.
func (h *Handler) CreatePublicLink(w http.ResponseWriter, r *http.Request) {
	itemID, ok := parseItemIDParam(w, r)
	if !ok {
		return
	}

	var req publicLinkRequest
	if err := decodeJSON(w, r, &req); err != nil {
		if errors.Is(err, errJSONBodyTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "JSON body too large")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid JSON body")
		return
	}

	expiresIn, ok := publicLinkExpiresIn(w, req.ExpiresInSeconds)
	if !ok {
		return
	}

	link, err := h.items.CreatePublicLink(r.Context(), itemID, expiresIn)
	if err != nil {
		h.writePublicLinkUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, publicLinkResponse{
		URL:       link.URL,
		Token:     link.Token,
		ExpiresAt: link.ExpiresAt,
	})
}

// RevokePublicLink handles DELETE /api/items/{id}/public-link.
func (h *Handler) RevokePublicLink(w http.ResponseWriter, r *http.Request) {
	itemID, ok := parseItemIDParam(w, r)
	if !ok {
		return
	}

	if err := h.items.RevokePublicLink(r.Context(), itemID); err != nil {
		h.writePublicLinkUseCaseError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// PublicShareAPI handles GET /api/share/{token}.
func (h *Handler) PublicShareAPI(w http.ResponseWriter, r *http.Request) {
	view, err := h.items.PublicShareView(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrUnsupportedShare):
			writeJSONError(w, http.StatusUnsupportedMediaType, "unsupported_share", "shared file must be downloaded")
		case errors.Is(err, usecase.ErrNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "public link not found")
		default:
			h.log.Error("public share api: resolve", "err", err)
			writeJSONError(w, http.StatusInternalServerError, "server_error", "public share could not be loaded")
		}
		return
	}

	writeJSON(w, http.StatusOK, publicShareResponse{
		Filename:      view.Item.Filename,
		FilesizeBytes: view.Item.Filesize,
		ItemType:      view.Item.Type,
		MIME:          view.DisplayMIME,
		SourceURL:     view.SourceURL,
		PosterURL:     view.PosterURL,
		DownloadURL:   view.DownloadURL,
		ExpiresAt:     view.ExpiresAt,
		Processing:    view.Item.Metadata.Processing,
	})
}

// PublicShareFile handles GET /share/{token}/file.
func (h *Handler) PublicShareFile(w http.ResponseWriter, r *http.Request) {
	h.servePublicSharedFile(w, r, chi.URLParam(r, "token"), "display")
}

// PublicShareDownload handles GET /share/{token}/download.
func (h *Handler) PublicShareDownload(w http.ResponseWriter, r *http.Request) {
	h.servePublicSharedFile(w, r, chi.URLParam(r, "token"), "download")
}

// PublicShareThumb handles GET /share/{token}/thumb.
func (h *Handler) PublicShareThumb(w http.ResponseWriter, r *http.Request) {
	h.servePublicSharedFile(w, r, chi.URLParam(r, "token"), "thumb")
}

func (h *Handler) servePublicSharedFile(w http.ResponseWriter, r *http.Request, token string, variant string) {
	file, err := h.items.PublicSharedFile(r.Context(), token, variant)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrForbidden):
			http.Error(w, "forbidden", http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
		return
	}

	setPublicShareFileHeaders(w, file)
	http.ServeFile(w, r, file.Path)
}

func parseItemIDParam(w http.ResponseWriter, r *http.Request) (int64, bool) {
	rawID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid item id")
		return 0, false
	}
	return id, true
}

func publicLinkExpiresIn(w http.ResponseWriter, seconds *int64) (*time.Duration, bool) {
	if seconds == nil {
		return nil, true
	}

	maxSeconds := int64((10 * 365 * 24 * time.Hour) / time.Second)
	if *seconds <= 0 || *seconds > maxSeconds {
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid expiry")
		return nil, false
	}

	duration := time.Duration(*seconds) * time.Second
	return &duration, true
}

func (h *Handler) writePublicLinkUseCaseError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, usecase.ErrInvalidInput):
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid public link request")
	case errors.Is(err, usecase.ErrUnsupportedShare):
		writeJSONError(w, http.StatusUnsupportedMediaType, "unsupported_share", "item cannot be shared as a public file link")
	case errors.Is(err, usecase.ErrNotFound):
		writeJSONError(w, http.StatusNotFound, "not_found", "item not found")
	default:
		h.log.Error("public link: usecase", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "server_error", "public link operation failed")
	}
}

func setPublicShareFileHeaders(w http.ResponseWriter, file usecase.PublicSharedFile) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if file.MIME != "" {
		w.Header().Set("Content-Type", file.MIME)
	}

	disposition := "attachment"
	if file.Inline {
		disposition = "inline"
	}
	if value := mime.FormatMediaType(disposition, map[string]string{"filename": file.Filename}); value != "" {
		w.Header().Set("Content-Disposition", value)
	}

	if file.Inline && isActiveUploadDocument(file.RelPath) {
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self' data: blob:; media-src 'self' blob:; style-src 'unsafe-inline'")
	}
}
