package handler

import "net/http"

// Events proxies to the SSE broker's ServeSSE handler.
// GET /events
func (h *Handler) Events(w http.ResponseWriter, r *http.Request) {
	h.broker.ServeSSE(w, r)
}
