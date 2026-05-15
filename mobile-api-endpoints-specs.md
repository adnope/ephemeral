# Ephemeral Mobile API Endpoint Contract

This contract defines the smallest backend API surface the Android app needs to fully communicate with Ephemeral.

The design intentionally reuses the current backend routes and use cases where possible. Avoid creating a parallel `/api/mobile/v1` tree unless the backend later needs hard API versioning for multiple public clients.

## Contract Goals

- Provide JSON data needed by the Android app.
- Preserve existing web/HTMX behavior.
- Avoid duplicate upload, download, thumbnail, preview, auth, and SSE code paths.
- Keep binary file serving on the existing file-serving endpoint.
- Keep events as lightweight invalidation signals.
- Make unauthenticated API requests return `401` JSON instead of web redirects.

## Problems In The Previous Draft

- `/api/mobile/v1` duplicated the existing `/api` surface without a current versioning need.
- Separate `/setup/first-account` duplicated current login behavior, because the existing auth use case creates the first account when no user exists.
- Separate `/auth/session` was unnecessary because the Android app can validate a restored session by calling the protected config endpoint.
- Separate `/items/{id}/download`, `/items/{id}/content`, and `/items/{id}/thumbnail` duplicated the existing `/api/files/*` binary route. `http.ServeFile` already supports range requests for video.
- Item ID based file endpoints would require extra lookup and path-resolution handlers for every download/preview, while the backend already stores content paths and serves them safely.
- JSON SSE payloads duplicated existing event semantics. The current SSE stream already sends event name plus item ID.
- `displayName`, `mimeType`, and `sizeBytes` upload parts duplicated server-side filename, size, and MIME detection.
- A `previewable` field would duplicate backend previewability logic. The app can call preview and handle `413`/`415`, matching current web behavior.

## Common Rules

Android API requests must send:

```http
Accept: application/json
```

For endpoints shared with the web UI, the backend should preserve current HTML/redirect behavior unless `Accept: application/json` is present.

Authentication:

- Use the existing `session_token` HttpOnly cookie.
- Login returns `Set-Cookie`.
- Logout clears the cookie.
- Protected JSON requests without a valid session return `401` with the JSON error shape below.
- Protected web requests without a valid session may continue redirecting to `/login`.

Timestamps:

- New mobile JSON item/page responses use `createdAtEpochMillis`.
- Existing preview response fields may be preserved for web compatibility.

Numbers:

- IDs, cursors, file sizes, and byte counters are signed 64-bit integers.

Pagination:

- `cursor=0` means first page.
- `nextCursor=0` means no next page.
- Sort order is newest first.
- `nextCursor` is the ID of the last item in the returned page.

Error shape for JSON responses:

```json
{
  "code": "validation_error",
  "message": "Username is required."
}
```

Required error codes:

- `validation_error`
- `unauthenticated`
- `forbidden`
- `not_found`
- `payload_too_large`
- `unsupported_preview`
- `server_error`

## Endpoint Summary

| Method | Path | Auth | Status | Purpose |
| :--- | :--- | :---: | :--- | :--- |
| `GET` | `/api/auth/state` | No | New | Returns whether first-account setup is required. |
| `POST` | `/api/login` | No | Existing, add JSON mode | Login or first-account setup. |
| `POST` | `/api/logout` | Yes | Existing, add JSON mode | Invalidates session and clears cookie. |
| `GET` | `/api/config` | Yes | New | Runtime UI limits and session validation. |
| `GET` | `/api/items?cursor=0` | Yes | New | Chat feed page, newest first. |
| `POST` | `/api/message` | Yes | Existing, add JSON mode | Creates a text item. |
| `POST` | `/api/upload` | Yes | Existing, add JSON mode | Streams one uploaded file. |
| `DELETE` | `/api/items/{itemId}` | Yes | Existing | Permanently deletes one item. |
| `GET` | `/api/history?...` | Yes | New | Search/filter history page. |
| `GET` | `/api/file-preview/{itemId}` | Yes | Existing | Bounded text/code preview. |
| `GET` | `/api/files/{path}` | Yes | Existing | Original files and generated thumbnails. |
| `GET` | `/api/events` | Yes | Existing | SSE item event stream. |

## Removed Endpoints From Previous Draft

Do not add these endpoints for the first Android API pass:

- `POST /setup/first-account`
- `POST /auth/login`
- `POST /auth/logout`
- `GET /auth/session`
- `GET /chat`
- `POST /items/text`
- `POST /uploads`
- `GET /items/{itemId}/preview`
- `GET /items/{itemId}/download`
- `GET /items/{itemId}/content`
- `GET /items/{itemId}/thumbnail`
- `GET /events`

Their responsibilities are covered by existing `/api` routes or by the small new routes listed above.

## Shared JSON Types

### Item

Used by chat pages, history pages, text-message creation, and upload responses.

