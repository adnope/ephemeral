# Ephemeral Verify

Use this as the final gate for implementation, debugging, refactors, and security fixes. It includes docs sync.

## Verification Policy

Run the narrowest reliable checks first, then broaden when the change touches shared behavior.

Minimum for Go code changes:

```bash
go test ./...
go vet ./cmd/... ./internal/... ./web
```

Preferred full local gate:

```bash
make format
make lint
go test ./...
```

Use targeted tests during iteration:

```bash
go test ./internal/infrastructure/media
go test ./internal/infrastructure/sqlite
go test ./internal/delivery/http
go test ./internal/middleware
go test ./cmd/ephemeral
```

Use web checks when touching `web/`:

```bash
cd web && npm run format
cd web && npm run lint
```

Use Docker checks when touching `Dockerfile`, `docker-compose.yml`, runtime env, FFmpeg packaging, non-root paths, or deployment docs:

```bash
docker compose build
docker compose up -d
docker compose logs ephemeral
docker compose down
```

Do not claim Docker verification unless those commands actually ran.

## Docs Sync Checklist

Update documentation in the same change when behavior changes:

- `docs/API.md`: routes, auth requirements, request bodies, JSON shapes, error codes, status codes, pagination, SSE events, runtime limits.
- `README.md`: features, requirements, env vars, development commands, Docker commands.
- `docker-compose.yml`: env defaults, memory tuning, ports, volumes.
- `Dockerfile`: runtime dependencies, user permissions, exposed ports.
- `.agent/*.md`: only if agent guidance becomes stale.

If implementation and docs disagree, treat that as a bug. The code is the source of runtime truth, but the final change should make docs match the code.

## Risk-Based Gates

Auth/session changes:

- Test login, logout, initial setup, expired session behavior, rolling session refresh, JSON unauthenticated response, browser redirects.
- Inspect cookie flags: `HttpOnly`, `SameSite`, path, max age.

Upload/media changes:

- Test small file, large file near limit, missing file field, unsafe filename/path cases, image thumbnail, video metadata/thumbnail if FFmpeg is available.
- Confirm FFmpeg-dependent tests skip cleanly when the binary is absent.
- Watch memory behavior when changing decode, buffering, worker count, or upload concurrency.

SQLite/search changes:

- Test migrations from an empty database.
- Test FTS search, history filters, body search, delete cleanup, metadata update.
- Check WAL/pragmas and connection-pool effects before making throughput claims.

HTTP/API changes:

- Verify both browser/HTMX and JSON clients when an endpoint supports both.
- Keep JSON error shape stable:

```json
{"code":"validation_error","message":"Human-readable error"}
```

Frontend/template changes:

- Verify desktop and mobile layout manually when visual behavior changes.
- Keep HTML partial names and HTMX targets stable unless all callers are updated.

## Final Report Format

Report:

- Files changed.
- Behavior changed.
- Verification commands run and result.
- Commands not run, with reason.
- Remaining risks, if any.

Keep the report concise and specific. Do not overstate coverage.
