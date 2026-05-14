package handler

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// GET /api/files/{path}
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request) {
	relPath := chi.URLParam(r, "*")

	decodedPath, err := url.PathUnescape(relPath)
	if err != nil {
		http.Error(w, "bad file path", http.StatusBadRequest)
		return
	}

	cleanPath := filepath.Clean(decodedPath)
	if cleanPath == "." || filepath.IsAbs(cleanPath) || strings.Contains(cleanPath, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	absPath := filepath.Join(h.dataDir, "uploads", cleanPath)
	http.ServeFile(w, r, absPath)
}
