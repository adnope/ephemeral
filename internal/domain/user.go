package domain

import (
	"context"
	"time"
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type UserRepository interface {
	Create(ctx context.Context, user *User) (int64, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	UpdatePassword(ctx context.Context, id int64, oldHash, newHash string) (bool, error)
	Count(ctx context.Context) (int, error)
}
