package httpdelivery

import (
	"errors"
	"net/http"

	"github.com/adnope/ephemeral/internal/usecase"
)

type messageRequest struct {
	Text string `json:"text"`
}

// Message handles POST /api/message.
func (h *Handler) Message(w http.ResponseWriter, r *http.Request) {
	var text string
	if hasJSONContentType(r) {
		var req messageRequest
		if err := decodeJSON(r, &req); err != nil {
			writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid JSON body")
			return
		}
		text = req.Text
	} else {
		if err := r.ParseForm(); err != nil {
			if wantsJSON(r) {
				writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid form")
			} else {
				http.Error(w, "invalid form", http.StatusBadRequest)
			}
			return
		}
		text = r.FormValue("text")
	}

	item, err := h.items.CreateMessage(r.Context(), text)
	if err != nil {
		if errors.Is(err, usecase.ErrEmptyMessage) {
			if wantsJSON(r) {
				writeJSONError(w, http.StatusBadRequest, "validation_error", "empty message")
			} else {
				http.Error(w, "empty message", http.StatusBadRequest)
			}
			return
		}
		h.log.Error("message: create", "err", err)
		if wantsJSON(r) {
			writeJSONError(w, http.StatusInternalServerError, "server_error", "internal error")
		} else {
			http.Error(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	if wantsJSON(r) {
		writeJSON(w, http.StatusOK, itemToResponse(item))
		return
	}

	if err := h.tmpl.ExecuteTemplate(w, "item_partial", item); err != nil {
		h.log.Error("message: render", "err", err)
	}
}
