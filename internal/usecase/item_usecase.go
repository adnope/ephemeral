package usecase

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/url"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/adnope/ephemeral/internal/domain"
)

type ItemUseCase struct {
	items      domain.ItemRepository
	broker     domain.EventBroker
	media      domain.MediaService
	search     domain.SearchService
	storage    domain.UploadStorage
	classifier domain.MediaClassifier
	log        *slog.Logger
	now        func() time.Time
}

type ItemPage struct {
	Items      []*domain.Item
	NextCursor int64
}

type FilePreview struct {
	ID          int64
	Filename    string
	MIME        string
	Language    string
	Content     string
	Filesize    int64
	CreatedAt   time.Time
	DownloadURL string
}

func NewItemUseCase(
	items domain.ItemRepository,
	broker domain.EventBroker,
	media domain.MediaService,
	search domain.SearchService,
	storage domain.UploadStorage,
	classifier domain.MediaClassifier,
	log *slog.Logger,
) *ItemUseCase {
	if log == nil {
		log = slog.Default()
	}

	return &ItemUseCase{
		items:      items,
		broker:     broker,
		media:      media,
		search:     search,
		storage:    storage,
		classifier: classifier,
		log:        log,
		now:        time.Now,
	}
}

func (uc *ItemUseCase) List(ctx context.Context, cursor int64, limit int) (ItemPage, error) {
	if cursor == 0 {
		cursor = math.MaxInt64
	}
	if limit <= 0 {
		return ItemPage{}, fmt.Errorf("%w: limit must be positive", ErrInvalidInput)
	}

	items, err := uc.items.List(ctx, domain.ListFilter{Cursor: cursor, Limit: limit})
	if err != nil {
		return ItemPage{}, fmt.Errorf("list items: %w", err)
	}

	return ItemPage{
		Items:      items,
		NextCursor: nextCursor(items, limit),
	}, nil
}

func (uc *ItemUseCase) SearchItems(ctx context.Context, query string, limit int) ([]*domain.Item, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, ErrEmptyQuery
	}
	if limit <= 0 {
		return nil, fmt.Errorf("%w: limit must be positive", ErrInvalidInput)
	}

	items, err := uc.items.Search(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	return items, nil
}

func (uc *ItemUseCase) CreateMessage(ctx context.Context, text string) (*domain.Item, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, ErrEmptyMessage
	}

	item := &domain.Item{
		Type:    domain.ItemTypeText,
		Content: text,
	}

	id, err := uc.items.Create(ctx, item)
	if err != nil {
		return nil, fmt.Errorf("create message item: %w", err)
	}

	item.ID = id
	item.CreatedAt = uc.now().UTC()
	uc.broadcast(domain.Event{Type: "item:new", ID: id})
	return item, nil
}

func (uc *ItemUseCase) UploadFile(ctx context.Context, filename string, reader io.Reader) (*domain.Item, error) {
	if reader == nil {
		return nil, fmt.Errorf("%w: upload reader is nil", ErrInvalidInput)
	}

	stored, err := uc.storage.Save(ctx, domain.UploadFile{
		Name:   filename,
		Reader: reader,
	})
	if err != nil {
		return nil, fmt.Errorf("save upload: %w", err)
	}

	created := false
	defer func() {
		if !created {
			if removeErr := uc.storage.Remove(stored.StoredName); removeErr != nil {
				uc.log.Warn("upload rollback remove failed", "content", stored.StoredName, "err", removeErr)
			}
		}
	}()

	mimeType, err := uc.classifier.DetectMIME(stored.AbsolutePath)
	if err != nil {
		mimeType = "application/octet-stream"
	}

	item := &domain.Item{
		Type:     uc.classifier.ItemTypeFromMIME(mimeType),
		Content:  stored.StoredName,
		Filename: stored.OriginalName,
		Filesize: stored.Size,
		Metadata: domain.Metadata{MIME: mimeType},
	}
	if item.Type == domain.ItemTypeVideo {
		item.Metadata.Processing = true
	}

	id, err := uc.items.Create(ctx, item)
	if err != nil {
		return nil, fmt.Errorf("create uploaded item: %w", err)
	}
	created = true

	item.ID = id
	item.CreatedAt = uc.now().UTC()

	uc.indexUploadedFile(id, item)
	uc.broadcast(domain.Event{Type: "item:new", ID: id})
	uc.enqueueMedia(domain.MediaJob{
		ItemID:   id,
		FilePath: stored.AbsolutePath,
		MIMEType: mimeType,
		Size:     stored.Size,
	})

	return item, nil
}

