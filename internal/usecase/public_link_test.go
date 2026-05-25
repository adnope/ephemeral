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

func TestCreatePublicLinkUpdatesActiveLinkWithoutChangingToken(t *testing.T) {
	now := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	repo := newFakePublicLinkRepo()
	repo.byToken[testPublicToken] = &domain.PublicLink{
		Token:     testPublicToken,
		ItemID:    3,
		ExpiresAt: ptrTime(now.Add(time.Hour)),
	}
	repo.byItem[3] = testPublicToken
	uc := newPublicLinkTestUseCaseWithRepo(map[int64]*domain.Item{
		3: {
			ID:       3,
			Type:     domain.ItemTypeFile,
			Content:  "sample.txt",
			Filename: "sample.txt",
			Metadata: domain.Metadata{MIME: "text/plain"},
		},
	}, repo)
	uc.now = func() time.Time { return now }

	expiresIn := 24 * time.Hour
	link, err := uc.CreatePublicLink(context.Background(), 3, &expiresIn)
	if err != nil {
		t.Fatalf("CreatePublicLink(): %v", err)
	}

	if link.Token != testPublicToken {
		t.Fatalf("Token = %q, want existing token %q", link.Token, testPublicToken)
	}
	if link.ExpiresAt == nil || !link.ExpiresAt.Equal(now.Add(expiresIn)) {
		t.Fatalf("ExpiresAt = %v, want %v", link.ExpiresAt, now.Add(expiresIn))
	}
}

func TestCreatePublicLinkReplacesExpiredLinkWithNewToken(t *testing.T) {
	now := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	repo := newFakePublicLinkRepo()
	repo.byToken[testPublicToken] = &domain.PublicLink{
		Token:     testPublicToken,
		ItemID:    4,
		ExpiresAt: ptrTime(now.Add(-time.Second)),
	}
	repo.byItem[4] = testPublicToken
	uc := newPublicLinkTestUseCaseWithRepo(map[int64]*domain.Item{
		4: {
			ID:       4,
			Type:     domain.ItemTypeFile,
			Content:  "sample.txt",
			Filename: "sample.txt",
			Metadata: domain.Metadata{MIME: "text/plain"},
		},
	}, repo)
	uc.now = func() time.Time { return now }

	link, err := uc.CreatePublicLink(context.Background(), 4, nil)
	if err != nil {
		t.Fatalf("CreatePublicLink(): %v", err)
	}

	if link.Token == "" || link.Token == testPublicToken {
		t.Fatalf("Token = %q, want a new token", link.Token)
	}
	if _, err := repo.GetByToken(context.Background(), testPublicToken); !errors.Is(err, domain.ErrPublicLinkNotFound) {
		t.Fatalf("old token error = %v, want ErrPublicLinkNotFound", err)
	}
}

