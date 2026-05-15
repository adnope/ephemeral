package httpdelivery

import "net/http"

// Events handles GET /api/events.
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	h.events.ServeSSE(w, r)
}
