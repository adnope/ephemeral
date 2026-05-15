package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/adnope/ephemeral/internal/domain"

	_ "modernc.org/sqlite"
)

func OpenDB(dbPath string, migrationSQL string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -4096",
		"PRAGMA foreign_keys = ON",
		"PRAGMA temp_store = MEMORY",
	}
	for _, pragma := range pragmas {
		if _, err := db.ExecContext(context.Background(), pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite pragma %q: %w", pragma, err)
		}
	}

	if migrationSQL != "" {
		if _, err := db.ExecContext(context.Background(), migrationSQL); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("sqlite migration: %w", err)
		}
		slog.Info("migrations applied")
	}

	slog.Info("database initialized", "path", dbPath)
	return db, nil
}

func NewItemRepository(db *sql.DB) domain.ItemRepository {
	return &itemRepository{db: db}
}

func NewSessionRepository(db *sql.DB) domain.SessionRepository {
	return &sessionRepository{db: db}
}

func NewUserRepository(db *sql.DB) domain.UserRepository {
	return &userRepository{db: db}
}

type metadataJSON struct {
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Duration string `json:"duration,omitempty"`
	MIME     string `json:"mime,omitempty"`
	Thumb    string `json:"thumb,omitempty"`
}

func (m *metadataJSON) Scan(src any) error {
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	case nil:
		*m = metadataJSON{}
		return nil
	default:
		return fmt.Errorf("unsupported metadata source %T", src)
	}

	if len(data) == 0 {
		*m = metadataJSON{}
		return nil
	}
	if err := json.Unmarshal(data, m); err != nil {
		return fmt.Errorf("unmarshal metadata: %w", err)
	}
	return nil
}

func metadataValue(meta domain.Metadata) (string, error) {
	data, err := json.Marshal(metadataFromDomain(meta))
	if err != nil {
		return "{}", fmt.Errorf("marshal metadata: %w", err)
	}
	return string(data), nil
}

func metadataFromDomain(meta domain.Metadata) metadataJSON {
	return metadataJSON{
		Width:    meta.Width,
		Height:   meta.Height,
		Duration: meta.Duration,
		MIME:     meta.MIME,
		Thumb:    meta.Thumb,
	}
}

func metadataToDomain(meta metadataJSON) domain.Metadata {
	return domain.Metadata{
		Width:    meta.Width,
		Height:   meta.Height,
		Duration: meta.Duration,
		MIME:     meta.MIME,
		Thumb:    meta.Thumb,
	}
}

func parseSQLiteTime(value string) time.Time {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
