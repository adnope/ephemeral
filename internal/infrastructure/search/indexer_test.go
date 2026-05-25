package search

import (
	"context"
	"io/fs"
	"math"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
	sqliteinfra "github.com/adnope/ephemeral/internal/infrastructure/sqlite"
	"github.com/adnope/ephemeral/internal/migrations"
)

func TestMetadataJSONScanIncludesVideoPlaybackFields(t *testing.T) {
	var meta metadataJSON
	if err := meta.Scan([]byte(`{
		"width": 1920,
		"height": 1080,
		"duration": "12m34s",
		"mime": "video/x-matroska",
		"thumb": "thumbs/video.jpg",
		"playback": "playback/video.mp4",
		"playbackMime": "video/mp4",
		"hls": "hls/video/index.m3u8",
		"processing": true
	}`)); err != nil {
		t.Fatalf("Scan(): %v", err)
	}

	got := meta.toDomain()
	if got.Playback != "playback/video.mp4" {
		t.Fatalf("Playback = %q", got.Playback)
	}
	if got.PlaybackMIME != "video/mp4" {
		t.Fatalf("PlaybackMIME = %q", got.PlaybackMIME)
	}
	if got.HLS != "hls/video/index.m3u8" {
		t.Fatalf("HLS = %q", got.HLS)
	}
	if !got.Processing {
		t.Fatal("Processing = false, want true")
	}
}

func TestSearchHistoryFiltersByActivePublicVisibility(t *testing.T) {
	db, err := sqliteinfra.OpenDB(t.TempDir()+"/ephemeral.db", loadSearchTestMigrations(t))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	items := sqliteinfra.NewItemRepository(db)
	links := sqliteinfra.NewPublicLinkRepository(db)
	indexer := NewIndexer(db, t.TempDir(), 1024, nil)

	activeItemID := createSearchHistoryTestItem(t, ctx, items, "active.txt")
	expiredItemID := createSearchHistoryTestItem(t, ctx, items, "expired.txt")
	privateItemID := createSearchHistoryTestItem(t, ctx, items, "private.txt")

	now := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	activeExpiresAt := now.Add(time.Hour)
	expiredAt := now.Add(-time.Second)

	if _, err := links.UpsertForItem(ctx, &domain.PublicLink{
		Token:     "active-token",
		ItemID:    activeItemID,
		ExpiresAt: &activeExpiresAt,
	}); err != nil {
		t.Fatalf("upsert active link: %v", err)
	}
	if _, err := links.UpsertForItem(ctx, &domain.PublicLink{
		Token:     "expired-token",
		ItemID:    expiredItemID,
		ExpiresAt: &expiredAt,
	}); err != nil {
		t.Fatalf("upsert expired link: %v", err)
	}

	publicItems, err := indexer.SearchHistory(ctx, domain.HistorySearchOptions{
		Types:      []string{domain.ItemTypeFile},
		Cursor:     math.MaxInt64,
		Limit:      10,
		Visibility: domain.HistoryVisibilityPublic,
		Now:        now,
	})
	if err != nil {
		t.Fatalf("SearchHistory(public): %v", err)
	}
	assertSearchHistoryIDs(t, publicItems, map[int64]bool{activeItemID: true})

	privateItems, err := indexer.SearchHistory(ctx, domain.HistorySearchOptions{
		Types:      []string{domain.ItemTypeFile},
		Cursor:     math.MaxInt64,
		Limit:      10,
		Visibility: domain.HistoryVisibilityPrivate,
		Now:        now,
	})
	if err != nil {
		t.Fatalf("SearchHistory(private): %v", err)
	}
	assertSearchHistoryIDs(t, privateItems, map[int64]bool{
		expiredItemID: true,
		privateItemID: true,
	})
}

func createSearchHistoryTestItem(
	t *testing.T,
	ctx context.Context,
	items domain.ItemRepository,
	filename string,
) int64 {
	t.Helper()

	itemID, err := items.Create(ctx, &domain.Item{
		Type:     domain.ItemTypeFile,
		Content:  filename,
		Filename: filename,
		Filesize: 12,
		Metadata: domain.Metadata{MIME: "text/plain"},
	})
	if err != nil {
		t.Fatalf("create item %s: %v", filename, err)
	}
	return itemID
}

func assertSearchHistoryIDs(t *testing.T, items []*domain.Item, want map[int64]bool) {
	t.Helper()

	if len(items) != len(want) {
		t.Fatalf("item count = %d, want %d: %#v", len(items), len(want), items)
	}

	for _, item := range items {
		if !want[item.ID] {
			t.Fatalf("unexpected item id %d in result", item.ID)
		}
	}
}

func loadSearchTestMigrations(t *testing.T) string {
	t.Helper()

	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		data, err := fs.ReadFile(migrations.FS, name)
		if err != nil {
			t.Fatalf("read migration %s: %v", name, err)
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}
