package handler

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/bodyindex"
)

const (
	historyPageSize   = 50
	historyDateLayout = "2006-01-02"
)

type historyFilters struct {
	Types       []string
	Query       string
	SearchBody  bool
	DateFrom    time.Time
	DateTo      time.Time
	Recent      string
	HasDateFrom bool
	HasDateTo   bool
}

// History serves the media gallery view.
// GET /history
func (h *Handler) History(w http.ResponseWriter, r *http.Request) {
	cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
	if cursor == 0 {
		cursor = math.MaxInt64
	}

	var types []string
	if t := r.URL.Query().Get("type"); t != "" {
		types = strings.Split(t, ",")
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	searchBody := r.URL.Query().Get("body") == "1"
	dateFromRaw := strings.TrimSpace(r.URL.Query().Get("from"))
	dateToRaw := strings.TrimSpace(r.URL.Query().Get("to"))
	recent := strings.TrimSpace(r.URL.Query().Get("recent"))

	filters, err := parseHistoryFilters(
		types,
		query,
		searchBody,
		dateFromRaw,
		dateToRaw,
		recent,
	)
	if err != nil {
		http.Error(w, "invalid date filter", http.StatusBadRequest)
		return
	}

	items, err := h.bodyIndex.SearchHistory(r.Context(), bodyindex.SearchOptions{
		Types:       filters.Types,
		Cursor:      cursor,
		Limit:       historyPageSize,
		Query:       filters.Query,
		SearchBody:  filters.SearchBody,
		DateFrom:    filters.DateFrom,
		DateTo:      filters.DateTo,
		HasDateFrom: filters.HasDateFrom,
		HasDateTo:   filters.HasDateTo,
	})
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
		"Query":      query,
		"SearchBody": searchBody,
		"DateFrom":   dateFromRaw,
		"DateTo":     dateToRaw,
		"Recent":     recent,
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

func parseHistoryFilters(
	types []string,
	query string,
	searchBody bool,
	dateFromRaw string,
	dateToRaw string,
	recent string,
) (historyFilters, error) {
	f := historyFilters{
		Types:      types,
		Query:      query,
		SearchBody: searchBody,
		Recent:     recent,
	}

	if dateFromRaw != "" {
		t, err := time.ParseInLocation(historyDateLayout, dateFromRaw, time.Local)
		if err != nil {
			return f, fmt.Errorf("parse from date: %w", err)
		}
		f.DateFrom = t
		f.HasDateFrom = true
	}

	if dateToRaw != "" {
		t, err := time.ParseInLocation(historyDateLayout, dateToRaw, time.Local)
		if err != nil {
			return f, fmt.Errorf("parse to date: %w", err)
		}
		f.DateTo = t.Add(24*time.Hour - time.Nanosecond)
		f.HasDateTo = true
	}

	if recent != "" {
		from, ok := recentCutoff(recent, time.Now())
		if ok && (!f.HasDateFrom || from.After(f.DateFrom)) {
			f.DateFrom = from
			f.HasDateFrom = true
		}
	}

	return f, nil
}

func recentCutoff(value string, now time.Time) (time.Time, bool) {
	switch value {
	case "1d":
		return now.AddDate(0, 0, -1), true
	case "7d":
		return now.AddDate(0, 0, -7), true
	case "14d":
		return now.AddDate(0, 0, -14), true
	case "30d":
		return now.AddDate(0, 0, -30), true
	case "90d":
		return now.AddDate(0, 0, -90), true
	case "6mo":
		return now.AddDate(0, -6, 0), true
	case "1y":
		return now.AddDate(-1, 0, 0), true
	default:
		return time.Time{}, false
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
