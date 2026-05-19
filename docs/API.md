# Ephemeral API Documentation

Ephemeral provides a minimal set of authenticated endpoints for sharing text, uploading files, browsing history, previewing files, deleting items, and receiving real-time updates.

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
UPLOAD_CONCURRENCY=1
```

Size values accept bytes or `KB`, `MB`, `GB`, `TB`, `KiB`, `MiB`, `GiB`, `TiB`.
JSON request bodies for JSON endpoints are limited to 64 KiB. `UPLOAD_CONCURRENCY` is enforced server-side and capped at 10.

## JSON API Conventions

Mobile/API clients should send:

```http
Accept: application/json
```

For shared endpoints such as login, logout, message creation, and upload, JSON requests receive JSON/status responses while browser form/HTMX requests keep the existing redirect or HTML partial behavior.

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
  "metadata": {
    "width": 640,
    "height": 480,
    "duration": "",
    "mime": "image/jpeg",
    "thumbnailUrl": "/api/files/thumbs%2F..."
  }
}
```

For text items, `text` contains the message body and file URLs are empty. For uploaded items, `text` is empty and file URLs point to the existing file-serving endpoint.

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

### `GET /api/history`

Returns history/search results as JSON.

**Auth**

Requires `session_token`.

**Query params**

| Param    | Type         | Description                                        |
| -------- | ------------ | -------------------------------------------------- |
| `cursor` | integer      | Load items with `id < cursor`                      |
| `type`   | string       | Filter by item type: `image`, `video`, or `file`   |
| `q`      | string       | Search query                                       |
| `body`   | `1`          | Enable text/code file body search                  |
| `from`   | `YYYY-MM-DD` | Start upload date                                  |
| `to`     | `YYYY-MM-DD` | End upload date, inclusive                         |
| `recent` | string       | Recent-time preset                                 |

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

Returns the rendered `item_partial` HTML for the newly created message.

For JSON requests, send:

```json
{
  "text": "message body"
}
```

and receive one mobile item JSON object.

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

Returns the rendered `item_partial` HTML for the uploaded item.

For JSON requests, send the same `multipart/form-data` body with:

```http
Accept: application/json
```

The response is one mobile item JSON object.

**Behavior**

- Detects MIME type.
- Stores the file under the upload directory.
- Rejects requests above `MAX_UPLOAD_SIZE`.
- Creates an `items` database row.
- For images/videos, metadata extraction and thumbnail generation run asynchronously.
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

Serves an uploaded file or generated thumbnail.

**Examples**

```http
GET /api/files/1710000000000_photo.png
GET /api/files/thumbs/1710000000000_video_thumb.jpg
```

**Response**

Returns the file content using `http.ServeFile`.

**Notes**

- Supports filenames with spaces and Unicode characters.
- Rejects unsafe paths such as absolute paths or `..`.
- Active document uploads such as HTML, SVG, XML, and XHTML are served with a sandbox Content Security Policy when opened directly.

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

## View Endpoints

These endpoints return server-rendered HTML.

### `GET /`

Main chat interface.

**Query params**

| Param    | Type    | Description                   |
| -------- | ------- | ----------------------------- |
| `cursor` | integer | Load items with `id < cursor` |

Used for cursor-based pagination and infinite scrolling.
The page size is `CHAT_PAGE_SIZE`.

When requested via HTMX, returns only the `items_partial` HTML.

---

### `GET /history`

History/gallery interface.

**Query params**

| Param    | Type         | Description                                        |
| -------- | ------------ | -------------------------------------------------- |
| `cursor` | integer      | Load items with `id < cursor`                      |
| `type`   | string       | Filter by item type, e.g. `image`, `video`, `file` |
| `q`      | string       | Search query                                       |
| `body`   | `1`          | Enable text/code file body search                  |
| `from`   | `YYYY-MM-DD` | Start upload date                                  |
| `to`     | `YYYY-MM-DD` | End upload date                                    |
| `recent` | string       | Recent-time preset                                 |

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

When requested via HTMX, returns only the `history_items` HTML.
The page size is `HISTORY_PAGE_SIZE`.

---

### `GET /search`

Searches existing items using the older item FTS search endpoint.

**Query params**

| Param | Type   | Required | Description  |
| ----- | ------ | -------: | ------------ |
| `q`   | string |      yes | Search query |

**Response**

Returns rendered `items_partial` HTML.
Returns at most `SEARCH_RESULT_LIMIT` items.

## Real-Time Updates

Ephemeral uses Server-Sent Events.

### `GET /api/events`

Opens the SSE stream.

**Protocol**

```http
text/event-stream
```

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

events.onerror = (error) => {
  console.error("SSE error:", error);
};
```

### Event Types

| Event          | Description                                        |
| -------------- | -------------------------------------------------- |
| `item:new`     | New text message or uploaded file was created      |
| `item:updated` | Background metadata/thumbnail processing completed |
| `item:deleted` | Item was permanently deleted                       |

## Data Model Summary

Items can have one of these types:

```text
text
image
video
file
```

Generic files may be previewable as text/code if their MIME type or extension is supported.

Image and video thumbnails are stored under:

```text
uploads/thumbs/
```

and referenced through metadata as:

```json
{
  "thumb": "thumbs/example_thumb.jpg"
}
```
