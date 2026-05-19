# Ephemeral Security Review

Use this for security audits or as a review pass after changes involving auth, uploads, file serving, templates, search, deployment, or external processes. Start with `repo-map.md`; finish with `verify.md` if patches are made.

## Review Priorities

Report findings first, ordered by severity. Use file and line references when possible. Prefer concrete exploit paths over generic advice.

Severity guide:

- Critical: remote code execution, arbitrary file read/write/delete, auth bypass, credential compromise.
- High: stored XSS, path traversal, upload abuse causing severe resource exhaustion, session theft, unsafe command invocation.
- Medium: weak rate limiting, missing validation, excessive information disclosure, incomplete authorization on sensitive paths.
- Low: defense-in-depth gaps, hardening opportunities, unclear errors.

## Auth and Session Checklist

- Public route list is intentionally small.
- Protected routes require `session_token`.
- Login validates missing and invalid credentials consistently.
- First-account setup cannot be abused once a user exists.
- Session tokens use cryptographic randomness.
- Password hashing uses bcrypt with an intentional cost.
- Cookies are `HttpOnly` and `SameSite=Lax`.
- Consider whether deployment requires `Secure` cookies behind HTTPS.
- Logout deletes server-side session and expires the cookie.
- Rolling session refresh cannot extend invalid or expired sessions.

## Upload and File Checklist

- Upload body size is bounded by `MAX_UPLOAD_SIZE`.
- Multipart parser handles malformed bodies and missing `file` field.
- File saves stream to disk; no unbounded request-body reads.
- Original filenames are reduced with `filepath.Base`.
- Stored paths are relative and resolved through safe path helpers.
- Absolute paths, `..`, and empty paths are rejected.
- Delete removes only files under the upload directory.
- Text preview reads are bounded by `TEXT_PREVIEW_MAX`.
- Body indexing reads are bounded by `BODY_INDEX_MAX`.
- File serving cannot expose database files, source files, or arbitrary host paths.

## XSS and HTML Checklist

- User-controlled text rendered in templates must rely on `html/template` escaping unless intentionally sanitized.
- Linkification must not create unsafe protocols or unescaped attributes.
- Filenames and metadata shown in HTML must be escaped.
- JSON responses should not embed raw HTML unless intended.
- Uploaded SVG is classified as previewable text/XML; consider browser execution behavior if serving SVG inline.

## External Process Checklist

- FFmpeg/FFprobe command arguments must be built as argument arrays, not shell strings.
- User-controlled paths must be passed as arguments, not interpolated through a shell.
- Processes must run with context timeouts.
- Generated thumbnail paths must stay under `uploads/thumbs`.
- Failures should degrade gracefully without corrupting item rows.

## SQLite/Search Checklist

- Query parameters should use placeholders, not string concatenation.
- Dynamic SQL must restrict user input to placeholder values.
- FTS query construction must quote or sanitize user tokens.
- Migrations must not drop user data unexpectedly.
- FTS cleanup triggers should prevent stale searchable content after delete/update.

## Rate Limit and Resource Checklist

- Rate limit classes separate auth-sensitive endpoints from default traffic.
- Static files, SSE, and uploaded file downloads are intentionally skipped; confirm this is acceptable for deployment.
- Upload/media concurrency defaults should fit Docker memory limits.
- FFmpeg child processes can exceed Go heap limits; account for container RSS, not only Go heap.
- Large preview/index limits should be bounded to avoid memory pressure.

## Deployment Checklist

- Docker runtime user is non-root.
- Data directory permissions match runtime user.
- Runtime image includes FFmpeg and CA certificates.
- Secrets are not baked into image or docs.
- `DATA_DIR` points to persistent storage in Compose.
- Reverse proxy limits are aligned with app upload limits.

## Output Format

Use this structure:

1. Findings, ordered by severity.
2. Open questions or assumptions.
3. Suggested fixes or implemented fixes.
4. Verification status.

If no issues are found, say that clearly and list remaining test gaps or environmental risks.
