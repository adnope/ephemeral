# Ephemeral API Documentation

Ephemeral provides a minimal set of authenticated endpoints for sharing text, uploading files, creating public file links, browsing history, previewing files, deleting items, and receiving real-time updates.

## Authentication

All protected endpoints require a valid `session_token` cookie.

Sessions are rolling sessions. When a session is close to expiry, authenticated requests refresh the session expiry and reset the cookie max age. The session TTL and secure-cookie behavior are configured with:

```env
SESSION_TTL=30d
COOKIE_SECURE=false
```

Supported examples:

```env
SESSION_TTL=30d
SESSION_TTL=720h
SESSION_TTL=24h
SESSION_TTL=2h
```

## Runtime Limits

These values are configured through environment variables:

```env
CHAT_PAGE_SIZE=100
HISTORY_PAGE_SIZE=100
SEARCH_RESULT_LIMIT=30
MAX_UPLOAD_SIZE=2GiB
TEXT_PREVIEW_MAX=10MiB
BODY_INDEX_MAX=20MiB
MEDIA_WORKER_COUNT=1
MEDIA_PROCESS_TIMEOUT=30m
HLS_MIN_SIZE=100MiB
HLS_MIN_DURATION=5m
UPLOAD_CONCURRENCY=1
```

Size values accept bytes or `KB`, `MB`, `GB`, `TB`, `KiB`, `MiB`, `GiB`, `TiB`.
Duration values accept Go duration strings such as `5m`, `30m`, or `1h`; `SESSION_TTL` also accepts day values such as `30d`.
JSON request bodies for JSON endpoints are limited to 64 KiB. `UPLOAD_CONCURRENCY` is enforced server-side and capped at 10.
Video uploads always get an asynchronous browser-friendly MP4 playback copy when FFmpeg succeeds. HLS is generated when either `HLS_MIN_SIZE` or `HLS_MIN_DURATION` is reached; set both to `0` to generate HLS for every video. The browser UI uses native HLS when available, HLS.js when MediaSource is available, and MP4 fallback otherwise.

## JSON API Conventions

Mobile/API clients should send:

```http
Accept: application/json
```

Message creation and upload endpoints always return JSON.
Login and logout retain form redirects for non-JSON clients.

JSON errors use:

```json
{
  "code": "validation_error",
  "message": "Human-readable error"
}
```

Common error codes:

```text
validation_error
unauthenticated
forbidden
not_found
payload_too_large
unsupported_preview
unsupported_share
server_error
```

Mobile item JSON shape:

```json
{
  "id": 42,
  "type": "image",
  "text": "",
  "filename": "photo.jpg",
  "filesizeBytes": 2048,
  "contentUrl": "/api/files/...",
  "downloadUrl": "/api/files/...",
  "createdAtEpochMillis": 1710000000000,
  "publicLinkActive": true,
  "metadata": {
    "width": 640,
    "height": 480,
    "duration": "",
    "mime": "image/jpeg",
    "thumbnailUrl": "/api/files/thumbs%2F...",
    "playbackUrl": "",
    "playbackMime": "",
    "hlsUrl": "",
    "processing": false
  }
}
```

For text items, `text` contains the message body and file URLs are empty. For uploaded items, `text` is empty and `contentUrl` / `downloadUrl` point to the original upload.
`publicLinkActive` is `true` only when the uploaded item has a public link that has not expired.

### Media Playback Contract

Media URL fields are server-relative URLs. Resolve them against the same origin used for the API, and use them exactly as returned:

```text
https://example.local + /api/files/hls%2Fexample%2Findex.m3u8
= https://example.local/api/files/hls%2Fexample%2Findex.m3u8
```

Do not split, decode, or rebuild `/api/files/...` URLs on the client. Filenames and generated media paths may contain spaces or Unicode characters, and the returned URL is already encoded for HTTP.

Video playback fields:

| Field                   | Meaning                                                       |
| ----------------------- | ------------------------------------------------------------- |
| `metadata.processing`   | Background thumbnail/playback/HLS processing is still running |
| `metadata.hlsUrl`       | HLS VOD playlist URL (`.m3u8`) when HLS was generated         |
| `metadata.playbackUrl`  | Browser-friendly MP4 playback copy URL                        |
| `metadata.playbackMime` | MIME type for `metadata.playbackUrl`, normally `video/mp4`    |
| `contentUrl`            | Original uploaded file URL                                    |
| `downloadUrl`           | Original uploaded file URL intended for download actions      |

