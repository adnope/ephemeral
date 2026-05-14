package store

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Item struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Filename  string    `json:"filename,omitempty"`
	Filesize  int64     `json:"filesize,omitempty"`
	Metadata  Metadata  `json:"metadata"`
	CreatedAt time.Time `json:"created_at"`
}

type Metadata struct {
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	Duration string `json:"duration,omitempty"`
	MIME     string `json:"mime,omitempty"`
	Thumb    string `json:"thumb,omitempty"`
}

func (m *Metadata) Scan(src any) error {
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

func (m Metadata) Value() (string, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return "{}", err
	}
	return string(b), nil
}

type Session struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type ListFilter struct {
	Cursor int64
	Limit  int
}

type NullableString = sql.NullString
