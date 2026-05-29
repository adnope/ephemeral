package usecase

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
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
	public     domain.PublicLinkRepository
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

type PublicLinkResult struct {
	Token     string
	URL       string
	ExpiresAt *time.Time
}

type PublicLinkState string

const (
	PublicLinkStateNone    PublicLinkState = "none"
	PublicLinkStateActive  PublicLinkState = "active"
	PublicLinkStateExpired PublicLinkState = "expired"
)

type PublicLinkStatus struct {
	State     PublicLinkState
	Token     string
	URL       string
	ExpiresAt *time.Time
}

type PublicShareView struct {
	Token       string
	Item        *domain.Item
	SourceURL   string
	PosterURL   string
	DownloadURL string
	DisplayMIME string
	ExpiresAt   *time.Time
}

type PublicSharedFile struct {
	Path     string
	RelPath  string
	Filename string
	MIME     string
	Inline   bool
}

const maxPublicLinkExpiry = 10 * 365 * 24 * time.Hour

func NewItemUseCase(
	items domain.ItemRepository,
	public domain.PublicLinkRepository,
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
		public:     public,
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

func (uc *ItemUseCase) GetItem(ctx context.Context, id int64) (*domain.Item, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: item id must be positive", ErrInvalidInput)
	}
	item, err := uc.items.GetByID(ctx, id)
	if err != nil {
		return nil, ErrNotFound
	}
	return item, nil
}

func (uc *ItemUseCase) DeleteItem(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: item id must be positive", ErrInvalidInput)
	}

	uc.media.CancelJob(id)

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

