package usecase

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

const testPublicToken = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestCreatePublicLinkRejectsTextItem(t *testing.T) {
	uc := newPublicLinkTestUseCase(map[int64]*domain.Item{
		1: {ID: 1, Type: domain.ItemTypeText, Content: "hello"},
	})

	_, err := uc.CreatePublicLink(context.Background(), 1, nil)
	if !errors.Is(err, ErrUnsupportedShare) {
		t.Fatalf("CreatePublicLink error = %v, want ErrUnsupportedShare", err)
	}
}

func TestCreatePublicLinkCreatesNonExpiringFileLink(t *testing.T) {
	uc := newPublicLinkTestUseCase(map[int64]*domain.Item{
		2: {
			ID:       2,
			Type:     domain.ItemTypeFile,
			Content:  "sample.txt",
			Filename: "sample.txt",
			Metadata: domain.Metadata{MIME: "text/plain"},
		},
	})

	link, err := uc.CreatePublicLink(context.Background(), 2, nil)
	if err != nil {
		t.Fatalf("CreatePublicLink(): %v", err)
	}

	if !strings.HasPrefix(link.URL, "/share/") {
		t.Fatalf("URL = %q, want /share/... path", link.URL)
	}
	if link.Token == "" {
		t.Fatal("Token is empty")
	}
	if link.ExpiresAt != nil {
		t.Fatalf("ExpiresAt = %v, want nil", link.ExpiresAt)
	}
}

func TestPublicShareViewExpiresLinks(t *testing.T) {
	now := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	repo := newFakePublicLinkRepo()
	repo.byToken[testPublicToken] = &domain.PublicLink{
		Token:     testPublicToken,
		ItemID:    3,
		ExpiresAt: ptrTime(now.Add(time.Second)),
	}
	uc := newPublicLinkTestUseCaseWithRepo(map[int64]*domain.Item{
		3: {
			ID:       3,
			Type:     domain.ItemTypeImage,
			Content:  "photo.jpg",
			Filename: "photo.jpg",
			Metadata: domain.Metadata{MIME: "image/jpeg"},
		},
	}, repo)
	uc.now = func() time.Time { return now.Add(2 * time.Second) }

	_, err := uc.PublicShareView(context.Background(), testPublicToken)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("PublicShareView error = %v, want ErrNotFound", err)
	}
	if _, err := repo.GetByToken(context.Background(), testPublicToken); err == nil {
		t.Fatal("expired public link was not deleted")
	}
}

func TestPublicSharedFileUsesPlaybackForDisplayAndOriginalForDownload(t *testing.T) {
	repo := newFakePublicLinkRepo()
	repo.byToken[testPublicToken] = &domain.PublicLink{
		Token:  testPublicToken,
		ItemID: 4,
	}
	uc := newPublicLinkTestUseCaseWithRepo(map[int64]*domain.Item{
		4: {
			ID:       4,
			Type:     domain.ItemTypeVideo,
			Content:  "clip.mov",
			Filename: "clip.mov",
			Metadata: domain.Metadata{
				MIME:         "video/quicktime",
				Playback:     "playback/clip_playback.mp4",
				PlaybackMIME: "video/mp4",
				Thumb:        "thumbs/clip_thumb.jpg",
			},
		},
	}, repo)

	display, err := uc.PublicSharedFile(context.Background(), testPublicToken, "display")
	if err != nil {
		t.Fatalf("PublicSharedFile(display): %v", err)
	}
	if display.RelPath != "playback/clip_playback.mp4" || display.MIME != "video/mp4" || !display.Inline {
		t.Fatalf("display file = %#v", display)
	}

	download, err := uc.PublicSharedFile(context.Background(), testPublicToken, "download")
	if err != nil {
		t.Fatalf("PublicSharedFile(download): %v", err)
	}
	if download.RelPath != "clip.mov" || download.MIME != "video/quicktime" || download.Inline {
		t.Fatalf("download file = %#v", download)
	}

	thumb, err := uc.PublicSharedFile(context.Background(), testPublicToken, "thumb")
	if err != nil {
		t.Fatalf("PublicSharedFile(thumb): %v", err)
	}
	if thumb.RelPath != "thumbs/clip_thumb.jpg" || thumb.MIME != "image/jpeg" || !thumb.Inline {
		t.Fatalf("thumb file = %#v", thumb)
	}
}

