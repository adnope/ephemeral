package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

type sessionRepository struct {
	db *sql.DB
}

func (r *sessionRepository) Create(ctx context.Context, session *domain.Session) error {
	const query = `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES (?, ?, ?)`

	if _, err := r.db.ExecContext(ctx, query, session.Token, session.UserID, session.ExpiresAt); err != nil {
		return fmt.Errorf("sqlite session create: %w", err)
	}
	return nil
}

func (r *sessionRepository) GetByToken(ctx context.Context, token string) (*domain.Session, error) {
	const query = `
		SELECT token, user_id, created_at, expires_at
		FROM sessions
		WHERE token = ?`

	var session domain.Session
	err := r.db.QueryRowContext(ctx, query, token).Scan(
		&session.Token,
		&session.UserID,
		&session.CreatedAt,
		&session.ExpiresAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite session get by token: %w", err)
	}
	return &session, nil
}

func (r *sessionRepository) Refresh(ctx context.Context, token string, expiresAt time.Time) error {
	const query = `
		UPDATE sessions
		SET expires_at = ?
		WHERE token = ?`

	result, err := r.db.ExecContext(ctx, query, expiresAt, token)
	if err != nil {
		return fmt.Errorf("sqlite session refresh: %w", err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("sqlite session refresh rows: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("sqlite session refresh: session not found")
	}
	return nil
}

func (r *sessionRepository) Delete(ctx context.Context, token string) error {
	const query = `DELETE FROM sessions WHERE token = ?`
	if _, err := r.db.ExecContext(ctx, query, token); err != nil {
		return fmt.Errorf("sqlite session delete: %w", err)
	}
	return nil
}

func (r *sessionRepository) PurgeExpired(ctx context.Context) error {
	const query = `DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("sqlite session purge expired: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected > 0 {
		fmt.Printf("purged %d expired sessions\n", affected)
	}
	return nil
}

type userRepository struct {
	db *sql.DB
}

func (r *userRepository) Create(ctx context.Context, user *domain.User) (int64, error) {
	const query = `
		INSERT INTO users (username, password_hash)
		VALUES (?, ?)
		RETURNING id`

	var id int64
	err := r.db.QueryRowContext(ctx, query, user.Username, user.PasswordHash).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("sqlite user create: %w", err)
	}
	return id, nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	const query = `
		SELECT id, username, password_hash, created_at
		FROM users
		WHERE username = ?`

	var user domain.User
	err := r.db.QueryRowContext(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("sqlite user get by username: %w", err)
	}
	return &user, nil
}

func (r *userRepository) Count(ctx context.Context) (int, error) {
	const query = `SELECT COUNT(*) FROM users`

	var count int
	if err := r.db.QueryRowContext(ctx, query).Scan(&count); err != nil {
		return 0, fmt.Errorf("sqlite user count: %w", err)
	}
	return count, nil
}
