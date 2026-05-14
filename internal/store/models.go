package store

import (
	"database/sql"
	"encoding/json"
	"time"
)

// Item represents a shared text message or file in the chat stream.
type Item struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"` // text, image, video, file
	Content   string    `json:"content"`
	Filename  string    `json:"filename,omitempty"`
	Filesize  int64     `json:"filesize,omitempty"`
	Metadata  Metadata  `json:"metadata"`
	CreatedAt time.Time `json:"created_at"`
}

// Metadata holds type-specific attributes stored as JSON in SQLite.
type Metadata struct {
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Duration string `json:"duration,omitempty"`
	MIME     string `json:"mime,omitempty"`
	Thumb    string `json:"thumb,omitempty"`
}

// Scan implements sql.Scanner for reading JSON metadata from SQLite.
func (m *Metadata) Scan(src interface{}) error {
	var data []byte
	switch v := src.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return nil
	}
	return json.Unmarshal(data, m)
}

// Value implements driver.Valuer for writing JSON metadata to SQLite.
func (m Metadata) Value() (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}", err
	}
	return string(b), nil
}

// Session represents an authenticated user session.
type Session struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// User represents a registered user.
type User struct {
	ID           int64          `json:"id"`
	Username     string         `json:"username"`
	PasswordHash string         `json:"-"`
	CreatedAt    time.Time      `json:"created_at"`
}

// ListFilter defines cursor-based pagination parameters.
type ListFilter struct {
	Cursor int64
	Limit  int
}

// NullableString wraps sql.NullString for scanning nullable TEXT columns.
type NullableString = sql.NullString
