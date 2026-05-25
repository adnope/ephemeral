package usecase

import "errors"

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrEmptyMessage       = errors.New("empty message")
	ErrEmptyQuery         = errors.New("empty query")
	ErrNotFound           = errors.New("not found")
	ErrForbidden          = errors.New("forbidden")
	ErrUnsupportedPreview = errors.New("unsupported preview")
	ErrUnsupportedShare   = errors.New("unsupported public share")
	ErrPreviewTooLarge    = errors.New("preview too large")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrMissingCredentials = errors.New("missing credentials")
	ErrUserCreationFailed = errors.New("user creation failed")
)