Playback selection:

1. If `metadata.processing` is `true`, show the item as processing and refresh it after `item:updated` from `GET /api/events`, or by polling `GET /api/items` / `GET /api/history`.
2. If `metadata.hlsUrl` is non-empty and the client supports HLS, play `metadata.hlsUrl`.
3. Otherwise, if `metadata.playbackUrl` is non-empty, play `metadata.playbackUrl` using `metadata.playbackMime`.
4. Otherwise, play `contentUrl` only if the client supports the original container and codecs.

HLS is generated only when the upload reaches the configured `HLS_MIN_SIZE` or `HLS_MIN_DURATION` threshold. If neither threshold is reached, `metadata.hlsUrl` is empty and clients should use the MP4 playback copy. Deployments can set both thresholds to `0` to generate HLS for every video.

HLS playlists reference generated segment URLs under `/api/files/...`. Clients must send the same authenticated `session_token` cookie for the playlist request and every segment request. This also applies to `metadata.playbackUrl`, `contentUrl`, thumbnails, and downloads because `/api/files/...` is protected by session authentication.

Mobile page JSON shape:

```json
{
  "items": [],
  "nextCursor": 0,
  "hasMore": false
}
```

`cursor=0` means the first page. `nextCursor=0` means no next page. Items are newest first.

## Mobile JSON Endpoints

### `GET /api/auth/state`

Returns whether first-account setup is required.

**Auth**

Public.

**Response**

```json
{
  "setupRequired": true
}
```

---

### `GET /api/config`

Returns runtime values needed by mobile clients. Also validates a restored session.

**Auth**

Requires `session_token`.

**Response**

```json
{
  "chatPageSize": 100,
  "historyPageSize": 100,
  "maxUploadSizeBytes": 2147483648,
  "textPreviewMaxBytes": 10485760,
  "uploadConcurrency": 1
}
```

---

### `GET /api/items`

Returns the chat feed as JSON.

**Auth**

Requires `session_token`.

**Query params**

| Param    | Type    | Description                   |
| -------- | ------- | ----------------------------- |
| `cursor` | integer | Load items with `id < cursor` |

**Response**

Returns a mobile page JSON object.

---

### `GET /api/items/{id}`

Returns one item using the mobile item JSON shape.
The SPA uses this endpoint to refresh a specific item after a real-time event.

**Auth**

Requires `session_token`.

---

### `GET /api/history`

Returns history/search results as JSON.

**Auth**

Requires `session_token`.

**Query params**

| Param    | Type         | Description                                      |
| -------- | ------------ | ------------------------------------------------ |
| `cursor` | integer      | Load items with `id < cursor`                    |
| `type`   | string       | Filter by item type: `image`, `video`, or `file` |
| `q`      | string       | Search query                                     |
| `body`   | `1`          | Enable text/code file body search                |
| `from`   | `YYYY-MM-DD` | Start upload date                                |
| `to`     | `YYYY-MM-DD` | End upload date, inclusive                       |
| `recent` | string       | Recent-time preset                               |

Supported `recent` values:

```text
1d
7d
14d
30d
90d
6mo
1y
```

**Response**

Returns a mobile page JSON object.

---

### `GET /login`

Renders the login page.

On first run, if no user exists, the login form creates the initial user account.

### `POST /api/login`

Authenticates a user.

On first run, if no user exists, this creates the initial user account and starts a session.

**Form Content-Type**

```http
application/x-www-form-urlencoded
```

**Body**

| Field      | Type   | Required | Description |
| ---------- | ------ | -------: | ----------- |
| `username` | string |      yes | Username    |
| `password` | string |      yes | Password    |

**Response**

On success:

```text
303 See Other -> /
```

Sets:

```http
Set-Cookie: session_token=...
```

On failure:

```text
303 See Other -> /login?error=invalid+credentials
```

**JSON request**

```http
Accept: application/json
Content-Type: application/json
```

```json
{
  "username": "alice",
  "password": "secret"
}
```

**JSON response**

```json
{
  "authenticated": true
}
```

