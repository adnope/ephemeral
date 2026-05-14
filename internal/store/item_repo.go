package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type sqliteItemRepo struct {
	db *sql.DB
}

func (r *sqliteItemRepo) Create(ctx context.Context, item *Item) (int64, error) {
	const q = `
		INSERT INTO items (type, content, filename, filesize, metadata)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id`

	metaJSON, err := item.Metadata.Value()
	if err != nil {
		return 0, fmt.Errorf("store.Create marshal: %w", err)
	}

	var id int64
	err = r.db.QueryRowContext(ctx, q,
		item.Type, item.Content, item.Filename,
		item.Filesize, metaJSON,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store.Create: %w", err)
	}
	return id, nil
}

func (r *sqliteItemRepo) GetByID(ctx context.Context, id int64) (*Item, error) {
	const q = `
		SELECT id, type, content, filename, filesize, metadata, created_at
		FROM items
		WHERE id = ?`

	var it Item
	var filename sql.NullString
	var filesize sql.NullInt64
	var createdAt string
	err := r.db.QueryRowContext(ctx, q, id).Scan(
		&it.ID, &it.Type, &it.Content, &filename,
		&filesize, &it.Metadata, &createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("store.GetByID: %w", err)
	}
	it.Filename = filename.String
	it.Filesize = filesize.Int64
	it.CreatedAt = parseSQLiteTime(createdAt)
	return &it, nil
}

func (r *sqliteItemRepo) List(ctx context.Context, f ListFilter) ([]*Item, error) {
	const q = `
		SELECT id, type, content, filename, filesize, metadata, created_at
		FROM items
		WHERE id < ?
		ORDER BY id DESC
		LIMIT ?`

	rows, err := r.db.QueryContext(ctx, q, f.Cursor, f.Limit)
	if err != nil {
		return nil, fmt.Errorf("store.List: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (r *sqliteItemRepo) Delete(ctx context.Context, id int64) error {
	const q = `DELETE FROM items WHERE id = ?`
	result, err := r.db.ExecContext(ctx, q, id)
	if err != nil {
		return fmt.Errorf("store.Delete: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store.Delete rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("store.Delete: item %d not found", id)
	}
	return nil
}

func (r *sqliteItemRepo) Search(ctx context.Context, q string, limit int) ([]*Item, error) {
	const query = `
		SELECT i.id, i.type, i.content, i.filename, i.filesize, i.metadata, i.created_at
		FROM items_fts
		JOIN items i ON items_fts.rowid = i.id
		WHERE items_fts MATCH ?
		ORDER BY rank
		LIMIT ?`

	safe := strings.ReplaceAll(q, `"`, `""`)

	rows, err := r.db.QueryContext(ctx, query, safe, limit)
	if err != nil {
		return nil, fmt.Errorf("store.Search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (r *sqliteItemRepo) MediaHistory(ctx context.Context, types []string, cursor int64, limit int) ([]*Item, error) {
	if len(types) == 0 {
		types = []string{"image", "video", "file"}
	}

	placeholders := make([]string, len(types))
	args := make([]any, 0, len(types)+2)
	for i, t := range types {
		placeholders[i] = "?"
		args = append(args, t)
	}
	args = append(args, cursor, limit)

	query := fmt.Sprintf(`
		SELECT id, type, content, filename, filesize, metadata, created_at
		FROM items
		WHERE type IN (%s) AND id < ?
		ORDER BY id DESC
		LIMIT ?`, strings.Join(placeholders, ","))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store.MediaHistory: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (r *sqliteItemRepo) UpdateMetadata(ctx context.Context, id int64, meta Metadata) error {
	const q = `UPDATE items SET metadata = ? WHERE id = ?`
	metaJSON, err := meta.Value()
	if err != nil {
		return fmt.Errorf("store.UpdateMetadata marshal: %w", err)
	}
	_, err = r.db.ExecContext(ctx, q, metaJSON, id)
	if err != nil {
		return fmt.Errorf("store.UpdateMetadata: %w", err)
	}
	return nil
}

func scanItems(rows *sql.Rows) ([]*Item, error) {
	var items []*Item
	for rows.Next() {
		var it Item
		var filename sql.NullString
		var filesize sql.NullInt64
		var createdAt string
		if err := rows.Scan(
			&it.ID, &it.Type, &it.Content, &filename,
			&filesize, &it.Metadata, &createdAt,
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
