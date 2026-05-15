package media

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/adnope/ephemeral/internal/sse"
	"github.com/adnope/ephemeral/internal/store"
)

const workerCount = 1

type Job struct {
	ItemID   int64
	FilePath string
	MIMEType string
}

type Pool struct {
	jobs   chan Job
	repo   store.ItemRepository
	broker *sse.Broker
	wg     sync.WaitGroup
}

func NewPool(repo store.ItemRepository, broker *sse.Broker) *Pool {
	p := &Pool{
		jobs:   make(chan Job, 16),
		repo:   repo,
		broker: broker,
	}
	p.wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go p.worker()
	}
	return p
}

func (p *Pool) Enqueue(job Job) {
	select {
	case p.jobs <- job:
	default:
		slog.Warn("media queue full, dropping job", "item_id", job.ItemID)
	}
}

func (p *Pool) Shutdown(ctx context.Context) {
	close(p.jobs)
	done := make(chan struct{})
	go func() { p.wg.Wait(); close(done) }()
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

func (p *Pool) process(job Job) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	meta := store.Metadata{MIME: job.MIMEType}

	switch {
	case strings.HasPrefix(job.MIMEType, "image/"):
		m, err := extractImageMeta(job.FilePath)
		if err != nil {
			return err
		}
		meta = m

	case strings.HasPrefix(job.MIMEType, "video/"):
		m, err := extractVideoMeta(ctx, job.FilePath, job.MIMEType)
		if err != nil {
			slog.Warn("video metadata extraction skipped", "path", job.FilePath, "err", err)
			break
		}
		meta = m

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
	p.broker.Broadcast(sse.Event{Type: "item:updated", ID: job.ItemID})
	return nil
}