Also sets `session_token`.

### `POST /api/logout`

Invalidates the current session and clears the session cookie.

**Response**

```text
303 See Other -> /login
```

For JSON requests:

```text
204 No Content
```

## API Endpoints

### `POST /api/message`

Creates a text message.

**Content-Type**

```http
application/x-www-form-urlencoded
```

**Body**

| Field  | Type   | Required | Description     |
| ------ | ------ | -------: | --------------- |
| `text` | string |      yes | Message content |

**Response**

Send JSON:

```json
{
  "text": "message body"
}
```

The response is one mobile item JSON object.

**Side effects**

Emits SSE event:

```text
item:new
```

---

### `POST /api/upload`

Uploads one file.

**Content-Type**

```http
multipart/form-data
```

**Body**

| Field  | Type | Required | Description    |
| ------ | ---- | -------: | -------------- |
| `file` | file |      yes | File to upload |

**Response**

Send the `multipart/form-data` body with:

```http
Accept: application/json
```

The response is always one mobile item JSON object.

**Behavior**

- Detects MIME type.
- Stores the file under the upload directory.
- Rejects requests above `MAX_UPLOAD_SIZE`.
- Creates an `items` database row.
- For images/videos, metadata extraction and thumbnail generation run asynchronously.
- For videos, browser-friendly MP4 playback copy generation runs asynchronously. Large or long videos also get HLS playlists and segments based on runtime thresholds.
- For text/code-like files up to `BODY_INDEX_MAX`, body content is indexed into SQLite FTS5 for history body search.

**Side effects**

Emits:

```text
item:new
```

Then later, after media processing:

```text
item:updated
```

---

### `GET /api/files/{path}`

Serves an uploaded file, generated thumbnail, generated MP4 playback copy, or generated HLS playlist/segment.

**Examples**

```http
GET /api/files/1710000000000_photo.png
GET /api/files/thumbs/1710000000000_video_thumb.jpg
GET /api/files/playback/1710000000000_video_playback.mp4
GET /api/files/hls/1710000000000_video/index.m3u8
```

**Response**

Returns the file content using `http.ServeFile`.

Standard HTTP range requests are supported. This is useful for MP4 seeking and for media players that request byte ranges.

Generated HLS files use these response content types:

| Extension | Content-Type                    |
| --------- | ------------------------------- |
| `.m3u8`   | `application/vnd.apple.mpegurl` |
| `.ts`     | `video/mp2t`                    |

**Notes**

- Supports filenames with spaces and Unicode characters.
- Rejects unsafe paths such as absolute paths or `..`.
- HLS playlists may contain root-relative segment URLs such as `/api/files/hls%2Fexample%2Fsegment_00000.ts`; resolve them against the same API origin and include the authenticated session cookie on each request.
- Active document uploads such as HTML, SVG, XML, and XHTML are served with a sandbox Content Security Policy when opened directly.

---

### `GET /api/items/download-zip`

Streams a single compressed ZIP file containing all selected items.

**Auth**

Requires `session_token`.

**Query params**

| Param | Type   | Required | Description                                                    |
| ----- | ------ | -------: | -------------------------------------------------------------- |
| `ids` | string |      yes | Comma-separated list of item database IDs (e.g. `ids=1,2,3,4`) |

**Response**

Returns a ZIP file containing the selected items with `Content-Type: application/zip` and `Content-Disposition: attachment; filename="ephemeral_download.zip"`.

- **Text items**: Packaged dynamically as virtual files named `message_{id}.txt`.
- **Binary items** (`image`, `video`, `file`): Serves their original uploaded data.
- **Name conflict resolution**: Duplicate filenames inside the ZIP archive are automatically deduplicated (e.g., `photo.png` and `photo (1).png`).
- Non-existent or inaccessible IDs are skipped dynamically.

**Status codes**

|  Code | Meaning                              |
| ----: | ------------------------------------ |
| `200` | ZIP file stream initiated            |
| `400` | Missing or invalid `ids` parameter   |
| `404` | No valid items found to download     |
| `500` | Failed to package or retrieve items  |

---

### `GET /api/items/{id}/public-link`

Returns the current public-link state for one uploaded item.

Text items cannot be shared as public file links.

**Auth**

Requires `session_token`.

