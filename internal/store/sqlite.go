package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

func OpenDB(dbPath string, migrationSQL string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("store.OpenDB: %w", err)
	}

	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -4096",
		"PRAGMA foreign_keys = ON",
		"PRAGMA temp_store = MEMORY",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(context.Background(), p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("store.OpenDB pragma %q: %w", p, err)
		}
	}

	if migrationSQL != "" {
		if _, err := db.ExecContext(context.Background(), migrationSQL); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("store.OpenDB migration: %w", err)
		}
		slog.Info("migrations applied")
	}

	slog.Info("database initialized", "path", dbPath)
	return db, nil
}

func NewItemRepo(db *sql.DB) ItemRepository {
	return &sqliteItemRepo{db: db}
}

func NewSessionRepo(db *sql.DB) SessionRepository {
	return &sqliteSessionRepo{db: db}
}

func NewUserRepo(db *sql.DB) UserRepository {
	return &sqliteUserRepo{db: db}
}
