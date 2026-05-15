package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

type AuthUseCase struct {
	users      domain.UserRepository
	sessions   domain.SessionRepository
	sessionTTL time.Duration
	now        func() time.Time
}

type LoginPageData struct {
	IsSetup bool
}

type LoginResult struct {
	Token string
	TTL   time.Duration
}

func NewAuthUseCase(
	users domain.UserRepository,
	sessions domain.SessionRepository,
	sessionTTL time.Duration,
) *AuthUseCase {
	return &AuthUseCase{
		users:      users,
		sessions:   sessions,
		sessionTTL: sessionTTL,
		now:        time.Now,
	}
}

func (uc *AuthUseCase) LoginPage(ctx context.Context) (LoginPageData, error) {
	count, err := uc.users.Count(ctx)
	if err != nil {
		return LoginPageData{}, fmt.Errorf("count users: %w", err)
	}
	return LoginPageData{IsSetup: count == 0}, nil
}

func (uc *AuthUseCase) Login(ctx context.Context, username string, password string) (LoginResult, error) {
	if username == "" || password == "" {
		return LoginResult{}, ErrMissingCredentials
	}

	count, err := uc.users.Count(ctx)
	if err != nil {
		return LoginResult{}, fmt.Errorf("count users: %w", err)
	}

	if count == 0 {
		if err := uc.createInitialUser(ctx, username, password); err != nil {
			return LoginResult{}, fmt.Errorf("%w: %w", ErrUserCreationFailed, err)
		}
	}

	user, err := uc.users.GetByUsername(ctx, username)
	if err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	token, err := generateSessionToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate session token: %w", err)
	}

	session := &domain.Session{
		Token:     token,
		UserID:    user.ID,
		ExpiresAt: uc.now().Add(uc.sessionTTL),
	}

	if err := uc.sessions.Create(ctx, session); err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	return LoginResult{Token: token, TTL: uc.sessionTTL}, nil
}

func (uc *AuthUseCase) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := uc.sessions.Delete(ctx, token); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (uc *AuthUseCase) createInitialUser(ctx context.Context, username string, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = uc.users.Create(ctx, &domain.User{
		Username:     username,
		PasswordHash: string(hash),
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func generateSessionToken() (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(tokenBytes), nil
}
