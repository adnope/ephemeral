# Project Specification: LeanDrop

## System Overview

LeanDrop is a single-user, self-hosted web application for instant text and file synchronization between devices over a Tailscale network. It uses a server-driven architecture to keep client-side resource usage minimal.

- Host Target: Arch Linux (Running via Docker or native binary).
- Client Target: Android (via PWA/Web Browser).
- Resource Budget: < 30MB RAM (Host), < 50MB RAM (Client).

## Tech Stack

- Backend: Go 1.21+ (Standard Library + chi router).
- Database: SQLite 3 (via modernc.org/sqlite for a CGO-free, memory-efficient driver).
- Frontend: HTMX (for AJAX-less updates), Tailwind CSS (Standalone CLI), and Alpine.js (for light UI logic like modals).
- Real-time: Server-Sent Events (SSE) for "Instant Sync" (Lighter than WebSockets).
- Media Processing: ffmpeg and ffprobe (system calls) for video; Go image package for images.

##  Core Requirements & Logic

###  Data Persistence (SQLite)

```sql
CREATE TABLE items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    type TEXT, -- 'text', 'image', 'video', 'file'
    content TEXT, -- message text or file path
    filename TEXT,
    filesize INTEGER,
    metadata TEXT, -- JSON: {width: 100, height: 100, duration: "00:05", mime: "text/x-python"}
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

### High-Performance File Handling (The "Zero-Copy" Rule)

- Uploads: Use io.Copy from the multipart request directly to a file block. Do not read files into memory (ioutil.ReadAll is forbidden).
- Downloads: Use http.ServeFile to leverage the OS's sendfile system call for efficient disk-to-network transfer.
- File Size: No hard limit in code; constrained only by available disk space and max_upload_size in the proxy (Caddy).

### Media & Metadata Extraction

- Images: On upload, use Go's image.DecodeConfig to extract dimensions without loading the full pixel buffer.
- Videos: Execute ffprobe to extract duration and resolution. Generate a .jpg thumbnail using ffmpeg.
- Code/Text: Check file extensions or sniffing MIME types. Use Prism.js (loaded via CDN only when viewing a code file) for syntax highlighting.

### The UI (PWA)

- Layout: A single-column "Chat" stream.
- Components:
    - Text Bubble: Supports multi-line strings, markdown-lite (optional).
    - Image Bubble: Shows thumbnail; clicking opens a native HTML <dialog> with the full-res image.
    - Video Bubble: Native <video> tag with preload="metadata".
    - File Bubble: Icon + Filename + Size + Download Button.

## API Endpoints for AI Agent to Build

- GET /: Main UI (SSR via Go html/template).
- GET /events: SSE stream to notify the client to trigger an HTMX swap when a new item is added.
- POST /upload: Handles multipart form for files/images/videos.
- POST /message: Handles simple text strings.
- GET /files/{path}: Serves raw files.
- GET /history: Returns a filtered gallery view of all media items.

## Performance Constraints for the Agent

- No Heavy Frameworks: Do not use React, Vue, or Angular.
- Manual Memory Management: Ensure all Body.Close() and rows.Close() are handled.
- Streaming I/O: All file operations must use io.Reader and io.Writer interfaces.
- Assets: Minimize external JS. Use one minified bundle for HTMX and Alpine.js.
