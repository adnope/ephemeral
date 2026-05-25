package sqlite

import (
	"context"
	"errors"
	"io/fs"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
	"github.com/adnope/ephemeral/internal/migrations"
)

func TestPublicLinkRepositoryUpsertAndCascadeDelete(t *testing.T) {
	db, err := OpenDB(t.TempDir()+"/ephemeral.db", loadTestMigrations(t))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	items := NewItemRepository(db)
	links := NewPublicLinkRepository(db)

	itemID, err := items.Create(ctx, &domain.Item{
		Type:     domain.ItemTypeFile,
		Content:  "sample.txt",
		Filename: "sample.txt",
		Filesize: 12,
		Metadata: domain.Metadata{MIME: "text/plain"},
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	expiresAt := time.Date(2026, 5, 25, 9, 0, 0, 0, time.UTC)
	first, err := links.UpsertForItem(ctx, &domain.PublicLink{
		Token:     "first-token",
		ItemID:    itemID,
		ExpiresAt: &expiresAt,
	})
	if err != nil {
		t.Fatalf("upsert first link: %v", err)
	}
	if first.Token != "first-token" || first.ExpiresAt == nil {
		t.Fatalf("first link = %#v", first)
	}
	firstForItem, err := links.GetForItem(ctx, itemID)
	if err != nil {
		t.Fatalf("get first link for item: %v", err)
	}
	if firstForItem.Token != "first-token" {
		t.Fatalf("first item token = %q, want first-token", firstForItem.Token)
	}

	second, err := links.UpsertForItem(ctx, &domain.PublicLink{
		Token:  "second-token",
		ItemID: itemID,
	})
	if err != nil {
		t.Fatalf("upsert second link: %v", err)
	}
	if second.Token != "second-token" || second.ExpiresAt != nil {
		t.Fatalf("second link = %#v", second)
	}
	if _, err := links.GetByToken(ctx, "first-token"); err == nil {
		t.Fatal("old token still resolves after item upsert")
	}
	if _, err := links.GetByToken(ctx, "second-token"); err != nil {
		t.Fatalf("new token does not resolve: %v", err)
	}
	secondForItem, err := links.GetForItem(ctx, itemID)
	if err != nil {
		t.Fatalf("get second link for item: %v", err)
	}
	if secondForItem.Token != "second-token" {
		t.Fatalf("second item token = %q, want second-token", secondForItem.Token)
	}

	if err := items.Delete(ctx, itemID); err != nil {
		t.Fatalf("delete item: %v", err)
	}
	if _, err := links.GetByToken(ctx, "second-token"); !errors.Is(err, domain.ErrPublicLinkNotFound) {
		t.Fatalf("public link survived item delete: %v", err)
	}
}

func loadTestMigrations(t *testing.T) string {
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
