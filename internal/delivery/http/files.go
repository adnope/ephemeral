package httpdelivery

import (
	"net/http"
	"net/url"

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

	http.ServeFile(w, r, absPath)
}
