package httpdelivery

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/adnope/ephemeral/internal/usecase"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		if wantsJSON(r) {
			writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid item id")
		} else {
			http.Error(w, "invalid item id", http.StatusBadRequest)
		}
		return
	}

	if err := h.items.DeleteItem(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidInput):
			if wantsJSON(r) {
				writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid item id")
			} else {
				http.Error(w, "invalid item id", http.StatusBadRequest)
			}
		case errors.Is(err, usecase.ErrNotFound):
			if wantsJSON(r) {
				writeJSONError(w, http.StatusNotFound, "not_found", "item not found")
			} else {
				http.Error(w, "item not found", http.StatusNotFound)
			}
		default:
			h.log.Error("delete item: usecase", "item_id", id, "err", err)
			if wantsJSON(r) {
				writeJSONError(w, http.StatusInternalServerError, "server_error", "delete failed")
			} else {
				http.Error(w, "delete failed", http.StatusInternalServerError)
			}
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
