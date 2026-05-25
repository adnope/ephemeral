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

type publicLinkRepository struct {
	db *sql.DB
}

func (r *publicLinkRepository) UpsertForItem(ctx context.Context, link *domain.PublicLink) (*domain.PublicLink, error) {
	if link == nil {
		return nil, fmt.Errorf("sqlite public link upsert: link is nil")
	}

	const query = `
		INSERT INTO public_links (token, item_id, expires_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(item_id) DO UPDATE SET
			token = excluded.token,
			expires_at = excluded.expires_at,
			updated_at = CURRENT_TIMESTAMP
		RETURNING token, item_id, expires_at, created_at, updated_at`

	row := r.db.QueryRowContext(ctx, query, link.Token, link.ItemID, nullableTime(link.ExpiresAt))
	return scanPublicLink(row)
}

func (r *publicLinkRepository) GetByToken(ctx context.Context, token string) (*domain.PublicLink, error) {
	const query = `
		SELECT token, item_id, expires_at, created_at, updated_at
		FROM public_links
		WHERE token = ?`

	row := r.db.QueryRowContext(ctx, query, token)
	return scanPublicLink(row)
}

func (r *publicLinkRepository) GetForItem(ctx context.Context, itemID int64) (*domain.PublicLink, error) {
	const query = `
		SELECT token, item_id, expires_at, created_at, updated_at
		FROM public_links
		WHERE item_id = ?`

	row := r.db.QueryRowContext(ctx, query, itemID)
	return scanPublicLink(row)
}

func (r *publicLinkRepository) ActiveItemIDs(ctx context.Context, itemIDs []int64, now time.Time) (map[int64]bool, error) {
	active := make(map[int64]bool)
	if len(itemIDs) == 0 {
		return active, nil
	}

	placeholders := make([]string, len(itemIDs))
	args := make([]any, 0, len(itemIDs)+1)
	for i, itemID := range itemIDs {
		placeholders[i] = "?"
		args = append(args, itemID)
	}
	args = append(args, now.UTC())

	query := fmt.Sprintf(`
		SELECT item_id
		FROM public_links
		WHERE item_id IN (%s)
		  AND (expires_at IS NULL OR expires_at > ?)`,
		strings.Join(placeholders, ","),
	)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite active public link item ids: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var itemID int64
		if err := rows.Scan(&itemID); err != nil {
			return nil, fmt.Errorf("sqlite active public link item id scan: %w", err)
		}
		active[itemID] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite active public link item ids rows: %w", err)
	}

	return active, nil
}

func (r *publicLinkRepository) DeleteForItem(ctx context.Context, itemID int64) error {
	const query = `DELETE FROM public_links WHERE item_id = ?`

	if _, err := r.db.ExecContext(ctx, query, itemID); err != nil {
		return fmt.Errorf("sqlite public link delete for item: %w", err)
	}
	return nil
}

func (r *publicLinkRepository) DeleteByToken(ctx context.Context, token string) error {
	const query = `DELETE FROM public_links WHERE token = ?`

	if _, err := r.db.ExecContext(ctx, query, token); err != nil {
		return fmt.Errorf("sqlite public link delete by token: %w", err)
	}
	return nil
}

type publicLinkScanner interface {
	Scan(dest ...any) error
}

func scanPublicLink(scanner publicLinkScanner) (*domain.PublicLink, error) {
	var link domain.PublicLink
	var expiresAt sql.NullTime

	if err := scanner.Scan(
		&link.Token,
		&link.ItemID,
		&expiresAt,
		&link.CreatedAt,
		&link.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrPublicLinkNotFound
		}
		return nil, fmt.Errorf("sqlite public link scan: %w", err)
	}

	if expiresAt.Valid {
		link.ExpiresAt = &expiresAt.Time
	}
	return &link, nil
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}
