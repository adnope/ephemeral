package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/sse"
	"github.com/adnope/ephemeral/internal/store"
)

// POST /message
func (h *Handler) Message(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	text := strings.TrimSpace(r.FormValue("text"))
	if text == "" {
		http.Error(w, "empty message", http.StatusBadRequest)
		return
	}

	item := &store.Item{
		Type:    "text",
		Content: text,
	}

	id, err := h.store.Create(r.Context(), item)
	if err != nil {
		h.log.Error("message: db create", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.broker.Broadcast(sse.Event{Type: "item:new", ID: id})

	item.ID = id
	item.CreatedAt = time.Now().UTC()
	if err := h.tmpl.ExecuteTemplate(w, "item_partial", item); err != nil {
		h.log.Error("message: render", "err", err)
	}
}
