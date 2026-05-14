package bodyindex

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/adnope/ephemeral/internal/store"
)

const (
	MaxIndexedTextBytes = 20 << 20 // 20 MiB
)

type Indexer struct {
	db      *sql.DB
	dataDir string
	log     *slog.Logger
}

type SearchOptions struct {
	Types       []string
	Cursor      int64
	Limit       int
	Query       string
	SearchBody  bool
	DateFrom    time.Time
	DateTo      time.Time
	HasDateFrom bool
	HasDateTo   bool
}

type candidate struct {
	ID       int64
	Content  string
	Filename string
	Filesize int64
	Metadata store.Metadata
}

func New(db *sql.DB, dataDir string, log *slog.Logger) *Indexer {
	return &Indexer{db: db, dataDir: dataDir, log: log}
}

func (x *Indexer) IndexUploadedFile(
	ctx context.Context,
	itemID int64,
	content string,
	filename string,
	filesize int64,
	meta store.Metadata,
) error {
	return x.IndexCandidate(ctx, candidate{
		ID:       itemID,
		Content:  content,
		Filename: filename,
		Filesize: filesize,
		Metadata: meta,
	})
}

func (x *Indexer) IndexCandidate(ctx context.Context, item candidate) error {
	if !isTextLike(item.Filename, item.Metadata.MIME) {
		return x.markState(ctx, item.ID, "skipped", 0, "not text-like")
	}

	if item.Filesize > MaxIndexedTextBytes {
		return x.markState(ctx, item.ID, "skipped", 0, "file too large")
	}

	path, err := x.safeUploadPath(item.Content)
	if err != nil {
		return x.markState(ctx, item.ID, "failed", 0, err.Error())
	}

	body, err := readTextFileLimited(path, MaxIndexedTextBytes)
	if err != nil {
		_ = x.markState(ctx, item.ID, "failed", 0, err.Error())
		return err
	}

	tx, err := x.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("body index begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM item_bodies_fts WHERE rowid = ?`, item.ID); err != nil {
		return fmt.Errorf("body index delete old fts: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO item_bodies_fts(rowid, filename, body) VALUES (?, ?, ?)`,
		item.ID, item.Filename, string(body),
	); err != nil {
		return fmt.Errorf("body index insert fts: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO item_body_index_state(item_id, status, body_bytes, error, indexed_at)
		VALUES (?, 'indexed', ?, NULL, CURRENT_TIMESTAMP)
		ON CONFLICT(item_id) DO UPDATE SET
			status = excluded.status,
			body_bytes = excluded.body_bytes,
			error = excluded.error,
			indexed_at = CURRENT_TIMESTAMP
	`, item.ID, len(body)); err != nil {
		return fmt.Errorf("body index upsert state: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("body index commit: %w", err)
	}

	return nil
}

func (x *Indexer) SearchHistory(ctx context.Context, opt SearchOptions) ([]*store.Item, error) {
	if opt.Cursor == 0 {
		return nil, fmt.Errorf("cursor must be non-zero")
	}
	if opt.Limit <= 0 {
		return nil, fmt.Errorf("limit must be positive")
	}

	args := make([]any, 0, 16)
	where := []string{"i.id < ?"}
	args = append(args, opt.Cursor)

	types := opt.Types
	if len(types) == 0 {
		types = []string{"image", "video", "file"}
	}

	typePlaceholders := make([]string, len(types))
	for i, t := range types {
		typePlaceholders[i] = "?"
		args = append(args, t)
	}
	where = append(where, "i.type IN ("+strings.Join(typePlaceholders, ",")+")")

	if opt.HasDateFrom {
		where = append(where, "i.created_at >= ?")
		args = append(args, opt.DateFrom.Format("2006-01-02 15:04:05"))
	}
	if opt.HasDateTo {
		where = append(where, "i.created_at <= ?")
		args = append(args, opt.DateTo.Format("2006-01-02 15:04:05"))
	}

	tokens := searchTokens(opt.Query)
	if len(tokens) > 0 {
		nameClauses := make([]string, 0, len(tokens))
		for _, token := range tokens {
			nameClauses = append(nameClauses, "LOWER(COALESCE(i.filename, '')) LIKE ?")
			args = append(args, "%"+strings.ToLower(token)+"%")
		}

		if opt.SearchBody {
			ftsQuery := buildFTSAndQuery(tokens)
			where = append(where, "(("+strings.Join(nameClauses, " AND ")+") OR i.id IN (SELECT rowid FROM item_bodies_fts WHERE item_bodies_fts MATCH ?))")
			args = append(args, ftsQuery)
		} else {
			where = append(where, "("+strings.Join(nameClauses, " AND ")+")")
		}
	}

	query := `
		SELECT i.id, i.type, i.content, i.filename, i.filesize, i.metadata, i.created_at
		FROM items i
		WHERE ` + strings.Join(where, " AND ") + `
		ORDER BY i.id DESC
		LIMIT ?`
	args = append(args, opt.Limit)

	rows, err := x.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("body index search history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (x *Indexer) markState(ctx context.Context, itemID int64, status string, bodyBytes int, message string) error {
	_, err := x.db.ExecContext(ctx, `
		INSERT INTO item_body_index_state(item_id, status, body_bytes, error, indexed_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(item_id) DO UPDATE SET
			status = excluded.status,
			body_bytes = excluded.body_bytes,
			error = excluded.error,
			indexed_at = CURRENT_TIMESTAMP
	`, itemID, status, bodyBytes, message)
	if err != nil {
		return fmt.Errorf("body index mark state: %w", err)
	}
	return nil
}

func (x *Indexer) safeUploadPath(content string) (string, error) {
	cleanPath := filepath.Clean(content)
	if cleanPath == "." || filepath.IsAbs(cleanPath) || strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("unsafe upload path")
	}
	return filepath.Join(x.dataDir, "uploads", cleanPath), nil
}

func readTextFileLimited(path string, maxBytes int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open text file: %w", err)
	}
	defer func() { _ = file.Close() }()

	body, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read text file: %w", err)
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("text file exceeds max indexed size")
	}

	return body, nil
}

func isTextLike(filename string, mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}

	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	switch mimeType {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/x-javascript",
		"application/x-sh",
		"application/sql",
		"image/svg+xml":
		return true
	}

	switch strings.ToLower(filepath.Ext(filename)) {
	case ".txt", ".md", ".markdown",
		".go", ".py", ".js", ".ts", ".tsx", ".jsx",
		".json", ".yaml", ".yml", ".toml", ".xml",
		".html", ".css", ".csv", ".sql", ".sh",
		".rs", ".c", ".cpp", ".h", ".hpp",
		".java", ".kt", ".rb", ".php", ".lua":
		return true
	default:
		return false
	}
}

var tokenSplitter = regexp.MustCompile(`[\s]+`)

func searchTokens(query string) []string {
	normalized := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			return unicode.ToLower(r)
		}
		return ' '
	}, query)

	raw := tokenSplitter.Split(strings.TrimSpace(normalized), -1)
	tokens := make([]string, 0, len(raw))

	for _, token := range raw {
		token = strings.TrimSpace(token)
		if token != "" {
			tokens = append(tokens, token)
		}
	}

	return tokens
}

func buildFTSAndQuery(tokens []string) string {
	parts := make([]string, 0, len(tokens))

	for _, token := range tokens {
		escaped := strings.ReplaceAll(token, `"`, `""`)
		parts = append(parts, `"`+escaped+`"*`)
	}

	return strings.Join(parts, " AND ")
}

func scanItems(rows *sql.Rows) ([]*store.Item, error) {
	var items []*store.Item

	for rows.Next() {
		var it store.Item
		var filename sql.NullString
		var filesize sql.NullInt64
		var createdAt string

		if err := rows.Scan(
			&it.ID,
			&it.Type,
			&it.Content,
			&filename,
			&filesize,
			&it.Metadata,
			&createdAt,
		); err != nil {
			return nil, err
		}

		it.Filename = filename.String
		it.Filesize = filesize.Int64
		it.CreatedAt = parseSQLiteTime(createdAt)
		items = append(items, &it)
	}

	return items, rows.Err()
}

func parseSQLiteTime(s string) time.Time {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}

	return time.Time{}
}
