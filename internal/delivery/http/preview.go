package httpdelivery

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/adnope/ephemeral/internal/usecase"
	"github.com/go-chi/chi/v5"
)

type filePreviewResponse struct {
	ID          int64  `json:"id"`
	Filename    string `json:"filename"`
	MIME        string `json:"mime"`
	Language    string `json:"language"`
	Content     string `json:"content"`
	Filesize    int64  `json:"filesize"`
	CreatedAt   string `json:"created_at"`
	DownloadURL string `json:"download_url"`
}

func (h *Handler) PreviewFile(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid item id")
		return
	}

	preview, err := h.items.PreviewFile(r.Context(), id, h.settings.TextPreviewMaxBytes)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidInput):
			writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid item id")
		case errors.Is(err, usecase.ErrNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "file not found")
		case errors.Is(err, usecase.ErrForbidden):
			writeJSONError(w, http.StatusForbidden, "forbidden", "forbidden")
		case errors.Is(err, usecase.ErrPreviewTooLarge):
			writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "file too large for preview")
		case errors.Is(err, usecase.ErrUnsupportedPreview):
			writeJSONError(w, http.StatusUnsupportedMediaType, "unsupported_preview", "file type is not previewable as text")
		default:
			h.log.Error("preview: usecase", "item_id", id, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "server_error", "preview failed")
		}
		return
	}

	response := filePreviewResponse{
		ID:          preview.ID,
		Filename:    preview.Filename,
		MIME:        preview.MIME,
		Language:    preview.Language,
		Content:     preview.Content,
		Filesize:    preview.Filesize,
		CreatedAt:   preview.CreatedAt.Format("Jan 2, 2006 3:04 PM"),
		DownloadURL: preview.DownloadURL,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("preview: encode response", "item_id", id, "err", err)
	}
}
