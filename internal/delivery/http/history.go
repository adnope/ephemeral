package httpdelivery

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/adnope/ephemeral/internal/usecase"
)

// HistoryAPI handles GET /api/history.
func (h *Handler) HistoryAPI(w http.ResponseWriter, r *http.Request) {
	cursor, err := parseCursor(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid cursor")
		return
	}

	result, err := h.searchHistory(r, cursor)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidInput) {
			writeJSONError(w, http.StatusBadRequest, "validation_error", "invalid history filter")
			return
		}
		h.log.Error("history api: query", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, pageToResponse(result.Items, result.NextCursor))
}

// History handles GET /history.
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)

	result, err := h.searchHistory(r, cursor)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidInput) {
			http.Error(w, "invalid history filter", http.StatusBadRequest)
			return
		}
		h.log.Error("history: query", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	items, err := h.itemTemplateData(r.Context(), result.Items)
	if err != nil {
		h.log.Error("history: public link state", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Items":      items,
		"NextCursor": result.NextCursor,
		"TypeFilter": strings.Join(result.Types, ","),
		"Query":      result.Query,
		"SearchBody": result.SearchBody,
		"Visibility": result.Visibility,
		"DateFrom":   result.DateFrom,
		"DateTo":     result.DateTo,
		"Recent":     result.Recent,
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

func (h *Handler) searchHistory(r *http.Request, cursor int64) (usecase.HistoryResult, error) {
	var types []string
	if rawTypes := r.URL.Query().Get("type"); rawTypes != "" {
		types = strings.Split(rawTypes, ",")
	}

	return h.history.Search(r.Context(), usecase.HistoryQuery{
		Cursor:      cursor,
		Types:       types,
		Query:       r.URL.Query().Get("q"),
		SearchBody:  r.URL.Query().Get("body") == "1",
		Visibility:  r.URL.Query().Get("visibility"),
		DateFromRaw: r.URL.Query().Get("from"),
		DateToRaw:   r.URL.Query().Get("to"),
		Recent:      r.URL.Query().Get("recent"),
		Limit:       h.settings.HistoryPageSize,
	})
}

// SearchItems handles GET /search?q=query.
func (h *Handler) SearchItems(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	items, err := h.items.SearchItems(r.Context(), query, h.settings.SearchResultLimit)
	if err != nil {
		if errors.Is(err, usecase.ErrEmptyQuery) {
			http.Error(w, "empty query", http.StatusBadRequest)
			return
		}
		h.log.Error("search: query", "err", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	templateItems, err := h.itemTemplateData(r.Context(), items)
	if err != nil {
		h.log.Error("search: public link state", "err", err)
		http.Error(w, "search failed", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Items": templateItems,
		"Query": query,
	}

	if err := h.tmpl.ExecuteTemplate(w, "items_partial", data); err != nil {
		h.log.Error("search: render", "err", err)
	}
}
