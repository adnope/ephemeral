package media

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/adnope/ephemeral/internal/domain"
)

type Pool struct {
	jobs    chan domain.MediaJob
	repo    domain.ItemRepository
	broker  domain.EventBroker
	options PoolOptions
	wg      sync.WaitGroup
}

type PoolOptions struct {
	WorkerCount    int
	ProcessTimeout time.Duration
	HLSMinBytes    int64
	HLSMinDuration time.Duration
}

func NewPool(repo domain.ItemRepository, broker domain.EventBroker, options PoolOptions) (*Pool, error) {
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
		jobs:    make(chan domain.MediaJob, 16),
		repo:    repo,
		broker:  broker,
		options: options,
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

func (p *Pool) process(job domain.MediaJob) error {
	ctx, cancel := context.WithTimeout(context.Background(), p.options.ProcessTimeout)
	defer cancel()

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
			slog.Warn("image thumbnail generation skipped", "path", job.FilePath, "err", err)
			break
		}
		meta.Thumb = thumbRelPath

	case strings.HasPrefix(job.MIMEType, "video/"):
		videoInfo, err := extractVideoInfo(ctx, job.FilePath, job.MIMEType)
		if err != nil {
			slog.Warn("video metadata extraction skipped", "path", job.FilePath, "err", err)
			break
		}
		meta = videoInfo.Metadata

		thumbRelPath, err := generateThumbnail(ctx, job.FilePath)
		if err != nil {
			slog.Warn("thumbnail generation skipped", "path", job.FilePath, "err", err)
		} else {
			meta.Thumb = thumbRelPath
		}

		playback, err := generateBrowserPlayback(ctx, job.FilePath, videoInfo)
		if err != nil {
			slog.Warn("video playback copy generation skipped", "path", job.FilePath, "err", err)
			break
		}
		meta.Playback = playback.RelPath
		meta.PlaybackMIME = playback.MIME

		if shouldGenerateHLS(job.Size, videoInfo.Duration, p.options) {
			hls, err := generateHLS(ctx, job.FilePath, playback.AbsPath)
			if err != nil {
				slog.Warn("hls generation skipped", "path", job.FilePath, "err", err)
				break
			}
			meta.HLS = hls.RelPath
		}
	}

	meta.Processing = false
	if err := p.repo.UpdateMetadata(ctx, job.ItemID, meta); err != nil {
		return err
	}
	p.broker.Broadcast(domain.Event{Type: "item:updated", ID: job.ItemID})
	return nil
}