func (uc *ItemUseCase) DeleteItem(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: item id must be positive", ErrInvalidInput)
	}

	item, err := uc.items.GetByID(ctx, id)
	if err != nil {
		return ErrNotFound
	}

	if err := uc.items.Delete(ctx, id); err != nil {
		return fmt.Errorf("delete item: %w", err)
	}

	if item.Type != domain.ItemTypeText {
		uc.removeUploadBestEffort(item.Content)
		if item.Metadata.Thumb != "" {
			uc.removeUploadBestEffort(item.Metadata.Thumb)
		}
		if item.Metadata.Playback != "" {
			uc.removeUploadBestEffort(item.Metadata.Playback)
		}
		if hlsDir := hlsUploadDir(item.Metadata.HLS); hlsDir != "" {
			uc.removeUploadTreeBestEffort(hlsDir)
		}
	}

	uc.broadcast(domain.Event{Type: "item:deleted", ID: id})
	return nil
}

func (uc *ItemUseCase) PreviewFile(ctx context.Context, id int64, maxBytes int64) (FilePreview, error) {
	if id <= 0 {
		return FilePreview{}, fmt.Errorf("%w: item id must be positive", ErrInvalidInput)
	}
	if maxBytes <= 0 {
		return FilePreview{}, fmt.Errorf("%w: max preview bytes must be positive", ErrInvalidInput)
	}

	item, err := uc.items.GetByID(ctx, id)
	if err != nil {
		return FilePreview{}, ErrNotFound
	}

	if item.Type != domain.ItemTypeFile {
		return FilePreview{}, fmt.Errorf("%w: preview only supports generic files", ErrUnsupportedPreview)
	}

	if !isPreviewableTextFile(item.Filename, item.Metadata.MIME) {
		return FilePreview{}, fmt.Errorf("%w: file type is not previewable as text", ErrUnsupportedPreview)
	}

	if item.Filesize > maxBytes {
		return FilePreview{}, ErrPreviewTooLarge
	}

	content, err := uc.storage.ReadLimited(item.Content, maxBytes)
	if err != nil {
		if strings.Contains(err.Error(), "unsafe upload path") {
			return FilePreview{}, ErrForbidden
		}
		if strings.Contains(err.Error(), "exceeds max read size") {
			return FilePreview{}, ErrPreviewTooLarge
		}
		return FilePreview{}, fmt.Errorf("read preview file: %w", err)
	}

	if !utf8.Valid(content) {
		return FilePreview{}, fmt.Errorf("%w: file is not valid utf-8 text", ErrUnsupportedPreview)
	}

	return FilePreview{
		ID:          item.ID,
		Filename:    item.Filename,
		MIME:        item.Metadata.MIME,
		Language:    previewLanguage(item.Filename, item.Metadata.MIME),
		Content:     string(content),
		Filesize:    item.Filesize,
		CreatedAt:   item.CreatedAt,
		DownloadURL: "/api/files/" + url.PathEscape(item.Content),
	}, nil
}

func (uc *ItemUseCase) ResolveUploadPath(content string) (string, error) {
	path, err := uc.storage.Path(content)
	if err != nil {
		return "", ErrForbidden
	}
	return path, nil
}

func (uc *ItemUseCase) indexUploadedFile(itemID int64, item *domain.Item) {
	if uc.search == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := uc.search.IndexUploadedFile(
			ctx,
			itemID,
			item.Content,
			item.Filename,
			item.Filesize,
			item.Metadata,
		)
		if err != nil {
			uc.log.Warn("upload body index failed", "item_id", itemID, "err", err)
		}
	}()
}

