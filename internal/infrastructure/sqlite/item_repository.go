package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/adnope/ephemeral/internal/domain"
)

type itemRepository struct {
	db *sql.DB
}

func (r *itemRepository) Create(ctx context.Context, item *domain.Item) (int64, error) {
	const query = `
		INSERT INTO items (type, content, filename, filesize, metadata)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id`

	metaJSON, err := metadataValue(item.Metadata)
	if err != nil {
		return 0, fmt.Errorf("sqlite item create metadata: %w", err)
	}

	var id int64
	err = r.db.QueryRowContext(ctx, query,
		item.Type,
		item.Content,
		item.Filename,
		item.Filesize,
		metaJSON,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("sqlite item create: %w", err)
	}
	return id, nil
}

func (r *itemRepository) GetByID(ctx context.Context, id int64) (*domain.Item, error) {
	const query = `
		SELECT id, type, content, filename, filesize, metadata, created_at
		FROM items
		WHERE id = ?`

	var item domain.Item
	var filename sql.NullString
	var filesize sql.NullInt64
	var metadata metadataJSON
	var createdAt string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID,
		&item.Type,
		&item.Content,
		&filename,
		&filesize,
		&metadata,
		&createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite item get by id: %w", err)
	}

	item.Filename = filename.String
	item.Filesize = filesize.Int64
	item.Metadata = metadataToDomain(metadata)
	item.CreatedAt = parseSQLiteTime(createdAt)
	return &item, nil
}

func (r *itemRepository) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Item, error) {
	const query = `
		SELECT id, type, content, filename, filesize, metadata, created_at
		FROM items
		WHERE id < ?
		ORDER BY id DESC
		LIMIT ?`

	rows, err := r.db.QueryContext(ctx, query, filter.Cursor, filter.Limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite item list: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (r *itemRepository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM items WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("sqlite item delete: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite item delete rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("sqlite item delete: item %d not found", id)
	}
	return nil
}

func (r *itemRepository) Search(ctx context.Context, queryText string, limit int) ([]*domain.Item, error) {
	const query = `
		SELECT i.id, i.type, i.content, i.filename, i.filesize, i.metadata, i.created_at
		FROM items_fts
		JOIN items i ON items_fts.rowid = i.id
		WHERE items_fts MATCH ?
		ORDER BY rank
		LIMIT ?`

	safeQuery := strings.ReplaceAll(queryText, `"`, `""`)
	rows, err := r.db.QueryContext(ctx, query, safeQuery, limit)
	if err != nil {
		return nil, fmt.Errorf("sqlite item search: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (r *itemRepository) MediaHistory(ctx context.Context, types []string, cursor int64, limit int) ([]*domain.Item, error) {
	if len(types) == 0 {
		types = []string{domain.ItemTypeImage, domain.ItemTypeVideo, domain.ItemTypeFile}
	}

	placeholders := make([]string, len(types))
	args := make([]any, 0, len(types)+2)
	for i, itemType := range types {
		placeholders[i] = "?"
		args = append(args, itemType)
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
		return nil, fmt.Errorf("sqlite item media history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanItems(rows)
}

func (r *itemRepository) UpdateMetadata(ctx context.Context, id int64, meta domain.Metadata) error {
	const query = `UPDATE items SET metadata = ? WHERE id = ?`

	metaJSON, err := metadataValue(meta)
	if err != nil {
		return fmt.Errorf("sqlite item update metadata: %w", err)
	}

	if _, err := r.db.ExecContext(ctx, query, metaJSON, id); err != nil {
		return fmt.Errorf("sqlite item update metadata: %w", err)
	}
	return nil
}

func scanItems(rows *sql.Rows) ([]*domain.Item, error) {
	items := make([]*domain.Item, 0)
	for rows.Next() {
		var item domain.Item
		var filename sql.NullString
		var filesize sql.NullInt64
		var metadata metadataJSON
		var createdAt string

		if err := rows.Scan(
			&item.ID,
			&item.Type,
			&item.Content,
			&filename,
			&filesize,
			&metadata,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}

		item.Filename = filename.String
		item.Filesize = filesize.Int64
		item.Metadata = metadataToDomain(metadata)
		item.CreatedAt = parseSQLiteTime(createdAt)
		items = append(items, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}
	return items, nil
}
