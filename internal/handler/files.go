package handler

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// ServeFile handles file downloads with path traversal protection.
// GET /files/{path}
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request) {
	relPath := chi.URLParam(r, "*")

	// Security: reject any path traversal attempt
	cleanPath := filepath.Clean(relPath)
	if strings.Contains(cleanPath, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	absPath := filepath.Join(h.dataDir, "uploads", cleanPath)

	// http.ServeFile uses sendfile(2) syscall on Linux: data moves
	// disk -> NIC via kernel, bypassing user space entirely.
	http.ServeFile(w, r, absPath)
}