func (uc *ItemUseCase) enqueueMedia(job domain.MediaJob) {
	if uc.media != nil {
		uc.media.Enqueue(job)
	}
}

func (uc *ItemUseCase) broadcast(event domain.Event) {
	if uc.broker != nil {
		uc.broker.Broadcast(event)
	}
}

func (uc *ItemUseCase) removeUploadBestEffort(content string) {
	if err := uc.storage.Remove(content); err != nil {
		uc.log.Warn("delete item remove upload failed", "content", content, "err", err)
	}
}

func (uc *ItemUseCase) removeUploadTreeBestEffort(content string) {
	if err := uc.storage.RemoveTree(content); err != nil {
		uc.log.Warn("delete item remove upload tree failed", "content", content, "err", err)
	}
}

func hlsUploadDir(playlist string) string {
	cleanPath := pathpkg.Clean(playlist)
	if cleanPath == "." || !strings.HasPrefix(cleanPath, "hls/") {
		return ""
	}

	dir := pathpkg.Dir(cleanPath)
	if dir == "." || dir == "hls" {
		return ""
	}
	return dir
}

func nextCursor(items []*domain.Item, limit int) int64 {
	if len(items) == limit {
		return items[len(items)-1].ID
	}
	return 0
}

func isPreviewableTextFile(filename string, mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}

	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	switch mimeType {
	case "application/json",
		"application/xml",
		"application/javascript",
		"application/x-javascript",
		"application/x-sh",
		"application/sql",
		"image/svg+xml":
		return true
	}

	_, ok := previewLangByExt[strings.ToLower(filepath.Ext(filename))]
	return ok
}

func previewLanguage(filename string, mimeType string) string {
	if lang, ok := previewLangByExt[strings.ToLower(filepath.Ext(filename))]; ok {
		return lang
	}

	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	if i := strings.IndexByte(mimeType, ';'); i >= 0 {
		mimeType = mimeType[:i]
	}

	if lang, ok := previewLangByMIME[mimeType]; ok {
		return lang
	}
	return "plaintext"
}

var previewLangByExt = map[string]string{
	".txt":        "plaintext",
	".log":        "plaintext",
	".md":         "markdown",
	".markdown":   "markdown",
	".mk":         "make",
	".mak":        "make",
	".make":       "make",
	".go":         "go",
	".py":         "python",
	".js":         "javascript",
	".mjs":        "javascript",
	".cjs":        "javascript",
	".ts":         "typescript",
	".tsx":        "tsx",
	".jsx":        "jsx",
	".json":       "json",
	".yaml":       "yaml",
	".yml":        "yaml",
	".toml":       "toml",
	".xml":        "xml",
	".html":       "html",
	".css":        "css",
	".scss":       "scss",
	".csv":        "csv",
	".sql":        "sql",
	".sh":         "shellscript",
	".bash":       "shellscript",
	".zsh":        "shellscript",
	".rs":         "rust",
	".c":          "c",
	".h":          "c",
	".cpp":        "cpp",
	".hpp":        "cpp",
	".java":       "java",
	".kt":         "kotlin",
	".rb":         "ruby",
	".php":        "php",
	".lua":        "lua",
	".dockerfile": "dockerfile",
}

var previewLangByMIME = map[string]string{
	"text/plain":               "plaintext",
	"text/markdown":            "markdown",
	"text/javascript":          "javascript",
	"text/typescript":          "typescript",
	"text/yaml":                "yaml",
	"text/html":                "html",
	"text/css":                 "css",
	"text/csv":                 "csv",
	"text/xml":                 "xml",
	"application/json":         "json",
	"application/xml":          "xml",
	"application/javascript":   "javascript",
	"application/x-javascript": "javascript",
	"application/sql":          "sql",
	"application/x-sh":         "shellscript",
	"image/svg+xml":            "xml",
}
