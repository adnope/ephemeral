FROM golang:1.21-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY migrations ./migrations
COPY web ./web

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags="-s -w -buildid=" \
      -o /out/ephemeral \
      ./cmd/ephemeral

FROM alpine:3.20 AS runtime

RUN apk add --no-cache \
      ffmpeg \
      ca-certificates \
    && addgroup -S ephemeral \
    && adduser -S -D -H -h /app -s /sbin/nologin -G ephemeral ephemeral

WORKDIR /app

COPY --from=builder --chown=ephemeral:ephemeral /out/ephemeral /app/ephemeral
COPY --from=builder --chown=ephemeral:ephemeral /src/web /app/web
COPY --from=builder --chown=ephemeral:ephemeral /src/migrations /app/migrations

RUN mkdir -p /app/data/uploads/thumbs \
    && chown -R ephemeral:ephemeral /app/data

USER ephemeral

EXPOSE 8080

ENTRYPOINT ["/app/ephemeral"]