**Response**

No link:

```json
{
  "status": "none",
  "expires_at": null
}
```

Active link:

```json
{
  "status": "active",
  "url": "/share/7_D5s5pJrBNppqJ0mAwbXlLh8r53gzWmBB2Z45TcaZU",
  "token": "7_D5s5pJrBNppqJ0mAwbXlLh8r53gzWmBB2Z45TcaZU",
  "expires_at": "2026-06-01T08:00:00Z"
}
```

Expired link:

```json
{
  "status": "expired",
  "url": "/share/7_D5s5pJrBNppqJ0mAwbXlLh8r53gzWmBB2Z45TcaZU",
  "token": "7_D5s5pJrBNppqJ0mAwbXlLh8r53gzWmBB2Z45TcaZU",
  "expires_at": "2026-05-25T08:00:00Z"
}
```

Expired links remain stored until the owner revokes the link or creates a new link for the same item.

**Status codes**

|  Code | Meaning                                     |
| ----: | ------------------------------------------- |
| `200` | Link state returned                         |
| `400` | Invalid item ID                             |
| `404` | Item not found                              |
| `415` | Item cannot be shared as a public file link |
| `500` | Link lookup failed                          |

---

### `POST /api/items/{id}/public-link`

Creates or replaces the public link for one uploaded item.

Text items cannot be shared as public file links. Uploaded images and videos open in a browser-view page. Generic files download when the public link is opened.

Each uploaded item has at most one public link:

- no existing link: creates a new token
- active existing link: updates `expires_at` and keeps the same token/URL
- expired existing link: deletes the expired row and creates a new token

The web dialog defaults new links to 24 hours. API clients may still send `null` to create a non-expiring link.

**Auth**

Requires `session_token`.

**Content-Type**

```http
application/json
```

**Body**

`expires_in_seconds` controls expiry. Use `null` for a link that never expires.

```json
{
  "expires_in_seconds": 604800
}
```

Never expire:

```json
{
  "expires_in_seconds": null
}
```

**Response**

```json
{
  "url": "/share/7_D5s5pJrBNppqJ0mAwbXlLh8r53gzWmBB2Z45TcaZU",
  "token": "7_D5s5pJrBNppqJ0mAwbXlLh8r53gzWmBB2Z45TcaZU",
  "expires_at": "2026-06-01T08:00:00Z"
}
```

For non-expiring links, `expires_at` is `null`.

Creating or updating a link emits an `item:updated` SSE event so authenticated clients can refresh `publicLinkActive`.

**Status codes**

|  Code | Meaning                                     |
| ----: | ------------------------------------------- |
| `200` | Link created or replaced                    |
| `400` | Invalid item ID, JSON body, or expiry       |
| `404` | Item not found                              |
| `413` | JSON body too large                         |
| `415` | Item cannot be shared as a public file link |
| `500` | Link creation failed                        |

---

### `DELETE /api/items/{id}/public-link`

Revokes the public link for one item. The operation is idempotent when the item has no stored link.

**Auth**

Requires `session_token`.

**Response**

```text
204 No Content
```

Validation/not-found/server errors use the shared JSON error shape.

Revoking a link emits an `item:updated` SSE event so authenticated clients can refresh `publicLinkActive`.

---

### `GET /share/{token}`

Public route. Does not require `session_token`.

**Behavior**

- Returns `404` for missing, malformed, expired, revoked, or deleted-item links.
- Image and video links render an HTML page with browser media controls.
- Processing video pages show a processing state. Completed video pages use the generated browser-friendly MP4 playback copy when available, then fall back to the original upload.
- Generic file links return the original file with `Content-Disposition: attachment`.
- Public file responses set `Cache-Control: private, no-store`.

Supporting public media URLs are implementation-owned and may be used by the rendered page:

```text
GET /share/{token}/file
GET /share/{token}/download
GET /share/{token}/thumb
```

`/share/{token}/download` always serves the original uploaded file as an attachment.

---

### `GET /api/share/{token}`

Returns the metadata needed by the public Vue view for an image or video share.
This route is public and does not require `session_token`.

