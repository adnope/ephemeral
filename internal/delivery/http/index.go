package httpdelivery

import (
	"net/http"
	"strconv"

	"github.com/adnope/ephemeral/internal/domain"
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
	activePublicLinks, err := h.items.ActivePublicLinkItemIDs(r.Context(), page.Items)
	if err != nil {
		h.log.Error("items: list active public links", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, pageToResponse(page.Items, page.NextCursor, activePublicLinks))
}

// Item handles GET /api/items/{id}.
func (h *Handler) Item(w http.ResponseWriter, r *http.Request) {
	itemID, ok := parseItemIDParam(w, r)
	if !ok {
		return
	}

	item, err := h.items.GetItem(r.Context(), itemID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "not_found", "item not found")
		return
	}
	activePublicLinks, err := h.items.ActivePublicLinkItemIDs(r.Context(), []*domain.Item{item})
	if err != nil {
		h.log.Error("item: list active public links", "err", err)
		writeJSONError(w, http.StatusInternalServerError, "server_error", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, itemToResponseWithPublicLink(item, activePublicLinks[item.ID]))
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
