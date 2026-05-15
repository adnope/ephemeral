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
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	if err := h.items.DeleteItem(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidInput):
			http.Error(w, "invalid item id", http.StatusBadRequest)
		case errors.Is(err, usecase.ErrNotFound):
			http.Error(w, "item not found", http.StatusNotFound)
		default:
			h.log.Error("delete item: usecase", "item_id", id, "err", err)
			http.Error(w, "delete failed", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