func TestPublicShareViewExpiresLinksWithoutDeletingThem(t *testing.T) {
	now := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	repo := newFakePublicLinkRepo()
	repo.byToken[testPublicToken] = &domain.PublicLink{
		Token:     testPublicToken,
		ItemID:    5,
		ExpiresAt: ptrTime(now.Add(time.Second)),
	}
	repo.byItem[5] = testPublicToken
	uc := newPublicLinkTestUseCaseWithRepo(map[int64]*domain.Item{
		5: {
			ID:       5,
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
	if _, err := repo.GetByToken(context.Background(), testPublicToken); err != nil {
		t.Fatalf("expired public link was deleted: %v", err)
	}
}

func TestPublicLinkStatusClassifiesLinkState(t *testing.T) {
	now := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	repo := newFakePublicLinkRepo()
	repo.byToken["active-token"] = &domain.PublicLink{
		Token:     "active-token",
		ItemID:    6,
		ExpiresAt: ptrTime(now.Add(time.Hour)),
	}
	repo.byItem[6] = "active-token"
	repo.byToken["expired-token"] = &domain.PublicLink{
		Token:     "expired-token",
		ItemID:    7,
		ExpiresAt: ptrTime(now.Add(-time.Second)),
	}
	repo.byItem[7] = "expired-token"
	uc := newPublicLinkTestUseCaseWithRepo(map[int64]*domain.Item{
		6: {
			ID:       6,
			Type:     domain.ItemTypeImage,
			Content:  "active.jpg",
			Filename: "active.jpg",
			Metadata: domain.Metadata{MIME: "image/jpeg"},
		},
		7: {
			ID:       7,
			Type:     domain.ItemTypeImage,
			Content:  "expired.jpg",
			Filename: "expired.jpg",
			Metadata: domain.Metadata{MIME: "image/jpeg"},
		},
		8: {
			ID:       8,
			Type:     domain.ItemTypeFile,
			Content:  "none.txt",
			Filename: "none.txt",
			Metadata: domain.Metadata{MIME: "text/plain"},
		},
	}, repo)
	uc.now = func() time.Time { return now }

	active, err := uc.PublicLinkStatus(context.Background(), 6)
	if err != nil {
		t.Fatalf("PublicLinkStatus(active): %v", err)
	}
	if active.State != PublicLinkStateActive || active.Token != "active-token" {
		t.Fatalf("active status = %#v", active)
	}

	expired, err := uc.PublicLinkStatus(context.Background(), 7)
	if err != nil {
		t.Fatalf("PublicLinkStatus(expired): %v", err)
	}
	if expired.State != PublicLinkStateExpired || expired.Token != "expired-token" {
		t.Fatalf("expired status = %#v", expired)
	}

	none, err := uc.PublicLinkStatus(context.Background(), 8)
	if err != nil {
		t.Fatalf("PublicLinkStatus(none): %v", err)
	}
	if none.State != PublicLinkStateNone {
		t.Fatalf("none status = %#v", none)
	}
}

func TestActivePublicLinkItemIDsReturnsOnlyUnexpiredUploadedItems(t *testing.T) {
	now := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	repo := newFakePublicLinkRepo()
	repo.byToken["active-token"] = &domain.PublicLink{
		Token:     "active-token",
		ItemID:    9,
		ExpiresAt: ptrTime(now.Add(time.Hour)),
	}
	repo.byItem[9] = "active-token"
	repo.byToken["expired-token"] = &domain.PublicLink{
		Token:     "expired-token",
		ItemID:    10,
		ExpiresAt: ptrTime(now.Add(-time.Second)),
	}
	repo.byItem[10] = "expired-token"
	repo.byToken["text-token"] = &domain.PublicLink{
		Token:  "text-token",
		ItemID: 11,
	}
	repo.byItem[11] = "text-token"

	uc := newPublicLinkTestUseCaseWithRepo(map[int64]*domain.Item{}, repo)
	uc.now = func() time.Time { return now }

	active, err := uc.ActivePublicLinkItemIDs(context.Background(), []*domain.Item{
		{ID: 9, Type: domain.ItemTypeFile},
		{ID: 10, Type: domain.ItemTypeFile},
		{ID: 11, Type: domain.ItemTypeText},
		{ID: 12, Type: domain.ItemTypeImage},
	})
	if err != nil {
		t.Fatalf("ActivePublicLinkItemIDs(): %v", err)
	}

	if !active[9] {
		t.Fatal("active item 9 missing")
	}
	if active[10] {
		t.Fatal("expired item 10 marked active")
	}
	if active[11] {
		t.Fatal("text item 11 marked active")
	}
	if active[12] {
		t.Fatal("unlinked item 12 marked active")
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
		return nil, domain.ErrPublicLinkNotFound
	}
	return clonePublicLink(link), nil
}

func (r *fakePublicLinkRepo) GetForItem(_ context.Context, itemID int64) (*domain.PublicLink, error) {
	token := r.byItem[itemID]
	if token == "" {
		return nil, domain.ErrPublicLinkNotFound
	}
	return r.GetByToken(context.Background(), token)
}

func (r *fakePublicLinkRepo) ActiveItemIDs(_ context.Context, itemIDs []int64, now time.Time) (map[int64]bool, error) {
	active := make(map[int64]bool)
	for _, itemID := range itemIDs {
		token := r.byItem[itemID]
		if token == "" {
			continue
		}
		link := r.byToken[token]
		if link == nil || (link.ExpiresAt != nil && !now.UTC().Before(link.ExpiresAt.UTC())) {
			continue
		}
		active[itemID] = true
	}
	return active, nil
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