```json
{
  "filename": "clip.mp4",
  "filesizeBytes": 2048,
  "itemType": "video",
  "mime": "video/mp4",
  "sourceUrl": "/share/token/file",
  "posterUrl": "/share/token/thumb",
  "downloadUrl": "/share/token/download",
  "expiresAt": "2026-06-01T08:00:00Z",
  "processing": false
}
```

Generic file shares return `415 unsupported_share` so browser clients can navigate directly to `/share/{token}/download`.

---

### `GET /api/file-preview/{id}`

Returns bounded text/code file content for the in-app preview dialog.

Only supports generic file items that are text/code-like and below the preview size limit.
The preview size limit is `TEXT_PREVIEW_MAX`.

**Response**

```json
{
  "id": 123,
  "filename": "main.go",
  "mime": "text/x-go",
  "language": "go",
  "content": "package main\n...",
  "filesize": 2048,
  "created_at": "May 15, 2026 10:30 AM",
  "download_url": "/api/files/..."
}
```

**Status codes**

|  Code | Meaning                         |
| ----: | ------------------------------- |
| `200` | Preview returned                |
| `400` | Invalid item ID                 |
| `403` | Forbidden file path             |
| `404` | Item not found                  |
| `413` | File too large for preview      |
| `415` | File is not previewable as text |
| `500` | Preview read failed             |

---

### `DELETE /api/items/{id}`

Deletes an item permanently.

**Behavior**

- Deletes the database row.
- Deletes the uploaded file if the item is not text.
- Deletes generated thumbnail if present.
- FTS cleanup is handled by database triggers.

**Response**

```text
204 No Content
```

For JSON requests, validation/not-found/server errors use the shared JSON error shape.

**Side effects**

Emits SSE event:

```text
item:deleted
```

## SPA Routes

The Go binary serves the embedded Vue SPA shell for these routes.

### `GET /`

Main chat interface.
The client loads data from `GET /api/items` and applies SSE changes reactively.

---

### `GET /history`

History/gallery interface.
Filter query parameters are preserved in the browser URL and passed to `GET /api/history`.

---

### `GET /login`

Login and first-account setup interface.

### `GET /share/{token}`

Public share interface.

## Real-Time Updates

Ephemeral uses Server-Sent Events.

### `GET /api/events`

Opens the SSE stream.

**Protocol**

```http
text/event-stream
```

Each item event includes a monotonically increasing SSE `id` within the running server process.
Browsers automatically send the last received ID when reconnecting, and the server replays retained events after that ID.
If the requested ID is no longer available or belongs to an earlier server process, the server emits `stream:reset` so the UI can reconcile against the current item collection.

### Browser example

```javascript
const events = new EventSource("/api/events");

events.addEventListener("item:new", (event) => {
  const itemId = Number.parseInt(event.data, 10);
  console.log("New item:", itemId);
});

events.addEventListener("item:updated", (event) => {
  const itemId = Number.parseInt(event.data, 10);
  console.log("Updated item:", itemId);
});

events.addEventListener("item:deleted", (event) => {
  const itemId = Number.parseInt(event.data, 10);
  console.log("Deleted item:", itemId);
});

events.addEventListener("stream:reset", () => {
  console.log("Reconcile the current item collection");
});

events.onerror = (error) => {
  console.error("SSE error:", error);
};
```

### Event Types

| Event          | Description                                                                          |
| -------------- | ------------------------------------------------------------------------------------ |
| `item:new`     | New text message or uploaded file was created                                        |
| `item:updated` | Media processing completed or the item's active public-link state changed            |
| `item:deleted` | Item was permanently deleted                                                         |
| `stream:reset` | Retained replay is unavailable and the client must reconcile                         |

## Data Model Summary

Items can have one of these types:

```text
text
image
video
file
```

Generic files may be previewable as text/code if their MIME type or extension is supported.
Public file links are stored separately from items using an opaque random token, an `item_id`, and a nullable `expires_at`. Deleting an item cascades to its public link.

Image and video thumbnails, video playback copies, and HLS outputs are stored under:

```text
uploads/thumbs/
uploads/playback/
uploads/hls/
```

and referenced through metadata as relative upload paths:

```json
{
  "thumb": "thumbs/example_thumb.jpg",
  "playback": "playback/example_playback.mp4",
  "playbackMime": "video/mp4",
  "hls": "hls/example/index.m3u8",
  "processing": false
}
```
