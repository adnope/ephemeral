package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

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
		SELECT id, type, content, filename, filesize, metadata, created_at, edited_at
		FROM items
		WHERE id = ?`

	item, err := scanItem(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return nil, fmt.Errorf("sqlite item get by id: %w", err)
	}
	return item, nil
}

func (r *itemRepository) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Item, error) {
	const query = `
		SELECT id, type, content, filename, filesize, metadata, created_at, edited_at
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
		SELECT i.id, i.type, i.content, i.filename, i.filesize, i.metadata, i.created_at, i.edited_at
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
		SELECT id, type, content, filename, filesize, metadata, created_at, edited_at
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

// UpdateText persists content and edit time in one transaction. The returned item
// is read in the same transaction so the response matches the committed edit.
func (r *itemRepository) UpdateText(ctx context.Context, id int64, text string, editedAt time.Time) (*domain.Item, bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("sqlite edit begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `UPDATE items SET content = ?, edited_at = ?
		WHERE id = ? AND type = 'text' AND content <> ?`, text, editedAt.UTC().Format(time.RFC3339Nano), id, text)
	if err != nil {
		return nil, false, fmt.Errorf("sqlite edit text: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, false, fmt.Errorf("sqlite edit rows: %w", err)
	}
	item, err := scanItem(tx.QueryRowContext(ctx, `SELECT id, type, content, filename, filesize, metadata, created_at, edited_at FROM items WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, domain.ErrItemNotFound
	}
	if err != nil {
		return nil, false, fmt.Errorf("sqlite read edited item: %w", err)
	}
	if item.Type != domain.ItemTypeText {
		return nil, false, domain.ErrItemNotText
	}
	if err := tx.Commit(); err != nil {
		return nil, false, fmt.Errorf("sqlite edit commit: %w", err)
	}
	return item, affected > 0, nil
}

type itemScanner interface {
	Scan(...any) error
}

func scanItem(row itemScanner) (*domain.Item, error) {
	var item domain.Item
	var filename sql.NullString
	var filesize sql.NullInt64
	var metadata metadataJSON
	var createdAt string
	var editedAt sql.NullString
	if err := row.Scan(&item.ID, &item.Type, &item.Content, &filename, &filesize, &metadata, &createdAt, &editedAt); err != nil {
		return nil, err
	}
	item.Filename = filename.String
	item.Filesize = filesize.Int64
	item.Metadata = metadataToDomain(metadata)
	item.CreatedAt = parseSQLiteTime(createdAt)
	if editedAt.Valid {
		value := parseSQLiteTime(editedAt.String)
		item.EditedAt = &value
	}
	return &item, nil
}

func scanItems(rows *sql.Rows) ([]*domain.Item, error) {
	items := make([]*domain.Item, 0)
	for rows.Next() {
		item, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate items: %w", err)
	}
	return items, nil
}
