package store

import "context"

// ItemRepository defines all persistence operations for items.
// All methods accept a context.Context to support cancellation
// and per-request timeouts.
type ItemRepository interface {
	Create(ctx context.Context, item *Item) (int64, error)
	GetByID(ctx context.Context, id int64) (*Item, error)
	List(ctx context.Context, filter ListFilter) ([]*Item, error)
	Delete(ctx context.Context, id int64) error
	Search(ctx context.Context, query string, limit int) ([]*Item, error)
	MediaHistory(ctx context.Context, types []string, cursor int64, limit int) ([]*Item, error)
	UpdateMetadata(ctx context.Context, id int64, meta Metadata) error
}

// SessionRepository defines persistence operations for user sessions.
type SessionRepository interface {
	Create(ctx context.Context, s *Session) error
	GetByToken(ctx context.Context, token string) (*Session, error)
	Delete(ctx context.Context, token string) error
	PurgeExpired(ctx context.Context) error
}

// UserRepository defines persistence operations for user accounts.
type UserRepository interface {
	Create(ctx context.Context, u *User) (int64, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	Count(ctx context.Context) (int, error)
}
