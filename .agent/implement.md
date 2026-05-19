# Ephemeral Implement

Use this for coding, developing, refactoring, and feature implementation. Start with `repo-map.md`; finish with `verify.md`.

## Workflow

1. Restate the requested behavior as acceptance criteria.
2. Inspect the current implementation before editing.
3. Identify ownership boundaries:
   - HTTP contract: `internal/delivery/http`, `docs/API.md`
   - Business behavior: `internal/usecase`, `internal/domain`
   - Persistence: `internal/infrastructure/sqlite`, `internal/migrations`
   - Upload/file safety: `internal/infrastructure/filesystem`
   - Media processing: `internal/infrastructure/media`
   - Search/history: `internal/infrastructure/search`
   - UI: `web/template`, `web/static`
   - Runtime/deploy: `cmd/ephemeral`, `Dockerfile`, `docker-compose.yml`, `README.md`
4. Make the smallest production-ready change that satisfies the behavior.
5. Add or update tests where behavior, parsing, persistence, security, or API contracts change.
6. Run the verification gate from `verify.md`.
7. Summarize implementation and verification without filler.

## Coding Rules

- Prefer existing architecture and interfaces over new abstractions.
- Keep handlers thin. Put business rules in usecases and persistence details in infrastructure.
- Return wrapped errors with actionable context.
- Keep JSON response structs typed. Avoid weakly typed maps except for small template data.
- Do not introduce placeholder logic, unhandled errors, or critical-path TODOs.
- Keep large uploads streaming. Do not use `io.ReadAll` on upload request bodies.
- Keep file path validation centralized through storage helpers.
- Preserve browser HTML behavior when adding or changing JSON API behavior.
- Preserve JSON compatibility when changing browser-facing endpoints.
- Avoid changing concurrency or memory defaults without checking Docker limits, SQLite pool behavior, FFmpeg child-process memory, and disk I/O.

## API Change Pattern

When adding or changing an endpoint:

1. Add or update handler in `internal/delivery/http`.
2. Put business logic in `internal/usecase`.
3. Extend domain interfaces only when needed.
4. Implement repository/storage/media/search changes in `internal/infrastructure`.
5. Add request validation and stable JSON errors.
6. Update route wiring in `cmd/ephemeral/main.go`.
7. Update `docs/API.md`.
8. Add tests for response shape, status codes, and edge cases.

Shared browser/API endpoints must branch by `Accept: application/json` or JSON `Content-Type` consistently with existing helpers.

## SQLite Change Pattern

When changing persistence:

1. Inspect existing schema and repository queries.
2. Add a new numbered migration in `internal/migrations`.
3. Keep migrations idempotent with `IF NOT EXISTS` where possible.
4. Preserve existing data.
5. Consider FTS triggers, delete cleanup, and body index state.
6. Add repository or migration tests.
7. Check `docs/API.md` if data shape changes.

Do not rewrite migrations that may already be used by deployed data unless the user explicitly asks and accepts the migration risk.

## Upload and Media Pattern

When touching uploads:

- Enforce `MAX_UPLOAD_SIZE` with `http.MaxBytesReader`.
- Accept only multipart field `file` unless changing the public contract.
- Keep save path temp-file -> fsync -> rename.
- Keep stored paths relative to the upload directory.
- Roll back created files when item creation fails.
- Treat FFmpeg/FFprobe as external runtime dependencies.
- Keep FFmpeg-dependent tests skip-safe when binaries are missing.
- Keep MIME detection and item type classification consistent with docs.

## Frontend Pattern

When touching templates/static assets:

- Check the template partial used by the handler before changing markup.
- Preserve HTMX partial contracts.
- Do not break mobile-friendly layout.
- Run web format/lint when changing JS/CSS/HTML if dependencies are installed.
- Avoid editing `web/static/app.min.js` manually unless the project specifically treats it as source.

## Subagent Use

Only use subagents when the user explicitly asks for orchestration, delegation, or parallel agent work.

Good independent splits:

- Backend implementation owner.
- SQLite/search owner.
- Upload/media owner.
- Security reviewer.
- Test/docs verifier.

Each subagent must get a disjoint file scope and must not revert unrelated changes.
