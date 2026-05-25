package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

func TestHistorySearchPassesVisibilityFilter(t *testing.T) {
	now := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	search := &fakeHistorySearch{}
	uc := NewHistoryUseCase(search)
	uc.now = func() time.Time { return now }

	result, err := uc.Search(context.Background(), HistoryQuery{
		Visibility: domain.HistoryVisibilityPublic,
		Limit:      10,
	})
	if err != nil {
		t.Fatalf("Search(): %v", err)
	}

	if result.Visibility != domain.HistoryVisibilityPublic {
		t.Fatalf("result.Visibility = %q, want public", result.Visibility)
	}
	if search.got.Visibility != domain.HistoryVisibilityPublic {
		t.Fatalf("SearchHistory Visibility = %q, want public", search.got.Visibility)
	}
	if !search.got.Now.Equal(now) {
		t.Fatalf("SearchHistory Now = %v, want %v", search.got.Now, now)
	}
}

func TestHistorySearchRejectsInvalidVisibility(t *testing.T) {
	uc := NewHistoryUseCase(&fakeHistorySearch{})

	_, err := uc.Search(context.Background(), HistoryQuery{
		Visibility: "shared",
		Limit:      10,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Search() error = %v, want ErrInvalidInput", err)
	}
}

type fakeHistorySearch struct {
	got domain.HistorySearchOptions
}

func (f *fakeHistorySearch) IndexUploadedFile(
	context.Context,
	int64,
	string,
	string,
	int64,
	domain.Metadata,
) error {
	return nil
}

func (f *fakeHistorySearch) SearchHistory(
	_ context.Context,
	options domain.HistorySearchOptions,
) ([]*domain.Item, error) {
	f.got = options
	return nil, nil
}
