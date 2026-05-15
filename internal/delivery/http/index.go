package httpdelivery

import (
	"net/http"
	"strconv"
)

// Index handles GET /.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)

	page, err := h.items.List(r.Context(), cursor, h.settings.ChatPageSize)
	if err != nil {
		h.log.Error("index: list items", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Items":             page.Items,
		"NextCursor":        page.NextCursor,
		"UploadConcurrency": h.settings.UploadConcurrency,
	}

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
