package handler

import (
	"errors"
	"net/http"
	"os"
	"strconv"

	"github.com/adnope/ephemeral/internal/sse"
	"github.com/go-chi/chi/v5"
)

func (h *Handler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	rawID := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "invalid item id", http.StatusBadRequest)
		return
	}

	item, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "item not found", http.StatusNotFound)
		return
	}

	if err := h.store.Delete(r.Context(), id); err != nil {
		h.log.Error("delete item: db delete", "item_id", id, "err", err)
		http.Error(w, "delete failed", http.StatusInternalServerError)
		return
	}

	if item.Type != "text" {
		h.removeUploadedFileBestEffort(item.Content)
		if item.Metadata.Thumb != "" {
			h.removeUploadedFileBestEffort(item.Metadata.Thumb)
		}
	}

	h.broker.Broadcast(sse.Event{Type: "item:deleted", ID: id})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) removeUploadedFileBestEffort(content string) {
	path, err := h.safeUploadPath(content)
	if err != nil {
		h.log.Warn("delete item: unsafe file path", "content", content, "err", err)
		return
	}

	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		h.log.Warn("delete item: remove file", "path", path, "err", err)
	}
}
