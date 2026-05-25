package usecase

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

const historyDateLayout = "2006-01-02"

type HistoryUseCase struct {
	search domain.SearchService
	now    func() time.Time
}

type HistoryQuery struct {
	Cursor      int64
	Types       []string
	Query       string
	SearchBody  bool
	Visibility  string
	DateFromRaw string
	DateToRaw   string
	Recent      string
	Limit       int
}

type HistoryResult struct {
	Items      []*domain.Item
	NextCursor int64
	Types      []string
	Query      string
	SearchBody bool
	Visibility string
	DateFrom   string
	DateTo     string
	Recent     string
}

type historyFilters struct {
	Types       []string
	Query       string
	SearchBody  bool
	Visibility  string
	DateFrom    time.Time
	DateTo      time.Time
	Recent      string
	HasDateFrom bool
	HasDateTo   bool
}

func NewHistoryUseCase(search domain.SearchService) *HistoryUseCase {
	return &HistoryUseCase{search: search, now: time.Now}
}

func (uc *HistoryUseCase) Search(ctx context.Context, query HistoryQuery) (HistoryResult, error) {
	if query.Cursor == 0 {
		query.Cursor = math.MaxInt64
	}
	if query.Limit <= 0 {
		return HistoryResult{}, fmt.Errorf("%w: limit must be positive", ErrInvalidInput)
	}

	now := uc.now()
	filters, err := parseHistoryFilters(
		query.Types,
		strings.TrimSpace(query.Query),
		query.SearchBody,
		strings.TrimSpace(query.Visibility),
		strings.TrimSpace(query.DateFromRaw),
		strings.TrimSpace(query.DateToRaw),
		strings.TrimSpace(query.Recent),
		now,
	)
	if err != nil {
		return HistoryResult{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	items, err := uc.search.SearchHistory(ctx, domain.HistorySearchOptions{
		Types:       filters.Types,
		Cursor:      query.Cursor,
		Limit:       query.Limit,
		Query:       filters.Query,
		SearchBody:  filters.SearchBody,
		Visibility:  filters.Visibility,
		DateFrom:    filters.DateFrom,
		DateTo:      filters.DateTo,
		Now:         now,
		HasDateFrom: filters.HasDateFrom,
		HasDateTo:   filters.HasDateTo,
	})
	if err != nil {
		return HistoryResult{}, fmt.Errorf("search history: %w", err)
	}

	return HistoryResult{
		Items:      items,
		NextCursor: nextCursor(items, query.Limit),
		Types:      filters.Types,
		Query:      filters.Query,
		SearchBody: filters.SearchBody,
		Visibility: filters.Visibility,
		DateFrom:   query.DateFromRaw,
		DateTo:     query.DateToRaw,
		Recent:     filters.Recent,
	}, nil
}

func parseHistoryFilters(
	types []string,
	query string,
	searchBody bool,
	visibility string,
	dateFromRaw string,
	dateToRaw string,
	recent string,
	now time.Time,
) (historyFilters, error) {
	filters := historyFilters{
		Types:      types,
		Query:      query,
		SearchBody: searchBody,
		Recent:     recent,
	}

	switch visibility {
	case "", "all":
		filters.Visibility = domain.HistoryVisibilityAll
	case domain.HistoryVisibilityPublic, domain.HistoryVisibilityPrivate:
		filters.Visibility = visibility
	default:
		return filters, fmt.Errorf("invalid visibility")
	}

	if dateFromRaw != "" {
		dateFrom, err := time.ParseInLocation(historyDateLayout, dateFromRaw, time.Local)
		if err != nil {
			return filters, fmt.Errorf("parse from date: %w", err)
		}
		filters.DateFrom = dateFrom
		filters.HasDateFrom = true
	}

	if dateToRaw != "" {
		dateTo, err := time.ParseInLocation(historyDateLayout, dateToRaw, time.Local)
		if err != nil {
			return filters, fmt.Errorf("parse to date: %w", err)
		}
		filters.DateTo = dateTo.Add(24*time.Hour - time.Nanosecond)
		filters.HasDateTo = true
	}

	if recent != "" {
		from, ok := recentCutoff(recent, now)
		if ok && (!filters.HasDateFrom || from.After(filters.DateFrom)) {
			filters.DateFrom = from
			filters.HasDateFrom = true
		}
	}

	return filters, nil
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
