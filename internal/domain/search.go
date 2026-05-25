package domain

import (
	"context"
	"time"
)

const (
	HistoryVisibilityAll     = ""
	HistoryVisibilityPublic  = "public"
	HistoryVisibilityPrivate = "private"
)

type HistorySearchOptions struct {
	Types       []string
	Cursor      int64
	Limit       int
	Query       string
	SearchBody  bool
	Visibility  string
	DateFrom    time.Time
	DateTo      time.Time
	Now         time.Time
	HasDateFrom bool
	HasDateTo   bool
}

type SearchService interface {
	IndexUploadedFile(ctx context.Context, itemID int64, content string, filename string, filesize int64, meta Metadata) error
	SearchHistory(ctx context.Context, options HistorySearchOptions) ([]*Item, error)
}
