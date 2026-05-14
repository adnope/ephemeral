package media

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/adnope/leandrop/internal/sse"
	"github.com/adnope/leandrop/internal/store"
)

const workerCount = 2

// Job represents a media extraction task.
type Job struct {
	ItemID   int64
	FilePath string
	MIMEType string
}

// Pool manages async media extraction workers.
type Pool struct {
	jobs   chan Job
	repo   store.ItemRepository
	broker *sse.Broker
	wg     sync.WaitGroup
}

// NewPool creates and starts the media worker pool.
func NewPool(repo store.ItemRepository, broker *sse.Broker) *Pool {
	p := &Pool{
		jobs:   make(chan Job, 64), // buffered: upload handler never blocks
		repo:   repo,
		broker: broker,
	}
	p.wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go p.worker()
	}
	return p
}

// Enqueue adds a media extraction job. Non-blocking; drops job if queue is full.
func (p *Pool) Enqueue(job Job) {
	select {
	case p.jobs <- job:
	default:
		// Queue full: log and drop. Upload is already persisted;
		// metadata will be absent but data is not lost.
		slog.Warn("media queue full, dropping job", "item_id", job.ItemID)
	}
}

// Shutdown gracefully drains the worker pool with a deadline.
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
			slog.Error("media extraction failed",
				"item_id", job.ItemID, "err", err)
		}
	}
}

func (p *Pool) process(job Job) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var meta store.Metadata

	switch {
	case strings.HasPrefix(job.MIMEType, "image/"):
		m, err := extractImageMeta(job.FilePath)
		if err != nil {
			return err
		}
		meta = m

	case strings.HasPrefix(job.MIMEType, "video/"):
		m, err := extractVideoMeta(ctx, job.FilePath)
		if err != nil {
			return err
		}
		meta = m
		if err := generateThumbnail(ctx, job.FilePath); err != nil {
			slog.Warn("thumbnail generation failed", "path", job.FilePath, "err", err)
		}
		// Set thumb path relative to uploads dir
		meta.Thumb = strings.TrimSuffix(job.FilePath, "."+strings.Split(job.FilePath, ".")[len(strings.Split(job.FilePath, "."))-1]) + "_thumb.jpg"

	default:
		meta = store.Metadata{MIME: job.MIMEType}
	}

	if err := p.repo.UpdateMetadata(ctx, job.ItemID, meta); err != nil {
		return err
	}
	p.broker.Broadcast(sse.Event{Type: "item:updated", ID: job.ItemID})
	return nil
}
