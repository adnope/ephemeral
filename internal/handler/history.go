package handler

import (
	"math"
	"net/http"
	"strconv"
	"strings"
)

const historyPageSize = 100

// History serves the media gallery view.
// GET /history
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	if cursor == 0 {
		cursor = math.MaxInt64
	}

	// Parse type filter: ?type=image,video
	var types []string
	if t := r.URL.Query().Get("type"); t != "" {
		types = strings.Split(t, ",")
	}

	items, err := h.store.MediaHistory(r.Context(), types, cursor, historyPageSize)
	if err != nil {
		h.log.Error("history: query", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var nextCursor int64
	if len(items) == historyPageSize {
		nextCursor = items[len(items)-1].ID
	}

	data := map[string]interface{}{
		"Items":      items,
		"NextCursor": nextCursor,
		"TypeFilter": strings.Join(types, ","),
	}

	if r.Header.Get("HX-Request") == "true" {
		if err := h.tmpl.ExecuteTemplate(w, "history_items", data); err != nil {
			h.log.Error("history: render partial", "err", err)
		}
		return
	}

	if err := h.tmpl.ExecuteTemplate(w, "history.html", data); err != nil {
		h.log.Error("history: render", "err", err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// SearchItems handles the search endpoint.
// GET /search?q=query
func (h *Handler) SearchItems(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		http.Error(w, "empty query", http.StatusBadRequest)
		return
	}

	items, err := h.store.Search(r.Context(), q, 30)
	if err != nil {
		h.log.Error("search: query", "err", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Items": items,
		"Query": q,
	}

	if err := h.tmpl.ExecuteTemplate(w, "items_partial", data); err != nil {
		h.log.Error("search: render", "err", err)
	}
}
