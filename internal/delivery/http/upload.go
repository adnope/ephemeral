package httpdelivery

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
)

// Upload handles POST /api/upload.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	releaseUploadSlot, err := h.acquireUploadSlot(r.Context())
	if err != nil {
		return
	}
	defer releaseUploadSlot()

	r.Body = http.MaxBytesReader(w, r.Body, h.settings.MaxUploadBytes)

	reader, err := r.MultipartReader()
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid multipart form")
		return
	}

	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "file too large")
				return
			}
			writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid multipart data")
			return
		}

		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}

		originalName := filepath.Base(part.FileName())
		if originalName == "." || originalName == "" {
			_ = part.Close()
			writeJSONError(w, http.StatusBadRequest, "validation_error", "missing filename")
			return
		}

		item, uploadErr := h.items.UploadFile(r.Context(), originalName, part)
		_ = part.Close()
		if uploadErr != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(uploadErr, &maxBytesErr) {
				writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "file too large")
				return
			}
			h.log.Error("upload: create item", "err", uploadErr)
			writeJSONError(w, http.StatusInternalServerError, "server_error", "upload failed")
			return
		}

		writeJSON(w, http.StatusOK, itemToResponse(item))
		return
	}

	writeJSONError(w, http.StatusBadRequest, "validation_error", "missing file field")
}
