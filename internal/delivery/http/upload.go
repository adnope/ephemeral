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
		if wantsJSON(r) {
			writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid multipart form")
		} else {
			http.Error(w, "invalid multipart form", http.StatusBadRequest)
		}
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
				if wantsJSON(r) {
					writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "file too large")
				} else {
					http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
				}
				return
			}
			if wantsJSON(r) {
				writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid multipart data")
			} else {
				http.Error(w, "invalid multipart data", http.StatusBadRequest)
			}
			return
		}

		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}

		originalName := filepath.Base(part.FileName())
		if originalName == "." || originalName == "" {
			_ = part.Close()
			if wantsJSON(r) {
				writeJSONError(w, http.StatusBadRequest, "validation_error", "missing filename")
			} else {
				http.Error(w, "missing filename", http.StatusBadRequest)
			}
			return
		}

		item, uploadErr := h.items.UploadFile(r.Context(), originalName, part)
		_ = part.Close()
		if uploadErr != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(uploadErr, &maxBytesErr) {
				if wantsJSON(r) {
					writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "file too large")
				} else {
					http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
				}
				return
			}
			h.log.Error("upload: create item", "err", uploadErr)
			if wantsJSON(r) {
				writeJSONError(w, http.StatusInternalServerError, "server_error", "upload failed")
			} else {
				http.Error(w, "upload failed", http.StatusInternalServerError)
			}
			return
		}

		if wantsJSON(r) {
			writeJSON(w, http.StatusOK, itemToResponse(item))
			return
		}

		if err := h.tmpl.ExecuteTemplate(w, "item_partial", item); err != nil {
			h.log.Error("upload: render", "err", err)
		}
		return
	}

	if wantsJSON(r) {
		writeJSONError(w, http.StatusBadRequest, "validation_error", "missing file field")
	} else {
		http.Error(w, "missing file field", http.StatusBadRequest)
	}
}
