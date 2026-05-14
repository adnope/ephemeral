package handler

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/store"
)

const (
	historyPageSize           = 50
	historySearchScanBatch    = 200
	historySearchMaxScanned   = 5000
	historySearchMaxTextBytes = 2 << 20 // 2 MiB
	historyDateLayout         = "2006-01-02"
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

	filters, err := parseHistoryFilters(types, query, searchBody, dateFromRaw, dateToRaw, recent)
	if err != nil {
		http.Error(w, "invalid date filter", http.StatusBadRequest)
		return
	}

	var (
		items      []*store.Item
		nextCursor int64
	)

	hasAdvancedFilter := query != "" || filters.HasDateFrom || filters.HasDateTo

	if hasAdvancedFilter {
		items, nextCursor, err = h.filteredHistory(r.Context(), cursor, filters)
	} else {
		items, err = h.store.MediaHistory(r.Context(), types, cursor, historyPageSize)
		if err == nil && len(items) == historyPageSize {
			nextCursor = items[len(items)-1].ID
		}
	}

	if err != nil {
		h.log.Error("history: query", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
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

func (h *Handler) filteredHistory(
	ctx context.Context,
	cursor int64,
	f historyFilters,
) ([]*store.Item, int64, error) {
	matches := make([]*store.Item, 0, historyPageSize)
	scanCursor := cursor
	scanned := 0

	for len(matches) < historyPageSize && scanned < historySearchMaxScanned {
		batch, err := h.store.MediaHistory(ctx, f.Types, scanCursor, historySearchScanBatch)
		if err != nil {
			return nil, 0, err
		}
		if len(batch) == 0 {
			return matches, 0, nil
		}

		for _, item := range batch {
			scanned++
			scanCursor = item.ID

			if h.historyItemMatchesFilters(item, f) {
				matches = append(matches, item)
				if len(matches) == historyPageSize {
					break
				}
			}

			if scanned >= historySearchMaxScanned {
				break
			}
		}

		if len(batch) < historySearchScanBatch {
			return matches, 0, nil
		}

		if len(matches) == historyPageSize || scanned >= historySearchMaxScanned {
			return matches, scanCursor, nil
		}
	}

	return matches, scanCursor, nil
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

func (h *Handler) historyItemMatchesFilters(item *store.Item, f historyFilters) bool {
	if f.HasDateFrom && item.CreatedAt.Before(f.DateFrom) {
		return false
	}

	if f.HasDateTo && item.CreatedAt.After(f.DateTo) {
		return false
	}

	if f.Query == "" {
		return true
	}

	return h.historyItemMatches(item, f.Query, f.SearchBody)
}

func (h *Handler) historyItemMatches(item *store.Item, query string, searchBody bool) bool {
	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	if normalizedQuery == "" {
		return true
	}

	if strings.Contains(strings.ToLower(item.Filename), normalizedQuery) {
		return true
	}

	if !searchBody || item.Type != "file" || !isTextLikeFile(item) {
		return false
	}

	return h.textFileBodyContains(item, normalizedQuery)
}

func (h *Handler) textFileBodyContains(item *store.Item, normalizedQuery string) bool {
	cleanPath := filepath.Clean(item.Content)
	if cleanPath == "." || filepath.IsAbs(cleanPath) || strings.Contains(cleanPath, "..") {
		return false
	}

	path := filepath.Join(h.dataDir, "uploads", cleanPath)

	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	body, err := io.ReadAll(io.LimitReader(file, historySearchMaxTextBytes+1))
	if err != nil || len(body) > historySearchMaxTextBytes {
		return false
	}

	return strings.Contains(strings.ToLower(string(body)), normalizedQuery)
}

func isTextLikeFile(item *store.Item) bool {
	mimeType := strings.ToLower(strings.TrimSpace(item.Metadata.MIME))
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}

	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	switch mimeType {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/x-javascript",
		"application/x-sh",
		"application/sql",
		"image/svg+xml":
		return true
	}

	switch strings.ToLower(filepath.Ext(item.Filename)) {
	case ".txt", ".md", ".markdown",
		".go", ".py", ".js", ".ts", ".tsx", ".jsx",
		".json", ".yaml", ".yml", ".toml", ".xml",
		".html", ".css", ".csv", ".sql", ".sh",
		".rs", ".c", ".cpp", ".h", ".hpp",
		".java", ".kt", ".rb", ".php", ".lua":
		return true
	default:
		return false
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
