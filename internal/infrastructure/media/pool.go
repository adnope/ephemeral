package media

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

type Pool struct {
	jobs    chan domain.MediaJob
	repo    domain.ItemRepository
	broker  domain.EventBroker
	storage domain.UploadStorage
	options PoolOptions
	wg      sync.WaitGroup

	mu         sync.Mutex
	activeJobs map[int64]context.CancelFunc
}

type PoolOptions struct {
	WorkerCount    int
	ProcessTimeout time.Duration
	HLSMinBytes    int64
	HLSMinDuration time.Duration
}

func NewPool(repo domain.ItemRepository, broker domain.EventBroker, storage domain.UploadStorage, options PoolOptions) (*Pool, error) {
	if options.WorkerCount <= 0 {
		return nil, fmt.Errorf("media worker count must be positive")
	}
	if options.ProcessTimeout <= 0 {
		return nil, fmt.Errorf("media process timeout must be positive")
	}
	if options.HLSMinBytes < 0 {
		return nil, fmt.Errorf("hls min bytes must be non-negative")
	}
	if options.HLSMinDuration < 0 {
		return nil, fmt.Errorf("hls min duration must be non-negative")
	}

	pool := &Pool{
		jobs:       make(chan domain.MediaJob, 16),
		repo:       repo,
		broker:     broker,
		storage:    storage,
		options:    options,
		activeJobs: make(map[int64]context.CancelFunc),
	}
	pool.wg.Add(options.WorkerCount)
	for i := 0; i < options.WorkerCount; i++ {
		go pool.worker()
	}
	return pool, nil
}

func (p *Pool) Enqueue(job domain.MediaJob) {
	select {
	case p.jobs <- job:
	default:
		slog.Warn("media queue full, dropping job", "item_id", job.ItemID)
	}
}

func (p *Pool) CancelJob(itemID int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if cancel, exists := p.activeJobs[itemID]; exists {
		cancel()
	}
}

func (p *Pool) Shutdown(ctx context.Context) {
	close(p.jobs)

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("media workers shut down cleanly")
	case <-ctx.Done():
		slog.Warn("media workers shutdown timed out")
	}
}

func (p *Pool) worker() {
	defer p.wg.Done()
	for job := range p.jobs {
		if err := p.process(job); err != nil {
			slog.Error("media extraction failed", "item_id", job.ItemID, "err", err)
		}
	}
}

func (p *Pool) process(job domain.MediaJob) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), p.options.ProcessTimeout)
	defer cancel()

	p.mu.Lock()
	p.activeJobs[job.ItemID] = cancel
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.activeJobs, job.ItemID)
		p.mu.Unlock()
	}()

	var generatedFiles []string
	var generatedDirs []string

	defer func() {
		if err != nil {
			for _, path := range generatedFiles {
				if removeErr := p.storage.Remove(path); removeErr != nil {
					slog.Warn("cleanup: failed to remove file", "path", path, "err", removeErr)
				}
			}
			for _, dir := range generatedDirs {
				if removeErr := p.storage.RemoveTree(dir); removeErr != nil {
					slog.Warn("cleanup: failed to remove directory tree", "dir", dir, "err", removeErr)
				}
			}
		}
	}()

	meta := domain.Metadata{MIME: job.MIMEType}

	switch {
	case strings.HasPrefix(job.MIMEType, "image/"):
		imageMeta, err := extractImageMeta(job.FilePath)
		if err != nil {
			return err
		}
		meta = imageMeta

		thumbRelPath, err := generateImageThumbnail(ctx, job.FilePath)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("image thumbnail generation skipped", "path", job.FilePath, "err", err)
			break
		}
		meta.Thumb = thumbRelPath
		generatedFiles = append(generatedFiles, thumbRelPath)

	case strings.HasPrefix(job.MIMEType, "video/"):
		videoInfo, err := extractVideoInfo(ctx, job.FilePath, job.MIMEType)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("video metadata extraction skipped", "path", job.FilePath, "err", err)
			break
		}
		meta = videoInfo.Metadata

		thumbRelPath, err := generateThumbnail(ctx, job.FilePath)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("thumbnail generation skipped", "path", job.FilePath, "err", err)
		} else {
			meta.Thumb = thumbRelPath
			generatedFiles = append(generatedFiles, thumbRelPath)
		}

		playback, err := generateBrowserPlayback(ctx, job.FilePath, videoInfo)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			slog.Warn("video playback copy generation skipped", "path", job.FilePath, "err", err)
			break
		}
		meta.Playback = playback.RelPath
		meta.PlaybackMIME = playback.MIME
		generatedFiles = append(generatedFiles, playback.RelPath)

		if shouldGenerateHLS(job.Size, videoInfo.Duration, p.options) {
			hls, err := generateHLS(ctx, job.FilePath, playback.AbsPath)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				slog.Warn("hls generation skipped", "path", job.FilePath, "err", err)
				break
			}
			meta.HLS = hls.RelPath
			cleanPath := filepath.ToSlash(filepath.Clean(hls.RelPath))
			if cleanPath != "." && strings.HasPrefix(cleanPath, "hls/") {
				dir := filepath.Dir(cleanPath)
				if dir != "." && dir != "hls" {
					generatedDirs = append(generatedDirs, dir)
				}
			}
		}
	}

	meta.Processing = false
	if err := p.repo.UpdateMetadata(ctx, job.ItemID, meta); err != nil {
		return err
	}
	p.broker.Broadcast(domain.Event{Type: "item:updated", ID: job.ItemID})
	return nil
}