```json
{
  "id": 42,
  "type": "image",
  "text": "",
  "filename": "a.jpg",
  "filesizeBytes": 2048,
  "contentUrl": "/api/files/1710000000000_a.jpg",
  "downloadUrl": "/api/files/1710000000000_a.jpg",
  "createdAtEpochMillis": 1710000000000,
  "metadata": {
    "width": 640,
    "height": 480,
    "duration": "",
    "mime": "image/jpeg",
    "thumbnailUrl": "/api/files/thumbs/1710000000000_a_thumb.jpg"
  }
}
```

Rules:

- `type` is one of `text`, `image`, `video`, `file`.
- For `text` items, `text` contains the message body and file fields are empty or zero.
- For upload items, `text` is empty.
- `contentUrl` points to `/api/files/{path}` for uploaded files.
- `downloadUrl` can equal `contentUrl`; Android controls whether the stream is displayed inline or saved.
- `metadata.thumbnailUrl` is empty until an image or video thumbnail exists.
- URL fields are server-generated and must be treated as opaque by Android. Do not construct file URLs client-side from filenames.
- Do not include a `previewable` field in the first pass. Android should call preview for generic file View and handle `413` or `415`.

### Page

```json
{
  "items": [],
  "nextCursor": 42,
  "hasMore": true
}
```

`hasMore` is equivalent to `nextCursor != 0`. It is included for client convenience.

### Runtime Config

```json
{
  "chatPageSize": 100,
  "historyPageSize": 100,
  "maxUploadSizeBytes": 2147483648,
  "textPreviewMaxBytes": 10485760,
  "uploadConcurrency": 1
}
```

Only include values the Android UI directly needs. Do not expose internal-only values such as media worker count unless the mobile app needs them later.

## Endpoint Details

### GET /api/auth/state

Public endpoint.

Returns whether the server needs first-account setup.

Response:

```json
{
  "setupRequired": true
}
```

Implementation note:

- Reuse the current auth login-page use case that already counts users.
- Do not render the login template for this endpoint.

### POST /api/login

Public endpoint.

Reuses the current login route and auth use case.

Request:

```json
{
  "username": "alice",
  "password": "secret"
}
```

Response:

```json
{
  "authenticated": true
}
```

Also returns `Set-Cookie: session_token=...`.

Rules:

- If no user exists, this creates the first account and starts a session.
- If a user exists, this authenticates normally.
- For non-JSON web form requests, preserve the existing redirect behavior.
- For JSON requests, return status codes and JSON errors, not redirects.

Expected JSON errors:

- `400 validation_error` for missing credentials.
- `401 unauthenticated` for invalid credentials.
- `500 server_error` for unexpected failures.

### POST /api/logout

Protected endpoint.

Reuses the current logout route.

JSON response:

```http
204 No Content
```

Rules:

- Clear the session cookie.
- Delete the session if a valid cookie exists.
- For non-JSON web form requests, preserve the existing redirect behavior.

### GET /api/config

Protected endpoint.

Returns runtime limits needed by Android and validates the restored session.

Response:

```json
{
  "chatPageSize": 100,
  "historyPageSize": 100,
  "maxUploadSizeBytes": 2147483648,
  "textPreviewMaxBytes": 10485760,
  "uploadConcurrency": 1
}
```

Rules:

- Android should call this after launch when it has a stored cookie.
- `200` means the session is valid.
- `401 unauthenticated` means clear local session and show login.

### GET /api/items

Protected endpoint.

Returns a chat feed page, newest first.

Query params:

- `cursor`: optional signed 64-bit integer, default `0`.

Response:

```json
{
  "items": [],
  "nextCursor": 0,
  "hasMore": false
}
```

Implementation note:

- Reuse `ItemUseCase.List`.
- This is the JSON equivalent of the current chat page data, not a new data source.

### POST /api/message

Protected endpoint.

Reuses the current text-message route and use case.

Request:

```json
{
  "text": "message body"
}
```

Response:

```json
{
  "id": 43,
  "type": "text",
  "text": "message body",
  "filename": "",
  "filesizeBytes": 0,
  "contentUrl": "",
  "downloadUrl": "",
  "createdAtEpochMillis": 1710000000000,
  "metadata": {
    "width": 0,
    "height": 0,
    "duration": "",
    "mime": "",
    "thumbnailUrl": ""
  }
}
```

Rules:

- Trim and reject empty messages on the backend.
- For non-JSON web requests, preserve current form parsing and HTML partial response.
- For JSON requests, return an `Item`.

Expected JSON errors:

- `400 validation_error` for empty message.
- `500 server_error` for unexpected failures.

### POST /api/upload

Protected endpoint.

Reuses the current upload route and use case.

Request:

- `multipart/form-data`
- One file part named `file`.

Do not require extra `displayName`, `mimeType`, or `sizeBytes` parts in the contract.

Response:

