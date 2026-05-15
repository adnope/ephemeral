package httpdelivery

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
)

// Upload handles POST /api/upload.
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.settings.MaxUploadBytes)

	reader, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
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
				http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid multipart data", http.StatusBadRequest)
			return
		}

		if part.FormName() != "file" {
			_ = part.Close()
			continue
		}

		originalName := filepath.Base(part.FileName())
		if originalName == "." || originalName == "" {
			_ = part.Close()
			http.Error(w, "missing filename", http.StatusBadRequest)
			return
		}

		item, uploadErr := h.items.UploadFile(r.Context(), originalName, part)
		_ = part.Close()
		if uploadErr != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(uploadErr, &maxBytesErr) {
				http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
				return
			}
			h.log.Error("upload: create item", "err", uploadErr)
			http.Error(w, "upload failed", http.StatusInternalServerError)
			return
		}

		if err := h.tmpl.ExecuteTemplate(w, "item_partial", item); err != nil {
			h.log.Error("upload: render", "err", err)
		}
		return
	}

	http.Error(w, "missing file field", http.StatusBadRequest)
}
