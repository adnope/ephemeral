package handler

import (
	"math"
	"net/http"
	"strconv"

	"github.com/adnope/ephemeral/internal/store"
)

const chatPageSize = 100

// Index serves the main chat stream page.
// GET /
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	if cursor == 0 {
		cursor = math.MaxInt64
	}

	items, err := h.store.List(r.Context(), store.ListFilter{
		Cursor: cursor,
		Limit:  chatPageSize,
	})
	if err != nil {
		h.log.Error("index: list items", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Determine next cursor for infinite scroll
	var nextCursor int64
	if len(items) == chatPageSize {
		nextCursor = items[len(items)-1].ID
	}

	data := map[string]interface{}{
		"Items":      items,
		"NextCursor": nextCursor,
	}

	// If this is an HTMX request for more items, render only the partial
	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, "items_partial", data); err != nil {
			h.log.Error("index: render partial", "err", err)
		}
		return
	}

	if err := h.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		h.log.Error("index: render", "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}
