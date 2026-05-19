# Ephemeral Debugging

Use this for runtime errors, failing tests, performance regressions, upload failures, container restarts, CI failures, and unexplained behavior. Start with evidence, then patch.

## Debug Loop

1. Capture the exact symptom.
2. Reproduce or inspect logs.
3. Locate the failing layer.
4. Patch the smallest root cause.
5. Verify with targeted checks.
6. Repeat if the evidence changes.
7. Stop after the issue is fixed or the remaining blocker is explicit.

Do not diagnose from the surface message alone. Identify the component that actually rejects, crashes, blocks, or returns the wrong data.

## First Evidence

Useful commands:

```bash
git status --short
go test ./...
docker compose ps
docker compose logs ephemeral
docker compose stats
```

For local server behavior:

```bash
make run
curl -i http://localhost:8080/api/auth/state
```

For browser/API mismatch, inspect:

- Request method, path, headers, content type, accept header.
- Status code and response body.
- Whether the route is public or requires `session_token`.
- Whether the endpoint returns HTML partials or JSON for that request.

## Common Failure Paths

Upload failure:

- Check whether rejection came from app, reverse proxy, Cloudflare, browser, or Docker memory limit.
- `HTTP/3 413` with `server: cloudflare` means Cloudflare rejected before the origin app.
- Origin-side `MAX_UPLOAD_SIZE` is enforced by `http.MaxBytesReader`.
- Upload storage streams to temp files; memory spikes usually come from media decode, FFmpeg, indexing, or concurrent uploads.

Media failure:

- Check `ffmpeg` and `ffprobe` availability.
- Image metadata uses `image.DecodeConfig`; keep image format registration imports.
- Thumbnails are generated through FFmpeg and should not require full image decode in Go.
- Missing FFmpeg in tests should skip binary-dependent tests, not fail unrelated CI.

SQLite failure:

- Check migrations, WAL mode, pragmas, connection pool settings, and busy/interrupt behavior.
- Re-check `internal/infrastructure/sqlite/sqlite.go`; do not rely on stale notes.
- For query bugs, inspect repository SQL and FTS token construction.
- For delete/search bugs, inspect FTS triggers and cleanup tables.

Auth/session failure:

- First run creates the initial user during login when no user exists.
- Protected routes require `session_token`.
- Sessions are rolling; near expiry, middleware refreshes DB expiry and cookie max age.
- JSON clients receive JSON unauthenticated errors; browsers redirect to `/login`.

SSE failure:

- Route is `GET /api/events`.
- Rate limiter skips SSE.
- Producers broadcast `item:new`, `item:updated`, and `item:deleted`.
- Check whether the broker is running and whether the client reconnects correctly.

## Patch Rules

- Fix the root cause, not just the error string.
- Preserve existing successful behavior unless the user asks to change it.
- Keep fixes narrow during incident work.
- Add regression tests for reproducible bugs.
- If a fix changes runtime behavior, run `verify.md` docs sync.

## Final Debug Report

Report:

- Actual cause.
- Files changed.
- Why the fix addresses the cause.
- Verification command output summary.
- Any remaining environmental dependency, such as missing FFmpeg, proxy limits, or unavailable Docker.
