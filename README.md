# Ephemeral

Ephemeral is a lightweight self-hosted web app for quickly sharing text messages and files across devices.

Features:

- Chat-style feed
- Image previews
- Video thumbnails and playback
- Generic file view/download
- History page with filters
- Pagination
- Live updates via SSE
- SQLite data persistence
- Docker deployment
- Hot-reload development

## Tech Stack

- Go
- SQLite
- Go HTML templates
- Alpine.js
- HTMX
- FFmpeg
- Docker / Docker Compose

## Requirements (Development Only)

```bash
go >= 1.21
node >= 20
npm
ffmpeg
air
```

## Configuration

Environment variables:

```env
PORT=8080
DATA_DIR=./data
SESSION_SECRET=change_me
```

Create local env file:

```bash
cp .env.example .env
```

## Docker Deployment

Run with Docker Compose:

```bash
docker compose up -d --build
```

## Development

Install web dependencies:

```bash
make install-web
```

Run with hot reload:

```bash
make dev
```

Go to:

```text
http://localhost:8080
```

### Build and Run Locally

Build:

```bash
make build
```

Run:

```bash
make run
```

Clean binary:

```bash
make clean
```

Delete local app data:

```bash
make clean-data
```

### Format and Lint

Format:

```bash
make format
```

Lint:

```bash
make lint
```

Auto-fix:

```bash
make fix
```
