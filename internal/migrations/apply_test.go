package migrations_test

import (
	"context"
	"testing"

	"github.com/adnope/ephemeral/internal/domain"
	"github.com/adnope/ephemeral/internal/infrastructure/sqlite"
	"github.com/adnope/ephemeral/internal/migrations"
)

func TestApplyFreshAndLegacyDatabases(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "fresh"
		if legacy {
			name = "legacy"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			path := t.TempDir() + "/migration.db"
			db, err := sqlite.OpenDB(path, "")
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = db.Close() }()
			if legacy {
				for _, name := range []string{"001_initial.sql", "002_item_bodies_fts.sql", "003_public_links.sql"} {
					body, err := migrations.FS.ReadFile(name)
					if err != nil {
						t.Fatal(err)
					}
					if _, err := db.ExecContext(ctx, string(body)); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := db.ExecContext(ctx, `INSERT INTO items(type,content) VALUES ('text','legacyword')`); err != nil {
					t.Fatal(err)
				}
			}
			for range 2 {
				if err := migrations.Apply(ctx, db); err != nil {
					t.Fatal(err)
				}
			}
			if legacy {
				item, err := sqlite.NewItemRepository(db).GetByID(ctx, 1)
				if err != nil {
					t.Fatal(err)
				}
				if item.Content != "legacyword" || item.EditedAt != nil {
					t.Fatalf("legacy item: %+v", item)
				}
				matches, err := sqlite.NewItemRepository(db).Search(ctx, "legacyword", 10)
				if err != nil || len(matches) != 1 {
					t.Fatalf("legacy FTS: matches=%v err=%v", matches, err)
				}
			} else {
				if _, err := sqlite.NewItemRepository(db).Create(ctx, &domain.Item{Type: domain.ItemTypeText, Content: "fresh"}); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = sqlite.OpenDB(path, "")
			if err != nil {
				t.Fatal(err)
			}
			if err := migrations.Apply(ctx, db); err != nil {
				t.Fatal(err)
			}
			var count int
			if err := db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != 4 {
				t.Fatalf("recorded migrations = %d", count)
			}
		})
	}
}