```json
{
  "id": 44,
  "type": "file",
  "text": "",
  "filename": "report.pdf",
  "filesizeBytes": 102400,
  "contentUrl": "/api/files/1710000000000_report.pdf",
  "downloadUrl": "/api/files/1710000000000_report.pdf",
  "createdAtEpochMillis": 1710000000000,
  "metadata": {
    "width": 0,
    "height": 0,
    "duration": "",
    "mime": "application/pdf",
    "thumbnailUrl": ""
  }
}
```

Rules:

- Backend owns filename sanitization, size measurement, MIME detection, item creation, indexing, and async media processing.
- For non-JSON web requests, preserve current HTML partial response.
- For JSON requests, return an `Item`.
- Return `413 payload_too_large` when the upload exceeds `maxUploadSizeBytes`.

### DELETE /api/items/{itemId}

Protected endpoint.

Existing route.

Response:

```http
204 No Content
```

Rules:

- Deletes the item.
- Deletes uploaded file and thumbnail best-effort for non-text items.
- Emits `item:deleted`.
- Current `404 not_found` is acceptable; Android should treat it as successful local removal.
- Optionally make delete idempotent later by returning `204` for already-deleted items.

### GET /api/history

Protected endpoint.

Returns a history page with filters.

Query params:

- `cursor`: optional signed 64-bit integer, default `0`.
- `type`: optional; one of `image`, `video`, `file`; omit for all.
- `q`: optional search query.
- `body`: optional; `1` to search text/code file body.
- `from`: optional date, `YYYY-MM-DD`.
- `to`: optional date, `YYYY-MM-DD`, inclusive.
- `recent`: optional; one of `1d`, `7d`, `14d`, `30d`, `90d`, `6mo`, `1y`.

Response:

```json
{
  "items": [],
  "nextCursor": 0,
  "hasMore": false
}
```

Implementation note:

- Reuse `HistoryUseCase.Search`.
- Keep query param names aligned with the current web history page to avoid duplicate parsing concepts.
- Android can map its `all/images/videos/files` UI to omit `type`, `type=image`, `type=video`, or `type=file`.

Expected JSON errors:

- `400 validation_error` for invalid dates or filters.
- `500 server_error` for unexpected failures.

### GET /api/file-preview/{itemId}

Protected endpoint.

Existing JSON endpoint.

Current response shape is acceptable:

```json
{
  "id": 7,
  "filename": "main.go",
  "mime": "text/plain",
  "language": "go",
  "content": "package main",
  "filesize": 12,
  "created_at": "May 15, 2026 10:30 AM",
  "download_url": "/api/files/1710000000000_main.go"
}
```

Rules:

- Preserve current fields for the web text viewer.
- Android should support this existing shape.
- If desired later, add alias fields such as `filesizeBytes`, `createdAtEpochMillis`, and `downloadUrl` without removing the existing fields.

Expected JSON errors:

- `400 validation_error` for invalid item ID.
- `403 forbidden`.
- `404 not_found`.
- `413 payload_too_large`.
- `415 unsupported_preview`.
- `500 server_error`.

### GET /api/files/{path}

Protected endpoint.

Existing binary endpoint for original uploads and generated image/video thumbnails.

Rules:

- `contentUrl`, `downloadUrl`, and `metadata.thumbnailUrl` point here.
- Android uses the same URL for image display, video playback, file download, and thumbnail loading.
- `http.ServeFile` supports range requests, so no separate video content endpoint is required.
- Android must use the URL returned in item/preview JSON instead of building this path itself.
- Backend continues rejecting unsafe paths.

### GET /api/events

Protected endpoint.

Existing SSE endpoint.

Current event format is acceptable:

```text
event: item:new
data: 42

event: item:updated
data: 42

event: item:deleted
data: 42
```

Rules:

- Android parses `data` as item ID.
- `item:new`: refresh the first chat/history page or insert if already known.
- `item:updated`: refresh visible pages or reload affected visible item from a future single-item endpoint if added later.
- `item:deleted`: remove item ID from all visible lists and close any viewer showing it.
- Events are best-effort invalidation. The app must tolerate missed events by refreshing pages.

## Optional Later Endpoint

Add this only if Android update handling becomes inefficient:

| Method | Path | Auth | Purpose |
| :--- | :--- | :---: | :--- |
| `GET` | `/api/items/{itemId}` | Yes | Return one item as JSON. |

This is not required for the first Android API because events can trigger page refreshes.

## Backend Implementation Notes

- Add a small JSON response helper and shared item serializer.
- Do not duplicate use cases for mobile.
- Add content negotiation to existing `POST /api/login`, `POST /api/logout`, `POST /api/message`, and `POST /api/upload`.
- Add JSON auth behavior in middleware for `/api/*`: return `401` JSON instead of redirect when the request accepts JSON.
- Add only these new handlers initially: auth state, config, item list, history JSON.
- Keep file serving centralized in `ServeFile`.
- Keep SSE broker unchanged.
- Keep web templates and HTMX partial responses unchanged.

For this self-hosted app, prefer consistency for item/session writes over availability during DB partitions. Event delivery can be best-effort because clients treat events as invalidation and can refresh visible pages.
