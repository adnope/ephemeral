package httpdelivery

import (
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ServeFile handles GET /api/files/{path}.
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request) {
	relPath := chi.URLParam(r, "*")

	decodedPath, err := url.PathUnescape(relPath)
	if err != nil {
		http.Error(w, "bad file path", http.StatusBadRequest)
		return
	}

	absPath, err := h.items.ResolveUploadPath(decodedPath)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	setUploadFileHeaders(w, decodedPath)
	http.ServeFile(w, r, absPath)
}

func setUploadFileHeaders(w http.ResponseWriter, relPath string) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if isActiveUploadDocument(relPath) {
		w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; img-src 'self' data: blob:; media-src 'self' blob:; style-src 'unsafe-inline'")
	}
}

func isActiveUploadDocument(relPath string) bool {
	switch strings.ToLower(filepath.Ext(relPath)) {
	case ".htm", ".html", ".svg", ".xml", ".xhtml":
		return true
	default:
		return false
	}
}
