package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/adnope/ephemeral/internal/domain"
)

func TestDSNWithPragmas(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ephemeral test.db")

	dsn, err := dsnWithPragmas(dbPath)
	if err != nil {
		t.Fatalf("build dsn: %v", err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse dsn: %v", err)
	}
	if parsed.Scheme != "file" {
		t.Fatalf("scheme = %q, want file", parsed.Scheme)
	}
	if parsed.Path != dbPath {
		t.Fatalf("path = %q, want %q", parsed.Path, dbPath)
	}

	query := parsed.Query()
	for _, pragma := range connectionPragmas {
		if !contains(query["_pragma"], pragma) {
			t.Fatalf("dsn missing _pragma=%q in %q", pragma, parsed.RawQuery)
		}
	}
}

func TestOpenDBAppliesPragmasOnNewConnections(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ephemeral.db")
	db, err := OpenDB(dbPath, `CREATE TABLE IF NOT EXISTS smoke (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if got := db.Stats().MaxOpenConnections; got != maxOpenConns {
		t.Fatalf("max open connections = %d, want %d", got, maxOpenConns)
	}

	for i := 0; i < 3; i++ {
		assertConnectionPragmas(t, db)
	}
}

func TestOpenDBSupportsRelativePath(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.MkdirAll("data", 0o755); err != nil {
		t.Fatalf("mkdir data: %v", err)
	}

	db, err := OpenDB(filepath.Join(".", "data", "ephemeral.db"), `CREATE TABLE IF NOT EXISTS smoke (id INTEGER PRIMARY KEY)`)
	if err != nil {
		t.Fatalf("open relative db: %v", err)
	}
	defer func() { _ = db.Close() }()

	if _, err := db.ExecContext(context.Background(), `INSERT INTO smoke DEFAULT VALUES`); err != nil {
		t.Fatalf("insert smoke row: %v", err)
	}
}

func TestRetryInterruptedReadRetriesOnlyWhileRequestIsActive(t *testing.T) {
	t.Run("retries interrupt", func(t *testing.T) {
		attempts := 0
		err := retryInterruptedRead(context.Background(), func() error {
			attempts++
			if attempts < 3 {
				return errors.New("interrupted (9)")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("retry interrupted read: %v", err)
		}
		if attempts != 3 {
			t.Fatalf("attempts = %d, want 3", attempts)
		}
	})

	t.Run("stops on canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		attempts := 0
		err := retryInterruptedRead(ctx, func() error {
			attempts++
			return errors.New("interrupted (9)")
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v, want context canceled", err)
		}
		if attempts != 1 {
			t.Fatalf("attempts = %d, want 1", attempts)
		}
	})
}

func TestMetadataRoundTripIncludesPlaybackFields(t *testing.T) {
	t.Parallel()

	meta := domain.Metadata{
		Width:        1920,
		Height:       1080,
		Duration:     "24:00",
		MIME:         "video/x-matroska",
		Thumb:        "thumbs/sample_thumb.jpg",
		Playback:     "playback/sample_playback.mp4",
		PlaybackMIME: "video/mp4",
		HLS:          "hls/sample/index.m3u8",
		Processing:   true,
	}

	value, err := metadataValue(meta)
	if err != nil {
		t.Fatalf("metadataValue(): %v", err)
	}

	var scanned metadataJSON
	if err := scanned.Scan(value); err != nil {
		t.Fatalf("Scan(): %v", err)
	}

	got := metadataToDomain(scanned)
	if got != meta {
		t.Fatalf("metadata round trip = %#v, want %#v", got, meta)
	}
}

func assertConnectionPragmas(t *testing.T, db *sql.DB) {
	t.Helper()

	ctx := context.Background()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("get conn: %v", err)
	}
	defer func() { _ = conn.Close() }()

	var foreignKeys int
	if err := conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("query foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	var tempStore int
	if err := conn.QueryRowContext(ctx, `PRAGMA temp_store`).Scan(&tempStore); err != nil {
		t.Fatalf("query temp_store: %v", err)
	}
	if tempStore != 2 {
		t.Fatalf("temp_store = %d, want 2", tempStore)
	}

	var journalMode string
	if err := conn.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		t.Fatalf("query journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", journalMode)
	}

	var busyTimeout int
	if err := conn.QueryRowContext(ctx, `PRAGMA busy_timeout`).Scan(&busyTimeout); err != nil {
		t.Fatalf("query busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
