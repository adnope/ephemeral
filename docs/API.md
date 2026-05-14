# Ephemeral API Documentation

Ephemeral provides a minimal and focused set of endpoints for interacting with the application, managing content, and receiving real-time updates.

## Authentication

All protected endpoints require a valid session cookie (`session_token`). Authentication is managed via the login endpoints.

- **`POST /login`**: Authenticates a user.
  - **Content-Type**: `application/x-www-form-urlencoded`
  - **Body**: `username` (string), `password` (string)
  - **Response**: Sets the `session_token` cookie and redirects to `/`. On the very first run (when no users exist in the database), it creates the primary user account using the provided credentials.

- **`POST /logout`**: Clears the session cookie and invalidates the session on the server.

- **`GET /login`**: Renders the login page.

## API Endpoints

### Post a Text Message
- **Endpoint**: `POST /message`
- **Description**: Sends a new text message to the chat stream.
- **Content-Type**: `application/x-www-form-urlencoded`
- **Body**:
  - `text` (string): The content of the message.
- **Response**: Returns the HTML partial for the newly created item, which can be swapped directly into the DOM (designed for HTMX).

### Upload a File
- **Endpoint**: `POST /upload`
- **Description**: Uploads a file (image, video, or generic document).
- **Content-Type**: `multipart/form-data`
- **Body**:
  - `file` (file): The file to upload.
- **Response**: HTTP 200 OK on success. Generates thumbnails and extracts metadata asynchronously for media files via background workers.

### Serve a File
- **Endpoint**: `GET /files/{path}`
- **Description**: Serves an uploaded file or its generated thumbnail.

## View Endpoints (UI)

These endpoints return HTML views rendered by the server.

- **`GET /`**: Main chat interface.
  - **Query Params**: `cursor` (optional) - The ID of the last fetched item to support pagination (load more).
- **`GET /history`**: The gallery/history view for browsing past media and files.
- **`GET /search`**: Searches through past items.

## Real-Time Updates (Server-Sent Events)

Ephemeral uses standard Server-Sent Events (SSE) to push real-time updates to connected clients without the overhead of WebSockets.

- **Endpoint**: `GET /api/events`
- **Protocol**: HTTP Server-Sent Events (SSE)

### How to Connect

You can connect to the event stream using the native browser `EventSource` API:

```javascript
const eventSource = new EventSource('/api/events');

// Fired when a new message is saved or a file upload completes
eventSource.addEventListener('item:new', (event) => {
    console.log('New item received!', event.data);
    // E.g., trigger an HTMX request to fetch the latest messages
});

// Fired when background processing for an item finishes
eventSource.addEventListener('item:updated', (event) => {
    console.log('Item updated!', event.data);
    // E.g., update the UI to show the newly generated thumbnail
});

eventSource.onerror = (err) => {
    console.error('SSE Error:', err);
    // The EventSource will automatically attempt to reconnect
};
```

### Event Types

- **`item:new`**: Emitted immediately when a new text message is saved to the database or a file upload initially completes.
- **`item:updated`**: Emitted when background processing for an item finishes (e.g., when image dimensions are calculated or a video thumbnail is fully generated).
generated).
