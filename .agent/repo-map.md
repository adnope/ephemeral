# Ephemeral Repo Map

Use this first for any non-trivial task in this repository. It is the shared map for `implement`, `debugging`, `verify`, and `security-review`.

## Product Shape

Ephemeral is a single-node, self-hosted Go web app for sharing text messages and uploaded files across devices.

- Web UI: server-rendered templates with HTMX and Alpine.js.
- Mobile/API surface: JSON responses when the request asks for JSON.
- Persistence: SQLite with embedded migrations and FTS5.
- Real-time updates: Server-Sent Events.
- Media pipeline: uploaded images/videos are stored first, then metadata and thumbnails are generated asynchronously through FFmpeg/FFprobe-backed workers.
- Deployment: Docker image runs as a non-root user and stores data in `/app/data`.

## Key Files

- `cmd/ephemeral/main.go`: app composition, dependency wiring, router, middleware, HTTP server, graceful shutdown.
- `internal/config/config.go`: environment parsing, defaults, data directory creation.
- `internal/domain/*.go`: core entities and interfaces.
- `internal/usecase/*.go`: business flows for auth, items, history, file previews, upload side effects.
- `internal/delivery/http/*.go`: HTTP handlers, HTML-vs-JSON behavior, response mapping.
- `internal/middleware/*.go`: session authentication, rate limiting, request logging.
- `internal/infrastructure/sqlite/*.go`: SQLite connection, repositories, metadata JSON mapping.
- `internal/migrations/*.sql`: embedded schema and FTS migrations.
- `internal/infrastructure/filesystem/uploads.go`: upload storage, safe path resolution, bounded reads.
- `internal/infrastructure/media/*.go`: MIME classification, image metadata, FFmpeg thumbnails, FFprobe video metadata, media worker pool.
- `internal/infrastructure/search/indexer.go`: text/code body indexing and history search.
- `internal/infrastructure/sse/broker.go`: SSE fanout.
- `web/template/**/*.html`: server-rendered pages and partials.
- `web/static/*`: browser assets, CSS, service worker, minified app JS.
- `docs/API.md`: API contract for browser/mobile clients.
- `README.md`, `Dockerfile`, `docker-compose.yml`, `Makefile`: developer and deployment workflow.

## Runtime Wiring

`main.go` loads config, embedded migrations, SQLite repositories, SSE broker, upload storage, media pool, search indexer, templates, usecases, handlers, middleware, static files, and routes.

Public routes:

- `GET /login`
- `GET /api/auth/state`
- `POST /api/login`
- static assets under `/static/`

Protected routes include:

- Views: `GET /`, `GET /history`, `GET /search`
- JSON/API: `GET /api/config`, `GET /api/events`, `GET /api/items`, `GET /api/history`
- Mutations: `POST /api/message`, `POST /api/upload`, `DELETE /api/items/{id}`, `POST /api/logout`
- Files: `GET /api/files/*`, `GET /api/file-preview/{id}`

## Data Flow

Message creation:

1. HTTP handler validates form or JSON body.
2. `ItemUseCase.CreateMessage` trims text and rejects empty messages.
3. SQLite creates an `items` row with type `text`.
4. SSE broadcasts `item:new`.
5. Handler returns HTML partial or JSON item shape.

File upload:

1. `Upload` wraps the body with `http.MaxBytesReader` using `MAX_UPLOAD_SIZE`.
2. Multipart field must be named `file`.
3. `UploadStorage.Save` streams to a temp file with a 256 KiB buffer, syncs, then renames into `DATA_DIR/uploads`.
4. MIME is detected and mapped to `image`, `video`, or `file`.
5. SQLite creates the `items` row.
6. Text/code-like files are indexed asynchronously up to `BODY_INDEX_MAX`.
7. SSE broadcasts `item:new`.
8. Media jobs are queued for async metadata and thumbnail extraction.
9. Media worker updates metadata and broadcasts `item:updated`.

File serving and preview:

- `ServeFile` URL-decodes the route wildcard and resolves it through `ItemUseCase.ResolveUploadPath`.
- `UploadStorage.Path` rejects empty, absolute, and `..` paths before joining with `uploads`.
- `PreviewFile` only supports generic file items that are text/code-like and below `TEXT_PREVIEW_MAX`.

## SQLite Notes

- Driver: `modernc.org/sqlite`.
- Connection pool: `SetMaxOpenConns(4)` and `SetMaxIdleConns(0)`.
- Pragmas include `foreign_keys(ON)`, `temp_store(MEMORY)`, `busy_timeout(5000)`, and `cache_size(-4096)`.
- `journal_mode = WAL` is applied after open.
- Migrations are embedded and sorted lexicographically before execution.
- FTS tables:
  - `items_fts` indexes item content and filename.
  - `item_bodies_fts` indexes bounded text/code upload bodies.

Do not assume old architecture notes are current. Re-check `internal/infrastructure/sqlite/sqlite.go` before making performance claims.

## Operational Defaults

Defaults are loaded in `internal/config/config.go`:

- `PORT=8080`
- `DATA_DIR=./data`
- `SESSION_TTL=30d`
- `CHAT_PAGE_SIZE=100`
- `HISTORY_PAGE_SIZE=100`
- `SEARCH_RESULT_LIMIT=30`
- `MAX_UPLOAD_SIZE=2GiB`
- `TEXT_PREVIEW_MAX=10MiB`
- `BODY_INDEX_MAX=20MiB`
- `MEDIA_WORKER_COUNT=1`
- `UPLOAD_CONCURRENCY=1`

Docker Compose also sets:

- `mem_limit=768m`
- `GOMEMLIMIT=128MiB`
- `GOGC=50`

## Design Invariants

- Keep browser HTML behavior and JSON API behavior compatible when changing shared endpoints.
- Keep `docs/API.md` synchronized with endpoint, response, error, auth, pagination, SSE, and runtime-limit changes.
- Keep uploads streaming. Avoid reading large files into memory on request paths.
- Keep thumbnail generation out of long-lived Go heap where practical. Prefer bounded metadata reads plus FFmpeg/FFprobe child processes for media-heavy work.
- Keep path safety centralized through storage/path helpers.
- Keep SQLite schema changes migration-based and idempotent.
- Keep SSE event names stable unless docs and clients are changed together.
- Do not increase concurrency defaults without considering SQLite, FFmpeg memory, Docker limits, and disk I/O.
