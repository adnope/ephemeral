package httpdelivery

import (
	"errors"
	"net/http"

	"github.com/adnope/ephemeral/internal/usecase"
)

// Message handles POST /api/message.
func (h *Handler) Message(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	item, err := h.items.CreateMessage(r.Context(), r.FormValue("text"))
	if err != nil {
		if errors.Is(err, usecase.ErrEmptyMessage) {
			http.Error(w, "empty message", http.StatusBadRequest)
			return
		}
		h.log.Error("message: create", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.tmpl.ExecuteTemplate(w, "item_partial", item); err != nil {
		h.log.Error("message: render", "err", err)
	}
}
