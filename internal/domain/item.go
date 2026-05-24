package domain

import (
	"context"
	"io"
	"time"
)

const (
	ItemTypeText  = "text"
	ItemTypeImage = "image"
	ItemTypeVideo = "video"
	ItemTypeFile  = "file"
)

type Item struct {
	ID        int64
	Type      string
	Content   string
	Filename  string
	Filesize  int64
	Metadata  Metadata
	CreatedAt time.Time
}

type Metadata struct {
	Width        int
	Height       int
	Duration     string
	MIME         string
	Thumb        string
	Playback     string
	PlaybackMIME string
	HLS          string
	Processing   bool
}

type ListFilter struct {
	Cursor int64
	Limit  int
}

type ItemRepository interface {
	Create(ctx context.Context, item *Item) (int64, error)
	GetByID(ctx context.Context, id int64) (*Item, error)
	List(ctx context.Context, filter ListFilter) ([]*Item, error)
	Delete(ctx context.Context, id int64) error
	Search(ctx context.Context, query string, limit int) ([]*Item, error)
	MediaHistory(ctx context.Context, types []string, cursor int64, limit int) ([]*Item, error)
	UpdateMetadata(ctx context.Context, id int64, meta Metadata) error
}

type PublicLink struct {
	Token     string
	ItemID    int64
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PublicLinkRepository interface {
	UpsertForItem(ctx context.Context, link *PublicLink) (*PublicLink, error)
	GetByToken(ctx context.Context, token string) (*PublicLink, error)
	DeleteForItem(ctx context.Context, itemID int64) error
	DeleteByToken(ctx context.Context, token string) error
}

type Event struct {
	Type string
	ID   int64
}

type EventBroker interface {
	Broadcast(event Event)
}

type MediaJob struct {
	ItemID   int64
	FilePath string
	MIMEType string
	Size     int64
}

type MediaService interface {
	Enqueue(job MediaJob)
}

type MediaClassifier interface {
	DetectMIME(path string) (string, error)
	ItemTypeFromMIME(mimeType string) string
}

type UploadFile struct {
	Name   string
	Reader io.Reader
}

type StoredUpload struct {
	OriginalName string
	StoredName   string
	AbsolutePath string
	Size         int64
}

type UploadStorage interface {
	Save(ctx context.Context, file UploadFile) (StoredUpload, error)
	Path(content string) (string, error)
	Remove(content string) error
	RemoveTree(content string) error
	ReadLimited(content string, maxBytes int64) ([]byte, error)
}
