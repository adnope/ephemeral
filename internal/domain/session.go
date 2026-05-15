package domain

import (
	"context"
	"time"
)

type Session struct {
	Token     string
	UserID    int64
	CreatedAt time.Time
	ExpiresAt time.Time
}

type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	GetByToken(ctx context.Context, token string) (*Session, error)
	Refresh(ctx context.Context, token string, expiresAt time.Time) error
	Delete(ctx context.Context, token string) error
	PurgeExpired(ctx context.Context) error
}