func newPublicLinkTestUseCase(items map[int64]*domain.Item) *ItemUseCase {
	return newPublicLinkTestUseCaseWithRepo(items, newFakePublicLinkRepo())
}

func newPublicLinkTestUseCaseWithRepo(items map[int64]*domain.Item, public *fakePublicLinkRepo) *ItemUseCase {
	uc := NewItemUseCase(
		&fakePublicLinkItemRepo{items: items},
		public,
		nil,
		nil,
		nil,
		fakePublicLinkStorage{},
		nil,
		nil,
	)
	uc.now = func() time.Time { return time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC) }
	return uc
}

type fakePublicLinkItemRepo struct {
	items map[int64]*domain.Item
}

func (r *fakePublicLinkItemRepo) Create(context.Context, *domain.Item) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *fakePublicLinkItemRepo) GetByID(_ context.Context, id int64) (*domain.Item, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return item, nil
}

func (r *fakePublicLinkItemRepo) List(context.Context, domain.ListFilter) ([]*domain.Item, error) {
	return nil, errors.New("not implemented")
}

func (r *fakePublicLinkItemRepo) Delete(context.Context, int64) error {
	return errors.New("not implemented")
}

func (r *fakePublicLinkItemRepo) Search(context.Context, string, int) ([]*domain.Item, error) {
	return nil, errors.New("not implemented")
}

func (r *fakePublicLinkItemRepo) MediaHistory(context.Context, []string, int64, int) ([]*domain.Item, error) {
	return nil, errors.New("not implemented")
}

func (r *fakePublicLinkItemRepo) UpdateMetadata(context.Context, int64, domain.Metadata) error {
	return errors.New("not implemented")
}

type fakePublicLinkRepo struct {
	byToken map[string]*domain.PublicLink
	byItem  map[int64]string
}

func newFakePublicLinkRepo() *fakePublicLinkRepo {
	return &fakePublicLinkRepo{
		byToken: make(map[string]*domain.PublicLink),
		byItem:  make(map[int64]string),
	}
}

func (r *fakePublicLinkRepo) UpsertForItem(_ context.Context, link *domain.PublicLink) (*domain.PublicLink, error) {
	if oldToken := r.byItem[link.ItemID]; oldToken != "" {
		delete(r.byToken, oldToken)
	}
	stored := clonePublicLink(link)
	stored.CreatedAt = time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	stored.UpdatedAt = stored.CreatedAt
	r.byToken[stored.Token] = stored
	r.byItem[stored.ItemID] = stored.Token
	return clonePublicLink(stored), nil
}

func (r *fakePublicLinkRepo) GetByToken(_ context.Context, token string) (*domain.PublicLink, error) {
	link, ok := r.byToken[token]
	if !ok {
		return nil, errors.New("not found")
	}
	return clonePublicLink(link), nil
}

func (r *fakePublicLinkRepo) DeleteForItem(_ context.Context, itemID int64) error {
	if token := r.byItem[itemID]; token != "" {
		delete(r.byToken, token)
		delete(r.byItem, itemID)
	}
	return nil
}

func (r *fakePublicLinkRepo) DeleteByToken(_ context.Context, token string) error {
	link, ok := r.byToken[token]
	if ok {
		delete(r.byItem, link.ItemID)
	}
	delete(r.byToken, token)
	return nil
}

type fakePublicLinkStorage struct{}

func (fakePublicLinkStorage) Save(context.Context, domain.UploadFile) (domain.StoredUpload, error) {
	return domain.StoredUpload{}, errors.New("not implemented")
}

func (fakePublicLinkStorage) Path(content string) (string, error) {
	if content == "" || strings.Contains(content, "..") || filepath.IsAbs(content) {
		return "", errors.New("unsafe upload path")
	}
	return filepath.Join("/uploads", content), nil
}

func (fakePublicLinkStorage) Remove(string) error {
	return errors.New("not implemented")
}

func (fakePublicLinkStorage) RemoveTree(string) error {
	return errors.New("not implemented")
}

func (fakePublicLinkStorage) ReadLimited(string, int64) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func clonePublicLink(link *domain.PublicLink) *domain.PublicLink {
	if link == nil {
		return nil
	}
	cloned := *link
	if link.ExpiresAt != nil {
		expiresAt := *link.ExpiresAt
		cloned.ExpiresAt = &expiresAt
	}
	return &cloned
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
