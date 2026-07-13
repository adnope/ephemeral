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
		if err := decodeJSON(w, r, &req); err != nil {
			if errors.Is(err, errJSONBodyTooLarge) {
				writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "JSON body too large")
				return
			}
			writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid JSON body")
			return
		}
		text = req.Text
	} else {
		if err := r.ParseForm(); err != nil {
			writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid form")
			return
		}
		text = r.FormValue("text")
	}

	item, err := h.items.CreateMessage(r.Context(), text)
	if err != nil {
		if errors.Is(err, usecase.ErrEmptyMessage) {
			writeJSONError(w, http.StatusBadRequest, "validation_error", "empty message")
			return
		}
		h.log.Error("message: create", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, itemToResponse(item))
}
