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
	jobs   chan domain.MediaJob
	repo   domain.ItemRepository
	broker domain.EventBroker
	wg     sync.WaitGroup
}

func NewPool(repo domain.ItemRepository, broker domain.EventBroker, workerCount int) (*Pool, error) {
	if workerCount <= 0 {
		return nil, fmt.Errorf("media worker count must be positive")
	}

	pool := &Pool{
		jobs:   make(chan domain.MediaJob, 16),
		repo:   repo,
		broker: broker,
	}
	pool.wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	meta := domain.Metadata{MIME: job.MIMEType}

	switch {
	case strings.HasPrefix(job.MIMEType, "image/"):
		imageMeta, err := extractImageMeta(job.FilePath)
		if err != nil {
			return err
		}
		meta = imageMeta

	case strings.HasPrefix(job.MIMEType, "video/"):
		videoMeta, err := extractVideoMeta(ctx, job.FilePath, job.MIMEType)
		if err != nil {
			slog.Warn("video metadata extraction skipped", "path", job.FilePath, "err", err)
			break
		}
		meta = videoMeta

		thumbRelPath, err := generateThumbnail(ctx, job.FilePath)
		if err != nil {
			slog.Warn("thumbnail generation skipped", "path", job.FilePath, "err", err)
			break
		}
		meta.Thumb = thumbRelPath
	}

	if err := p.repo.UpdateMetadata(ctx, job.ItemID, meta); err != nil {
		return err
	}
	p.broker.Broadcast(domain.Event{Type: "item:updated", ID: job.ItemID})
	return nil
}
