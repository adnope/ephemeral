package httpdelivery

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/adnope/ephemeral/internal/usecase"
	"github.com/go-chi/chi/v5"
)

// EditMessage handles PATCH /api/items/{id} for text items.
func (h *Handler) EditMessage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid item id")
		return
	}
	if !hasJSONContentType(r) {
		writeJSONError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
		return
	}
	var req messageRequest
	if err := decodeJSON(w, r, &req); err != nil {
		if errors.Is(err, errJSONBodyTooLarge) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "JSON body too large")
			return
		}
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid JSON body")
		return
	}
	item, err := h.items.EditMessage(r.Context(), id, req.Text)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidInput), errors.Is(err, usecase.ErrEmptyMessage):
			writeJSONError(w, http.StatusBadRequest, "validation_error", "text must not be empty")
		case errors.Is(err, usecase.ErrNotFound):
			writeJSONError(w, http.StatusNotFound, "not_found", "item not found")
		case errors.Is(err, usecase.ErrUnsupportedEdit):
			writeJSONError(w, http.StatusUnsupportedMediaType, "unsupported_edit", "only text items can be edited")
		default:
			h.log.Error("edit message", "item_id", id, "err", err)
			writeJSONError(w, http.StatusInternalServerError, "server_error", "edit failed")
		}
		return
	}
	writeJSON(w, http.StatusOK, itemToResponse(item))
}
