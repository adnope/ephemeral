package sqlite

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
	"github.com/adnope/ephemeral/internal/infrastructure/search"
	"github.com/adnope/ephemeral/internal/migrations"
)

func TestTextEditsPersistAcrossReadPaths(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/items.db"
	db, err := OpenDB(path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	repo := NewItemRepository(db)
	id, err := repo.Create(ctx, &domain.Item{Type: domain.ItemTypeText, Content: "oldword"})
	if err != nil {
		t.Fatal(err)
	}
	original, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if original.EditedAt != nil {
		t.Fatal("new item is marked edited")
	}
	first := time.Date(2026, 10, 23, 12, 0, 0, 123456789, time.FixedZone("local", 7*3600))
	item, changed, err := repo.UpdateText(ctx, id, "oldword", first)
	if err != nil || changed || item.EditedAt != nil {
		t.Fatalf("initial no-op: item=%+v changed=%v err=%v", item, changed, err)
	}
	item, changed, err = repo.UpdateText(ctx, id, "newword", first)
	if err != nil || !changed {
		t.Fatalf("first edit: changed=%v err=%v", changed, err)
	}
	if item.EditedAt == nil || !item.EditedAt.Equal(first) {
		t.Fatalf("edit time = %v", item.EditedAt)
	}
	second := first.Add(time.Hour)
	item, changed, err = repo.UpdateText(ctx, id, "latestword", second)
	if err != nil || !changed {
		t.Fatalf("second edit: changed=%v err=%v", changed, err)
	}
	item, changed, err = repo.UpdateText(ctx, id, "latestword", second.Add(time.Hour))
	if err != nil || changed {
		t.Fatalf("no-op edit: changed=%v err=%v", changed, err)
	}
	check := func(t *testing.T, item *domain.Item) {
		t.Helper()
		if item.Content != "latestword" || item.EditedAt == nil || !item.EditedAt.Equal(second) || !item.CreatedAt.Equal(original.CreatedAt) {
			t.Fatalf("unexpected item: %+v", item)
		}
	}
	check(t, item)
	if err := repo.UpdateMetadata(ctx, id, domain.Metadata{Width: 42}); err != nil {
		t.Fatal(err)
	}
	indexer := search.NewIndexer(db, t.TempDir(), 1024, nil)
	for name, read := range map[string]func() ([]*domain.Item, error){
		"get": func() ([]*domain.Item, error) { item, err := repo.GetByID(ctx, id); return []*domain.Item{item}, err },
		"list": func() ([]*domain.Item, error) {
			return repo.List(ctx, domain.ListFilter{Cursor: math.MaxInt64, Limit: 10})
		},
		"search": func() ([]*domain.Item, error) { return repo.Search(ctx, "latestword", 10) },
		"typed history": func() ([]*domain.Item, error) {
			return repo.MediaHistory(ctx, []string{domain.ItemTypeText}, math.MaxInt64, 10)
		},
		"history indexer": func() ([]*domain.Item, error) {
			return indexer.SearchHistory(ctx, domain.HistorySearchOptions{Types: []string{domain.ItemTypeText}, Cursor: math.MaxInt64, Limit: 10})
		},
	} {
		t.Run(name, func(t *testing.T) {
			items, err := read()
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 1 {
				t.Fatalf("items = %d", len(items))
			}
			check(t, items[0])
		})
	}
	for _, old := range []string{"oldword", "newword"} {
		matches, err := repo.Search(ctx, old, 10)
		if err != nil || len(matches) != 0 {
			t.Fatalf("obsolete term %q: matches=%v err=%v", old, matches, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = OpenDB(path, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := migrations.Apply(ctx, db); err != nil {
		t.Fatal(err)
	}
	item, err = NewItemRepository(db).GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	check(t, item)
}

func TestTextEditRejectsMissingAndNonTextItems(t *testing.T) {
	ctx := context.Background()
	db, err := OpenDB(t.TempDir()+"/items.db", loadTestMigrations(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	repo := NewItemRepository(db)
	if _, changed, err := repo.UpdateText(ctx, 999, "edited", time.Now()); !errors.Is(err, domain.ErrItemNotFound) || changed {
		t.Fatalf("missing: changed=%v err=%v", changed, err)
	}
	for _, kind := range []string{domain.ItemTypeImage, domain.ItemTypeVideo, domain.ItemTypeFile} {
		id, err := repo.Create(ctx, &domain.Item{Type: kind, Content: "original.bin"})
		if err != nil {
			t.Fatal(err)
		}
		if _, changed, err := repo.UpdateText(ctx, id, "edited", time.Now()); !errors.Is(err, domain.ErrItemNotText) || changed {
			t.Fatalf("%s: changed=%v err=%v", kind, changed, err)
		}
		item, err := repo.GetByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if item.Content != "original.bin" || item.EditedAt != nil {
			t.Fatalf("non-text item mutated: %+v", item)
		}
	}
	// A database failure must roll back both content and timestamp.
	id, err := repo.Create(ctx, &domain.Item{Type: domain.ItemTypeText, Content: "original"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TRIGGER fail_edit AFTER UPDATE OF content ON items BEGIN SELECT RAISE(ABORT, 'edit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, changed, err := repo.UpdateText(ctx, id, "edited", time.Now()); err == nil || changed {
		t.Fatalf("failure: changed=%v err=%v", changed, err)
	}
	item, err := repo.GetByID(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if item.Content != "original" || item.EditedAt != nil {
		t.Fatalf("failed edit mutated item: %+v", item)
	}
}
