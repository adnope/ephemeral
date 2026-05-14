package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	_ "modernc.org/sqlite"
)

// OpenDB initializes the SQLite database with performance-tuned PRAGMAs.
// Migration SQL must be provided by the caller (embedded at the cmd level).
func OpenDB(dbPath string, migrationSQL string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("store.OpenDB: %w", err)
	}

	// Single writer connection to prevent SQLITE_BUSY under WAL mode.
	// Reads are concurrent; writes are serialized by SQLite's WAL lock.
	db.SetMaxOpenConns(1)

	// Performance-tuned PRAGMAs applied at connection time
	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -4096",
		"PRAGMA foreign_keys = ON",
		"PRAGMA temp_store = MEMORY",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(context.Background(), p); err != nil {
			db.Close()
			return nil, fmt.Errorf("store.OpenDB pragma %q: %w", p, err)
		}
	}

	if migrationSQL != "" {
		if _, err := db.ExecContext(context.Background(), migrationSQL); err != nil {
			db.Close()
			return nil, fmt.Errorf("store.OpenDB migration: %w", err)
		}
		slog.Info("migrations applied")
	}

	slog.Info("database initialized", "path", dbPath)
	return db, nil
}

// NewItemRepo creates a new SQLite-backed ItemRepository.
func NewItemRepo(db *sql.DB) ItemRepository {
	return &sqliteItemRepo{db: db}
}

// NewSessionRepo creates a new SQLite-backed SessionRepository.
func NewSessionRepo(db *sql.DB) SessionRepository {
	return &sqliteSessionRepo{db: db}
}

// NewUserRepo creates a new SQLite-backed UserRepository.
func NewUserRepo(db *sql.DB) UserRepository {
	return &sqliteUserRepo{db: db}
}
