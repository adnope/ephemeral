# ---------------------------------------------------------------
# Stage 1: Build - Full Go toolchain
# ---------------------------------------------------------------
FROM golang:1.21-alpine AS builder

WORKDIR /src

# Dependency layer - cached unless go.mod/go.sum change
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build
COPY . .

# CGO_ENABLED=0: required for modernc.org/sqlite (pure Go)
# -trimpath: remove local build paths from binary
# -ldflags: strip debug symbols (-s) and DWARF (-w)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /bin/leandrop \
    ./cmd/leandrop

# ---------------------------------------------------------------
# Stage 2: Runtime - Minimal image with ffmpeg
# ---------------------------------------------------------------
FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Non-root user for security
RUN useradd -r -u 1001 -s /sbin/nologin leandrop
USER leandrop

WORKDIR /app

COPY --from=builder /bin/leandrop     /app/leandrop
COPY --from=builder /src/web          /app/web
COPY --from=builder /src/migrations   /app/migrations

RUN mkdir -p /app/data/uploads/thumbs

EXPOSE 8080

ENTRYPOINT ["/app/leandrop"]
