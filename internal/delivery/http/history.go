package httpdelivery

import (
	"errors"
	"net/http"
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
	activePublicLinks, err := h.items.ActivePublicLinkItemIDs(r.Context(), result.Items)
	if err != nil {
		h.log.Error("history api: list active public links", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, pageToResponse(result.Items, result.NextCursor, activePublicLinks))
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
