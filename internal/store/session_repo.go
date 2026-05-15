package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type sqliteSessionRepo struct {
	db *sql.DB
}

func (r *sqliteSessionRepo) Create(ctx context.Context, s *Session) error {
	const q = `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES (?, ?, ?)`

	_, err := r.db.ExecContext(ctx, q, s.Token, s.UserID, s.ExpiresAt)
	if err != nil {
		return fmt.Errorf("store.session.Create: %w", err)
	}
	return nil
}

func (r *sqliteSessionRepo) GetByToken(ctx context.Context, token string) (*Session, error) {
	const q = `
		SELECT token, user_id, created_at, expires_at
		FROM sessions
		WHERE token = ?`

	var s Session
	err := r.db.QueryRowContext(ctx, q, token).Scan(
		&s.Token, &s.UserID, &s.CreatedAt, &s.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("store.session.GetByToken: %w", err)
	}
	return &s, nil
}

func (r *sqliteSessionRepo) Refresh(ctx context.Context, token string, expiresAt time.Time) error {
	const q = `
		UPDATE sessions
		SET expires_at = ?
		WHERE token = ?`

	result, err := r.db.ExecContext(ctx, q, expiresAt, token)
	if err != nil {
		return fmt.Errorf("store.session.Refresh: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("store.session.Refresh rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("store.session.Refresh: session not found")
	}

	return nil
}

func (r *sqliteSessionRepo) Delete(ctx context.Context, token string) error {
	const q = `DELETE FROM sessions WHERE token = ?`
	_, err := r.db.ExecContext(ctx, q, token)
	if err != nil {
		return fmt.Errorf("store.session.Delete: %w", err)
	}
	return nil
}

func (r *sqliteSessionRepo) PurgeExpired(ctx context.Context) error {
	const q = `DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP`
	result, err := r.db.ExecContext(ctx, q)
	if err != nil {
		return fmt.Errorf("store.session.PurgeExpired: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected > 0 {
		fmt.Printf("purged %d expired sessions\n", affected)
	}
	return nil
}

type sqliteUserRepo struct {
	db *sql.DB
}

func (r *sqliteUserRepo) Create(ctx context.Context, u *User) (int64, error) {
	const q = `
		INSERT INTO users (username, password_hash)
		VALUES (?, ?)
		RETURNING id`

	var id int64
	err := r.db.QueryRowContext(ctx, q, u.Username, u.PasswordHash).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("store.user.Create: %w", err)
	}
	return id, nil
}

func (r *sqliteUserRepo) GetByUsername(ctx context.Context, username string) (*User, error) {
	const q = `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = ?`

	var u User
	err := r.db.QueryRowContext(ctx, q, username).Scan(
		&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("store.user.GetByUsername: %w", err)
	}
	return &u, nil
}

func (r *sqliteUserRepo) Count(ctx context.Context) (int, error) {
	const q = `SELECT COUNT(*) FROM users`
	var count int
	err := r.db.QueryRowContext(ctx, q).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("store.user.Count: %w", err)
	}
	return count, nil
}
