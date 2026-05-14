package handler

import "net/http"

// GET /events
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	h.broker.ServeSSE(w, r)
}