func (uc *ItemUseCase) CreatePublicLink(ctx context.Context, itemID int64, expiresIn *time.Duration) (PublicLinkResult, error) {
	if itemID <= 0 {
		return PublicLinkResult{}, fmt.Errorf("%w: item id must be positive", ErrInvalidInput)
	}
	if uc.public == nil {
		return PublicLinkResult{}, fmt.Errorf("public link repository is not configured")
	}

	item, err := uc.items.GetByID(ctx, itemID)
	if err != nil {
		return PublicLinkResult{}, ErrNotFound
	}
	if item.Type == domain.ItemTypeText {
		return PublicLinkResult{}, fmt.Errorf("%w: text items cannot be shared as files", ErrUnsupportedShare)
	}

	expiresAt, err := uc.publicLinkExpiresAt(expiresIn)
	if err != nil {
		return PublicLinkResult{}, err
	}

	token, err := uc.publicLinkTokenForUpsert(ctx, item.ID)
	if err != nil {
		return PublicLinkResult{}, err
	}

	link, err := uc.public.UpsertForItem(ctx, &domain.PublicLink{
		Token:     token,
		ItemID:    item.ID,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		return PublicLinkResult{}, fmt.Errorf("save public link: %w", err)
	}

	return publicLinkResult(link), nil
}

func (uc *ItemUseCase) PublicLinkStatus(ctx context.Context, itemID int64) (PublicLinkStatus, error) {
	if itemID <= 0 {
		return PublicLinkStatus{}, fmt.Errorf("%w: item id must be positive", ErrInvalidInput)
	}
	if uc.public == nil {
		return PublicLinkStatus{}, fmt.Errorf("public link repository is not configured")
	}

	item, err := uc.items.GetByID(ctx, itemID)
	if err != nil {
		return PublicLinkStatus{}, ErrNotFound
	}
	if item.Type == domain.ItemTypeText {
		return PublicLinkStatus{}, fmt.Errorf("%w: text items cannot be shared as files", ErrUnsupportedShare)
	}

	link, err := uc.public.GetForItem(ctx, itemID)
	if err != nil {
		if errors.Is(err, domain.ErrPublicLinkNotFound) {
			return PublicLinkStatus{State: PublicLinkStateNone}, nil
		}
		return PublicLinkStatus{}, fmt.Errorf("get public link for item: %w", err)
	}

	state := PublicLinkStateActive
	if publicLinkExpired(link, uc.now()) {
		state = PublicLinkStateExpired
	}

	return PublicLinkStatus{
		State:     state,
		Token:     link.Token,
		URL:       "/share/" + link.Token,
		ExpiresAt: link.ExpiresAt,
	}, nil
}

func (uc *ItemUseCase) ActivePublicLinkItemIDs(ctx context.Context, items []*domain.Item) (map[int64]bool, error) {
	active := make(map[int64]bool)
	if uc.public == nil || len(items) == 0 {
		return active, nil
	}

	itemIDs := make([]int64, 0, len(items))
	for _, item := range items {
		if item == nil || item.ID <= 0 || item.Type == domain.ItemTypeText {
			continue
		}
		itemIDs = append(itemIDs, item.ID)
	}
	if len(itemIDs) == 0 {
		return active, nil
	}

	active, err := uc.public.ActiveItemIDs(ctx, itemIDs, uc.now())
	if err != nil {
		return nil, fmt.Errorf("list active public link item ids: %w", err)
	}
	return active, nil
}

func (uc *ItemUseCase) RevokePublicLink(ctx context.Context, itemID int64) error {
	if itemID <= 0 {
		return fmt.Errorf("%w: item id must be positive", ErrInvalidInput)
	}
	if uc.public == nil {
		return fmt.Errorf("public link repository is not configured")
	}

	if err := uc.public.DeleteForItem(ctx, itemID); err != nil {
		return fmt.Errorf("delete public link: %w", err)
	}
	return nil
}

func (uc *ItemUseCase) PublicShareView(ctx context.Context, token string) (PublicShareView, error) {
	link, item, err := uc.resolvePublicShare(ctx, token)
	if err != nil {
		return PublicShareView{}, err
	}
	if !isBrowserViewablePublicItem(item) {
		return PublicShareView{}, fmt.Errorf("%w: public share is not browser-viewable", ErrUnsupportedShare)
	}

	_, displayMIME := publicDisplayContent(item)
	view := PublicShareView{
		Token:       link.Token,
		Item:        item,
		SourceURL:   "/share/" + link.Token + "/file",
		DownloadURL: "/share/" + link.Token + "/download",
		DisplayMIME: displayMIME,
		ExpiresAt:   link.ExpiresAt,
	}
	if item.Metadata.Thumb != "" {
		view.PosterURL = "/share/" + link.Token + "/thumb"
	}
	return view, nil
}

func (uc *ItemUseCase) PublicSharedFile(ctx context.Context, token string, variant string) (PublicSharedFile, error) {
	_, item, err := uc.resolvePublicShare(ctx, token)
	if err != nil {
		return PublicSharedFile{}, err
	}

	var relPath string
	var mimeType string
	inline := false

	switch variant {
	case "display":
		if !isBrowserViewablePublicItem(item) {
			return PublicSharedFile{}, fmt.Errorf("%w: public share is not browser-viewable", ErrUnsupportedShare)
		}
		relPath, mimeType = publicDisplayContent(item)
		inline = true
	case "download":
		relPath = item.Content
		mimeType = item.Metadata.MIME
	case "thumb":
		if item.Metadata.Thumb == "" {
			return PublicSharedFile{}, ErrNotFound
		}
		relPath = item.Metadata.Thumb
		mimeType = "image/jpeg"
		inline = true
	default:
		return PublicSharedFile{}, fmt.Errorf("%w: invalid public share file variant", ErrInvalidInput)
	}

	path, err := uc.storage.Path(relPath)
	if err != nil {
		return PublicSharedFile{}, ErrForbidden
	}

	return PublicSharedFile{
		Path:     path,
		RelPath:  relPath,
		Filename: item.Filename,
		MIME:     mimeType,
		Inline:   inline,
	}, nil
}

func (uc *ItemUseCase) resolvePublicShare(ctx context.Context, token string) (*domain.PublicLink, *domain.Item, error) {
	if uc.public == nil {
		return nil, nil, fmt.Errorf("public link repository is not configured")
	}
	if !validPublicLinkToken(token) {
		return nil, nil, ErrNotFound
	}

	link, err := uc.public.GetByToken(ctx, token)
	if err != nil {
		return nil, nil, ErrNotFound
	}

	if publicLinkExpired(link, uc.now()) {
		return nil, nil, ErrNotFound
	}

	item, err := uc.items.GetByID(ctx, link.ItemID)
	if err != nil {
		return nil, nil, ErrNotFound
	}
	if item.Type == domain.ItemTypeText {
		return nil, nil, ErrNotFound
	}

	return link, item, nil
}

func (uc *ItemUseCase) publicLinkExpiresAt(expiresIn *time.Duration) (*time.Time, error) {
	if expiresIn == nil {
		return nil, nil
	}
	if *expiresIn <= 0 {
		return nil, fmt.Errorf("%w: expiry must be positive", ErrInvalidInput)
	}
	if *expiresIn > maxPublicLinkExpiry {
		return nil, fmt.Errorf("%w: expiry is too large", ErrInvalidInput)
	}

	value := uc.now().UTC().Add(*expiresIn)
	return &value, nil
}

func (uc *ItemUseCase) publicLinkTokenForUpsert(ctx context.Context, itemID int64) (string, error) {
	existing, err := uc.public.GetForItem(ctx, itemID)
	if err == nil {
		if !publicLinkExpired(existing, uc.now()) {
			return existing.Token, nil
		}
		if err := uc.public.DeleteForItem(ctx, itemID); err != nil {
			return "", fmt.Errorf("delete expired public link before replacement: %w", err)
		}
	} else if !errors.Is(err, domain.ErrPublicLinkNotFound) {
		return "", fmt.Errorf("get public link for replacement: %w", err)
	}

	token, err := generatePublicLinkToken()
	if err != nil {
		return "", fmt.Errorf("generate public link token: %w", err)
	}
	return token, nil
}

func publicLinkExpired(link *domain.PublicLink, now time.Time) bool {
	return link != nil && link.ExpiresAt != nil && !now.UTC().Before(link.ExpiresAt.UTC())
}

func publicLinkResult(link *domain.PublicLink) PublicLinkResult {
	if link == nil {
		return PublicLinkResult{}
	}
	return PublicLinkResult{
		Token:     link.Token,
		URL:       "/share/" + link.Token,
		ExpiresAt: link.ExpiresAt,
	}
}

func generatePublicLinkToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func validPublicLinkToken(token string) bool {
	if len(token) < 32 || len(token) > 128 {
		return false
	}
	for _, r := range token {
		if r >= 'a' && r <= 'z' {
			continue
		}
		if r >= 'A' && r <= 'Z' {
			continue
		}
		if r >= '0' && r <= '9' {
			continue
		}
		if r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func isBrowserViewablePublicItem(item *domain.Item) bool {
	if item == nil {
		return false
	}
	return item.Type == domain.ItemTypeImage || item.Type == domain.ItemTypeVideo
}

func publicDisplayContent(item *domain.Item) (string, string) {
	if item == nil {
		return "", ""
	}
	if item.Type == domain.ItemTypeVideo && item.Metadata.Playback != "" {
		if item.Metadata.PlaybackMIME != "" {
			return item.Metadata.Playback, item.Metadata.PlaybackMIME
		}
		return item.Metadata.Playback, "video/mp4"
	}
	return item.Content, item.Metadata.MIME
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
