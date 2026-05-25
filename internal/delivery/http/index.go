package httpdelivery

import (
	"net/http"
	"strconv"
)

// Items handles GET /api/items.
func (h *Handler) Items(w http.ResponseWriter, r *http.Request) {
	cursor, err := parseCursor(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid cursor")
		return
	}

	page, err := h.items.List(r.Context(), cursor, h.settings.ChatPageSize)
	if err != nil {
		h.log.Error("items: list", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, pageToResponse(page.Items, page.NextCursor))
}

// Index handles GET /.
func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)

	page, err := h.items.List(r.Context(), cursor, h.settings.ChatPageSize)
	if err != nil {
		h.log.Error("index: list items", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	items, err := h.itemTemplateData(r.Context(), page.Items)
	if err != nil {
		h.log.Error("index: public link state", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Items":             items,
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

func parseCursor(r *http.Request) (int64, error) {
	raw := r.URL.Query().Get("cursor")
	if raw == "" {
		return 0, nil
	}
	cursor, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || cursor < 0 {
		return 0, strconv.ErrSyntax
	}
	return cursor, nil
}
