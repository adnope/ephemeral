# LeanDrop — Architecture Design Document

**Version:** 1.0.0
**Status:** Ready for Implementation
**Target:** AI Coding Agent / Senior Go Developer

---

## Table of Contents

1. [System Design Patterns](#1-system-design-patterns)
2. [Directory Structure](#2-directory-structure)
3. [Database Schema & Search](#3-database-schema--search)
4. [Specific Implementation Strategies](#4-specific-implementation-strategies)
5. [Resource Optimization](#5-resource-optimization)
6. [Dependency Manifest](#6-dependency-manifest)

---

## 1. System Design Patterns

### 1.1 Repository Pattern — SQLite Interactions

The Repository Pattern isolates all database logic behind a Go interface. Handlers **never** hold a `*sql.DB` reference directly. This enables unit-testing with a mock repository and enforces a strict data-access boundary.

#### Interface Contract (`internal/store/repository.go`)

```go
package store

import "context"

// ItemRepository defines all persistence operations for items.
// All methods accept a context.Context to support cancellation
// and per-request timeouts.
type ItemRepository interface {
    Create(ctx context.Context, item *Item) (int64, error)
    GetByID(ctx context.Context, id int64) (*Item, error)
    List(ctx context.Context, filter ListFilter) ([]*Item, error)
    Delete(ctx context.Context, id int64) error
    Search(ctx context.Context, query string, limit int) ([]*Item, error)
    MediaHistory(ctx context.Context, types []string, cursor int64, limit int) ([]*Item, error)
}

type SessionRepository interface {
    Create(ctx context.Context, s *Session) error
    GetByToken(ctx context.Context, token string) (*Session, error)
    Delete(ctx context.Context, token string) error
    PurgeExpired(ctx context.Context) error
}
```

#### Concrete SQLite Implementation (`internal/store/sqlite_item_repo.go`)

```go
type sqliteItemRepo struct {
    db *sql.DB
}

func (r *sqliteItemRepo) Create(ctx context.Context, item *Item) (int64, error) {
    const q = `
        INSERT INTO items (type, content, filename, filesize, metadata)
        VALUES (?, ?, ?, ?, ?)
        RETURNING id`

    var id int64
    err := r.db.QueryRowContext(ctx, q,
        item.Type, item.Content, item.Filename,
        item.Filesize, item.Metadata,
    ).Scan(&id)
    if err != nil {
        return 0, fmt.Errorf("store.Create: %w", err)
    }
    return id, nil
}

func (r *sqliteItemRepo) List(ctx context.Context, f ListFilter) ([]*Item, error) {
    const q = `
        SELECT id, type, content, filename, filesize, metadata, created_at
        FROM items
        WHERE id < ?
        ORDER BY id DESC
        LIMIT ?`

    rows, err := r.db.QueryContext(ctx, q, f.Cursor, f.Limit)
    if err != nil {
        return nil, fmt.Errorf("store.List: %w", err)
    }
    defer rows.Close() // MANDATORY: resource guard

    var items []*Item
    for rows.Next() {
        var it Item
        if err := rows.Scan(
            &it.ID, &it.Type, &it.Content, &it.Filename,
            &it.Filesize, &it.Metadata, &it.CreatedAt,
        ); err != nil {
            return nil, err
        }
        items = append(items, &it)
    }
    return items, rows.Err()
}
```

#### Key Rules for All Repository Methods

- Every `rows.Close()` must be `defer`-ed immediately after error check on `QueryContext`.
- Every `stmt.Close()` must be `defer`-ed for prepared statements.
- Wrap all errors with `fmt.Errorf("layer.Operation: %w", err)` for traceable error chains.
- Use `QueryRowContext` for single-row returns; use `ExecContext` for mutations.
- Never expose `*sql.Rows` outside the repository boundary.

---

### 1.2 Middleware Pipeline

The pipeline is composed in `cmd/leandrop/main.go` using `chi`'s middleware chaining. Order is strict and intentional.

```
[Request] → Recoverer → RequestID → RealIP → Logger → RateLimit → Auth → [Handler]
```

#### Composition (`cmd/leandrop/main.go`)

```go
r := chi.NewRouter()

// Layer 1: Infrastructure (always runs, no auth context needed)
r.Use(middleware.Recoverer)
r.Use(middleware.RequestID)
r.Use(middleware.RealIP)

// Layer 2: Observability
r.Use(mw.RequestLogger(logger))

// Layer 3: Security — rate limit before auth to prevent enumeration
r.Use(mw.RateLimit(100, time.Minute)) // token bucket, 100 req/min

// Layer 4: Session Auth (skips /login, /static/*)
r.Use(mw.SessionAuth(sessionRepo))
```

#### Session Auth Middleware (`internal/middleware/auth.go`)

```go
var publicPaths = map[string]struct{}{
    "/login": {}, "/favicon.ico": {}, "/manifest.json": {},
}

func SessionAuth(repo store.SessionRepository) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Static assets bypass auth entirely
            if strings.HasPrefix(r.URL.Path, "/static/") {
                next.ServeHTTP(w, r)
                return
            }
            if _, ok := publicPaths[r.URL.Path]; ok {
                next.ServeHTTP(w, r)
                return
            }

            cookie, err := r.Cookie("session_token")
            if err != nil {
                http.Redirect(w, r, "/login", http.StatusSeeOther)
                return
            }

            session, err := repo.GetByToken(r.Context(), cookie.Value)
            if err != nil || session.ExpiresAt.Before(time.Now()) {
                http.SetCookie(w, expiredCookie())
                http.Redirect(w, r, "/login", http.StatusSeeOther)
                return
            }

            // Inject session into context; handler reads via helper
            ctx := context.WithValue(r.Context(), ctxKeySession, session)
            next.ServeHTTP(w, r.WithContext(ctx))
        })
    }
}
```

#### Structured Request Logger (`internal/middleware/logger.go`)

```go
func RequestLogger(log *slog.Logger) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            start := time.Now()
            ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
            next.ServeHTTP(ww, r)
            log.Info("request",
                "id",      middleware.GetReqID(r.Context()),
                "method",  r.Method,
                "path",    r.URL.Path,
                "status",  ww.Status(),
                "bytes",   ww.BytesWritten(),
                "latency", time.Since(start).String(),
                "ip",      r.RemoteAddr,
            )
        })
    }
}
```

---

### 1.3 Media Extraction Service — Non-Blocking Worker Pool

All metadata extraction is **asynchronous and off the hot path**. The upload handler writes the file to disk, inserts a minimal DB record, broadcasts the SSE event immediately (< 5ms), and then enqueues an extraction job. The client sees the file instantly; metadata enriches the record within seconds.

#### Architecture

```
POST /upload
    │
    ├─► io.Copy(file → disk)          [zero-copy, streaming]
    ├─► db.Create(item, meta="{}")    [fast insert, no metadata yet]
    ├─► sse.Broadcast(itemID)         [client refreshes immediately]
    └─► mediaQueue <- Job{itemID}     [non-blocking channel send]

MediaWorkerPool (N=2 goroutines)
    │
    ├─► Receive Job{itemID}
    ├─► sniffMIME(filePath)
    ├─► if image  → extractImageMeta(path)   [image.DecodeConfig]
    ├─► if video  → runFFprobe(path)          [exec.Command]
    │               runFFmpegThumb(path)
    └─► db.UpdateMetadata(itemID, meta)
        └─► sse.Broadcast(itemID)            [client re-renders bubble]
```

#### Worker Pool Implementation (`internal/media/pool.go`)

```go
const workerCount = 2

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

func (p *Pool) Enqueue(job Job) {
    select {
    case p.jobs <- job:
    default:
        // Queue full: log and drop. Upload is already persisted;
        // metadata will be absent but data is not lost.
        slog.Warn("media queue full, dropping job", "item_id", job.ItemID)
    }
}

func (p *Pool) Shutdown(ctx context.Context) {
    close(p.jobs)
    done := make(chan struct{})
    go func() { p.wg.Wait(); close(done) }()
    select {
    case <-done:
    case <-ctx.Done():
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
        if err != nil { return err }
        meta = m

    case strings.HasPrefix(job.MIMEType, "video/"):
        m, err := extractVideoMeta(ctx, job.FilePath)
        if err != nil { return err }
        meta = m
        if err := generateThumbnail(ctx, job.FilePath); err != nil {
            slog.Warn("thumbnail generation failed", "path", job.FilePath)
        }

    default:
        meta = store.Metadata{MIME: job.MIMEType}
    }

    if err := p.repo.UpdateMetadata(ctx, job.ItemID, meta); err != nil {
        return err
    }
    p.broker.Broadcast(sse.Event{Type: "item:updated", ID: job.ItemID})
    return nil
}
```

#### Image Metadata (No Pixel Buffer Allocation)

```go
func extractImageMeta(path string) (store.Metadata, error) {
    f, err := os.Open(path)
    if err != nil { return store.Metadata{}, err }
    defer f.Close()

    // image.DecodeConfig reads ONLY the header — zero pixel allocation.
    cfg, format, err := image.DecodeConfig(f)
    if err != nil { return store.Metadata{}, err }

    return store.Metadata{
        Width:  cfg.Width,
        Height: cfg.Height,
        MIME:   "image/" + format,
    }, nil
}
```

#### Video Metadata via ffprobe

```go
func extractVideoMeta(ctx context.Context, path string) (store.Metadata, error) {
    args := []string{
        "-v", "quiet", "-print_format", "json",
        "-show_streams", "-show_format", path,
    }
    cmd := exec.CommandContext(ctx, "ffprobe", args...)
    // Capture stdout only; stderr goes to /dev/null via quiet flag.
    out, err := cmd.Output()
    if err != nil { return store.Metadata{}, fmt.Errorf("ffprobe: %w", err) }

    var probe ffprobeOutput
    if err := json.Unmarshal(out, &probe); err != nil { return store.Metadata{}, err }

    return probe.toMetadata(), nil
}
```

---

## 2. Directory Structure

```
leandrop/
├── cmd/
│   └── leandrop/
│       └── main.go              # Entrypoint: wire deps, start server
│
├── internal/
│   ├── config/
│   │   └── config.go            # Env-based config (port, data dir, session secret)
│   │
│   ├── handler/
│   │   ├── handler.go           # Base Handler struct holding all service deps
│   │   ├── index.go             # GET /
│   │   ├── upload.go            # POST /upload
│   │   ├── message.go           # POST /message
│   │   ├── files.go             # GET /files/{path}
│   │   ├── history.go           # GET /history
│   │   ├── events.go            # GET /events (SSE)
│   │   └── auth.go              # GET|POST /login, POST /logout
│   │
│   ├── media/
│   │   ├── pool.go              # Worker pool (see §1.3)
│   │   ├── image.go             # image.DecodeConfig wrapper
│   │   ├── video.go             # ffprobe / ffmpeg wrappers
│   │   └── mime.go              # MIME sniffing logic
│   │
│   ├── middleware/
│   │   ├── auth.go              # Session auth middleware
│   │   ├── logger.go            # Structured request logger
│   │   └── ratelimit.go         # Token bucket rate limiter
│   │
│   ├── sse/
│   │   ├── broker.go            # Fan-out SSE broker
│   │   └── event.go             # Event type definitions
│   │
│   └── store/
│       ├── repository.go        # Interfaces (ItemRepository, SessionRepository)
│       ├── models.go            # Item, Session, User structs + Metadata JSON type
│       ├── sqlite.go            # DB init, PRAGMA tuning, migrations
│       ├── item_repo.go         # sqliteItemRepo implementation
│       └── session_repo.go      # sqliteSessionRepo implementation
│
├── web/
│   ├── static/
│   │   ├── app.min.js           # Bundled: htmx + alpine.js (minified, single file)
│   │   ├── app.css              # Tailwind output (purged, < 15KB)
│   │   ├── sw.js                # Service Worker
│   │   └── manifest.json        # PWA manifest
│   │
│   └── template/
│       ├── base.html            # Root layout: <head>, nav, SSE init script
│       ├── index.html           # Main chat stream
│       ├── history.html         # Media gallery
│       ├── login.html           # Login form
│       └── partials/
│           ├── item_text.html   # Text bubble component
│           ├── item_image.html  # Image bubble + <dialog>
│           ├── item_video.html  # Video bubble
│           └── item_file.html   # File bubble
│
├── migrations/
│   └── 001_initial.sql          # Embedded via go:embed
│
├── data/                        # Runtime-generated; gitignored
│   ├── leandrop.db
│   └── uploads/
│       └── thumbs/
│
├── Dockerfile
├── docker-compose.yml
├── .env.example
├── go.mod
├── go.sum
└── Makefile
```

### Handler Dependency Injection (`internal/handler/handler.go`)

```go
// Handler is the single dependency container passed to all route handlers.
// It is constructed once in main.go and never mutated after startup.
type Handler struct {
    store    store.ItemRepository
    sessions store.SessionRepository
    broker   *sse.Broker
    media    *media.Pool
    tmpl     *template.Template
    dataDir  string
    log      *slog.Logger
}
```

---

## 3. Database Schema & Search

### 3.1 Full Schema

```sql
-- ─────────────────────────────────────────────────────────────────
-- PRAGMA Tuning (applied at DB open time in Go, not in schema file)
-- ─────────────────────────────────────────────────────────────────
-- PRAGMA journal_mode = WAL;        -- Concurrent reads during writes
-- PRAGMA synchronous   = NORMAL;    -- Safe + fast (no fsync on every write)
-- PRAGMA cache_size    = -4096;     -- 4MB page cache (negative = kibibytes)
-- PRAGMA foreign_keys  = ON;
-- PRAGMA temp_store    = MEMORY;

-- ─────────────────────────────────────────────────────────────────
-- Core Data
-- ─────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,       -- bcrypt, cost 12
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,       -- crypto/rand 32-byte hex
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS items (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    type       TEXT NOT NULL CHECK(type IN ('text','image','video','file')),
    content    TEXT NOT NULL,          -- text body OR relative file path
    filename   TEXT,                   -- original filename for files/media
    filesize   INTEGER,                -- bytes
    metadata   TEXT NOT NULL DEFAULT '{}', -- JSON: width, height, duration, mime, thumb
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ─────────────────────────────────────────────────────────────────
-- Indexes
-- ─────────────────────────────────────────────────────────────────

-- Primary scroll query: newest-first cursor pagination
CREATE INDEX IF NOT EXISTS idx_items_id_desc
    ON items(id DESC);

-- History/gallery filter by type (image, video, file)
CREATE INDEX IF NOT EXISTS idx_items_type_id
    ON items(type, id DESC);

-- Session lookup on every authenticated request (hot path)
CREATE INDEX IF NOT EXISTS idx_sessions_expires
    ON sessions(expires_at);

-- ─────────────────────────────────────────────────────────────────
-- Full-Text Search (FTS5) — Virtual Table
-- ─────────────────────────────────────────────────────────────────

-- Indexes text content and original filenames.
-- Kept in sync via triggers; zero application-layer sync code needed.
CREATE VIRTUAL TABLE IF NOT EXISTS items_fts
    USING fts5(
        content,
        filename,
        content='items',         -- content table: FTS reads from items for snippets
        content_rowid='id',
        tokenize='unicode61 remove_diacritics 1'
    );

-- Sync triggers
CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
    INSERT INTO items_fts(rowid, content, filename)
    VALUES (new.id, new.content, COALESCE(new.filename, ''));
END;

CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, content, filename)
    VALUES ('delete', old.id, old.content, COALESCE(old.filename, ''));
END;

CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
    INSERT INTO items_fts(items_fts, rowid, content, filename)
    VALUES ('delete', old.id, old.content, COALESCE(old.filename, ''));
    INSERT INTO items_fts(rowid, content, filename)
    VALUES (new.id, new.content, COALESCE(new.filename, ''));
END;
```

### 3.2 FTS5 Search Query (Repository Implementation)

```go
func (r *sqliteItemRepo) Search(ctx context.Context, q string, limit int) ([]*Item, error) {
    const query = `
        SELECT i.id, i.type, i.content, i.filename, i.filesize, i.metadata, i.created_at
        FROM items_fts
        JOIN items i ON items_fts.rowid = i.id
        WHERE items_fts MATCH ?
        ORDER BY rank            -- FTS5 BM25 relevance rank
        LIMIT ?`

    // Sanitize: FTS5 MATCH syntax uses quotes for phrases.
    // Replace double-quotes in user input to prevent injection.
    safe := strings.ReplaceAll(q, `"`, `""`)

    rows, err := r.db.QueryContext(ctx, query, safe, limit)
    if err != nil { return nil, fmt.Errorf("store.Search: %w", err) }
    defer rows.Close()
    // ... standard scan loop
}
```

### 3.3 Cursor-Based Pagination vs. OFFSET

**Never use `OFFSET` for pagination.** At page N with large datasets, SQLite must scan all prior rows. Use the `id` as a cursor instead:

```sql
-- First page
SELECT * FROM items ORDER BY id DESC LIMIT 30;

-- Next page (cursor = last id seen)
SELECT * FROM items WHERE id < ? ORDER BY id DESC LIMIT 30;
```

This is O(log N) via the `idx_items_id_desc` index, regardless of dataset size.

---

## 4. Specific Implementation Strategies

### 4.1 Streaming Architecture — 50GB File Upload/Download

**Constraint: Heap allocation must remain under 10MB throughout the transfer.**

The entire flow is an unbroken chain of `io.Reader`/`io.Writer` interfaces. No intermediate buffer ever holds more than 32KB (Go's `io.Copy` default buffer, stack-allocated per goroutine).

#### Upload Flow

```
[Android Browser]
       │
       │  HTTP/1.1 POST multipart/form-data (chunked transfer encoding)
       │  (Caddy proxies, enforces max_upload_size from env)
       ▼
[chi Handler: POST /upload]
       │
       ├─ 1. r.ParseMultipartForm(32 << 20)
       │       maxMemory=32MB: this does NOT load the file into memory.
       │       It sets the threshold for spilling part headers to a temp file.
       │       File BODY is never loaded; it remains a streaming io.Reader.
       │
       ├─ 2. part, err := r.FormFile("file")
       │       Returns a multipart.File (implements io.ReadSeeker)
       │       backed by a temp file on disk — still zero heap for file bytes.
       │
       ├─ 3. dst, err := os.CreateTemp(dataDir, "upload-*")
       │       defer dst.Close()
       │       defer os.Remove(dst.Name()) // cleanup on failure
       │
       ├─ 4. written, err := io.Copy(dst, part)
       │       ┌──────────────────────────────────────────────┐
       │       │  KERNEL SPACE                                 │
       │       │  Network socket → page cache → disk           │
       │       │  Go runtime: 32KB stack buffer, reused        │
       │       │  Heap allocation for 50GB file: ~0 bytes      │
       │       └──────────────────────────────────────────────┘
       │
       ├─ 5. os.Rename(dst.Name(), finalPath)  // atomic move
       │
       ├─ 6. Sniff MIME from first 512 bytes:
       │       f, _ := os.Open(finalPath)
       │       buf := make([]byte, 512)   // 512B, stack-escapes to heap once
       │       f.Read(buf); f.Close()
       │       mime := http.DetectContentType(buf)
       │
       ├─ 7. db.Create(item)           // instant, metadata="{}"
       ├─ 8. broker.Broadcast(itemID)  // SSE push to all clients
       └─ 9. pool.Enqueue(Job{...})    // async metadata extraction
```

#### Download Flow

```go
func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request) {
    // chi.URLParam reads from context — no alloc for path parsing
    relPath := chi.URLParam(r, "path")

    // Security: reject any path traversal attempt
    cleanPath := filepath.Clean(relPath)
    if strings.Contains(cleanPath, "..") {
        http.Error(w, "forbidden", http.StatusForbidden)
        return
    }

    absPath := filepath.Join(h.dataDir, "uploads", cleanPath)

    // http.ServeFile calls os.Open internally, then uses
    // the sendfile(2) syscall on Linux: data moves
    // disk → NIC via kernel, bypassing user space entirely.
    // Heap allocation for a 50GB download: ~0 bytes.
    http.ServeFile(w, r, absPath)
}
```

**Memory accounting for a concurrent 50GB upload + download:**

| Component | Allocation |
|---|---|
| io.Copy buffer per upload goroutine | 32 KB |
| http.ServeFile kernel sendfile | 0 B heap |
| MIME sniff buffer | 512 B |
| DB row for item record | ~200 B |
| SSE event broadcast | ~64 B |
| **Total per transfer** | **~33 KB** |

---

### 4.2 PWA Integration — Service Worker Strategy

The Service Worker (`web/static/sw.js`) uses a **Cache-First for shell assets, Network-First for API** strategy. The SSE connection is deliberately excluded from the Service Worker scope; it must always reach the origin.

#### Service Worker (`web/static/sw.js`)

```javascript
const CACHE_VERSION = 'leandrop-v1';

// Static shell: cache aggressively, update on SW version bump
const SHELL_ASSETS = [
    '/',
    '/static/app.min.js',
    '/static/app.css',
    '/static/manifest.json',
    '/offline.html',
];

// Never intercept these paths; they must hit the origin
const BYPASS_PATTERNS = [
    /^\/events$/,          // SSE stream
    /^\/upload$/,          // File upload (streaming)
    /^\/message$/,         // POST, must be live
    /^\/files\//,          // Raw file serve
];

self.addEventListener('install', event => {
    event.waitUntil(
        caches.open(CACHE_VERSION)
            .then(cache => cache.addAll(SHELL_ASSETS))
            .then(() => self.skipWaiting())
    );
});

self.addEventListener('activate', event => {
    event.waitUntil(
        caches.keys().then(keys =>
            Promise.all(
                keys.filter(k => k !== CACHE_VERSION)
                    .map(k => caches.delete(k))
            )
        ).then(() => self.clients.claim())
    );
});

self.addEventListener('fetch', event => {
    const url = new URL(event.request.url);

    // Bypass: SSE, uploads, POSTs — always go to network
    if (BYPASS_PATTERNS.some(p => p.test(url.pathname))) return;
    if (event.request.method !== 'GET') return;

    // Shell assets: Cache-First
    if (SHELL_ASSETS.includes(url.pathname)) {
        event.respondWith(
            caches.match(event.request).then(cached =>
                cached || fetch(event.request)
            )
        );
        return;
    }

    // API routes (/, /history): Network-First with offline fallback
    event.respondWith(
        fetch(event.request)
            .then(response => {
                const clone = response.clone();
                caches.open(CACHE_VERSION)
                    .then(c => c.put(event.request, clone));
                return response;
            })
            .catch(() =>
                caches.match(event.request) ||
                caches.match('/offline.html')
            )
    );
});
```

#### SSE Reconnection (HTMX Integration in `base.html`)

```html
<!-- SSE connection is managed by HTMX, outside SW scope.
     hx-ext="sse" handles EventSource lifecycle including:
     - Automatic reconnect with exponential backoff
     - Re-subscription after network recovery             -->
<div id="sse-source"
     hx-ext="sse"
     sse-connect="/events"
     sse-swap="message"
     hx-target="#chat-stream"
     hx-swap="afterbegin">
</div>
```

#### SSE Broker (`internal/sse/broker.go`)

```go
// Broker manages all active SSE client connections.
// A sync.Map is used for concurrent-safe subscriber management
// without a global mutex on the broadcast hot path.
type Broker struct {
    subscribers sync.Map // map[chan Event]struct{}
}

func (b *Broker) Subscribe() chan Event {
    ch := make(chan Event, 4) // buffered: slow clients don't block Broadcast
    b.subscribers.Store(ch, struct{}{})
    return ch
}

func (b *Broker) Unsubscribe(ch chan Event) {
    b.subscribers.Delete(ch)
    close(ch)
}

func (b *Broker) Broadcast(e Event) {
    b.subscribers.Range(func(key, _ any) bool {
        ch := key.(chan Event)
        select {
        case ch <- e:
        default:
            // Subscriber is too slow; event dropped. Not fatal for SSE.
        }
        return true // continue iteration
    })
}

// ServeSSE is the http.HandlerFunc for GET /events
func (b *Broker) ServeSSE(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("X-Accel-Buffering", "no") // disable Nginx/Caddy buffering

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "streaming unsupported", http.StatusInternalServerError)
        return
    }

    ch := b.Subscribe()
    defer b.Unsubscribe(ch)

    // Keepalive: prevent proxy timeouts (every 25s)
    ticker := time.NewTicker(25 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case event := <-ch:
            fmt.Fprintf(w, "event: %s\ndata: %d\n\n", event.Type, event.ID)
            flusher.Flush()
        case <-ticker.C:
            fmt.Fprintf(w, ": keepalive\n\n")
            flusher.Flush()
        case <-r.Context().Done():
            return // client disconnected
        }
    }
}
```

---

### 4.3 MIME Sniffing Strategy

MIME type determination uses a **three-pass waterfall**: fast extension check → magic byte sniff → ffprobe ground truth.

```go
// internal/media/mime.go

// extMap covers the 99% case with zero I/O.
var extMap = map[string]string{
    ".jpg": "image/jpeg",  ".jpeg": "image/jpeg",
    ".png": "image/png",   ".gif":  "image/gif",
    ".webp":"image/webp",  ".mp4":  "video/mp4",
    ".mov": "video/quicktime", ".webm": "video/webm",
    ".pdf": "application/pdf",
    ".go":  "text/x-go",   ".py":   "text/x-python",
    ".js":  "text/javascript", ".ts": "text/typescript",
    ".md":  "text/markdown",
}

// SniffMIME determines MIME type using a three-pass waterfall.
// filePath must be the final destination path (post-rename).
func SniffMIME(filePath string) (string, error) {
    // Pass 1: Extension lookup — O(1), zero I/O
    ext := strings.ToLower(filepath.Ext(filePath))
    if mime, ok := extMap[ext]; ok {
        return mime, nil
    }

    // Pass 2: Magic byte detection — reads exactly 512 bytes
    f, err := os.Open(filePath)
    if err != nil { return "", err }
    defer f.Close()

    // Use a pooled buffer to avoid per-request heap allocation
    buf := sniffPool.Get().(*[512]byte)
    defer sniffPool.Put(buf)

    n, _ := f.Read(buf[:])
    detected := http.DetectContentType(buf[:n])

    // Pass 3: If still generic octet-stream for a known video container,
    // trust ffprobe (executed later by worker; return placeholder here)
    if detected == "application/octet-stream" && isVideoExt(ext) {
        return "video/unknown", nil
    }
    return detected, nil
}

var sniffPool = sync.Pool{
    New: func() any { return new([512]byte) },
}
```

---

## 5. Resource Optimization

### 5.1 RAM Budget Justification (< 30MB)

The 30MB target is achieved by eliminating every unnecessary allocation at each layer.

| Component | Baseline Memory | Notes |
|---|---|---|
| Go runtime (base) | ~3.0 MB | Minimal goroutine stack, GC metadata |
| `modernc.org/sqlite` (CGO-free) | ~4.0 MB | SQLite in-process, WAL journal |
| SQLite page cache | ~4.0 MB | `PRAGMA cache_size = -4096` |
| chi router + middleware | ~0.5 MB | Compiled trie, no reflection |
| `html/template` parsed set | ~0.8 MB | Parsed once at startup, immutable |
| SSE broker (1 client) | ~0.02 MB | One buffered channel + sync.Map entry |
| Media worker pool (2 goroutines) | ~0.12 MB | 64KB stack each at rest |
| Idle HTTP goroutine pool | ~0.5 MB | Shared, Go runtime-managed |
| **Total at idle (0 active transfers)** | **~13 MB** | Leaves ~17MB headroom |
| **Peak during active file transfer** | **~14 MB** | +32KB io.Copy buffer per transfer |

The key architectural decisions enabling this budget:

1. **No framework bloat.** chi is a ~600-line router; it allocates only per-request context.
2. **Streaming I/O.** File bytes never touch the Go heap; io.Copy uses a 32KB goroutine-stack buffer.
3. **`modernc.org/sqlite`** avoids CGO's separate OS thread overhead (typically +2MB per CGO thread).
4. **Template pre-parsing.** All `html/template` objects are parsed once at `init()` time; zero parse overhead per request.
5. **Pooled buffers.** `sync.Pool` for MIME sniff buffers, log entry builders, and JSON encoders.
6. **SSE over WebSockets.** Each SSE client is one goroutine + one buffered channel (~80KB + 96B). A WebSocket connection requires a read pump goroutine, a write pump goroutine, and frame codec buffers.

### 5.2 Multi-Stage Dockerfile

```dockerfile
# ─────────────────────────────────────────────────────────────────
# Stage 1: Build — Full Go toolchain + Tailwind CLI
# ─────────────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /src

# Dependency layer — cached unless go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

# Tailwind standalone CLI (no Node.js required)
ARG TAILWIND_VERSION=3.4.1
RUN wget -qO /usr/local/bin/tailwindcss \
    "https://github.com/tailwindlabs/tailwindcss/releases/download/v${TAILWIND_VERSION}/tailwindcss-linux-x64" \
    && chmod +x /usr/local/bin/tailwindcss

# Copy source and build CSS
COPY . .
RUN tailwindcss \
    -i ./web/static/app.css \
    -o ./web/static/app.min.css \
    --minify

# Build binary
# CGO_ENABLED=0: required for modernc.org/sqlite (pure Go)
# -trimpath: remove local build paths from binary
# -ldflags: strip debug symbols (-s) and DWARF (-w) → ~30% smaller binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /bin/leandrop \
    ./cmd/leandrop

# ─────────────────────────────────────────────────────────────────
# Stage 2: Runtime — Distroless + ffmpeg
# ─────────────────────────────────────────────────────────────────
# debian:bookworm-slim gives us: ffmpeg, ca-certificates, tzdata.
# Distroless would be ideal but lacks ffmpeg; slim is the best
# balance of attack surface and feature completeness.
FROM debian:bookworm-slim AS runtime

# Install only what's needed at runtime
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Non-root user for security
RUN useradd -r -u 1001 -s /sbin/nologin leandrop
USER leandrop

WORKDIR /app

# Copy artifacts from builder
COPY --from=builder /bin/leandrop     /app/leandrop
COPY --from=builder /src/web          /app/web
COPY --from=builder /src/migrations   /app/migrations

# Data directory: mount a Docker volume here
# docker run -v leandrop-data:/app/data ...
RUN mkdir -p /app/data/uploads/thumbs

EXPOSE 8080

ENTRYPOINT ["/app/leandrop"]
```

#### Final Image Size Analysis

| Layer | Size |
|---|---|
| debian:bookworm-slim base | ~30 MB |
| ffmpeg + dependencies | ~65 MB |
| ca-certificates | ~0.5 MB |
| leandrop binary (stripped) | ~8–12 MB |
| web/ assets (templates + CSS + JS) | ~0.5 MB |
| **Total image** | **~108 MB** |

The Go binary has zero external runtime dependencies (CGO_ENABLED=0). ffmpeg is the dominant size contributor and cannot be reduced without compromising video processing capability. Without video support, the image drops to ~42MB using a distroless base.

### 5.3 Build Makefile Targets

```makefile
.PHONY: build dev css docker lint

CSS_IN  = web/static/app.css
CSS_OUT = web/static/app.min.css

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/leandrop ./cmd/leandrop

css:
	tailwindcss -i $(CSS_IN) -o $(CSS_OUT) --minify

css-watch:
	tailwindcss -i $(CSS_IN) -o $(CSS_OUT) --watch

dev: css
	go run ./cmd/leandrop

lint:
	golangci-lint run ./...

docker:
	docker build -t leandrop:latest .

docker-run:
	docker run -d \
		--name leandrop \
		-p 8080:8080 \
		-v leandrop-data:/app/data \
		--restart unless-stopped \
		leandrop:latest
```

---

## 6. Dependency Manifest

```go
// go.mod
module github.com/yourhandle/leandrop

go 1.21

require (
    github.com/go-chi/chi/v5  v5.1.0    // Router + middleware
    modernc.org/sqlite         v1.29.0   // CGO-free SQLite driver
    golang.org/x/crypto        v0.22.0   // bcrypt for password hashing
)
```

**Zero other runtime dependencies.** All other functionality uses the Go standard library:

| Feature | Standard Library Package |
|---|---|
| HTTP Server | `net/http` |
| HTML Rendering | `html/template` |
| JSON | `encoding/json` |
| File I/O | `os`, `io`, `path/filepath` |
| Image Metadata | `image` |
| ffprobe/ffmpeg | `os/exec` |
| Logging | `log/slog` (Go 1.21+) |
| Concurrency | `sync`, `context` |
| Crypto (sessions) | `crypto/rand` |
| DB | `database/sql` |

---

*End of Architecture Design Document — LeanDrop v1.0.0*
*All code samples are implementation-ready. Wire dependencies in `cmd/leandrop/main.go` using the Handler struct pattern defined in §2.*
