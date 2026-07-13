FROM node:22-alpine3.23 AS web-builder

WORKDIR /src/web

COPY --link web/package.json web/package-lock.json ./

RUN --mount=type=cache,target=/root/.npm \
    npm ci

COPY --link web ./

RUN npm run build

FROM golang:1.26.3-alpine3.23 AS builder

WORKDIR /src

COPY --link go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY --link cmd ./cmd
COPY --link internal ./internal
COPY --link web ./web
COPY --link --from=web-builder /src/web/dist ./web/dist

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
      -trimpath \
      -ldflags="-s -w -buildid=" \
      -o /out/ephemeral \
      ./cmd/ephemeral

FROM alpine:3.23.4 AS runtime

LABEL org.opencontainers.image.title="Ephemeral" \
      org.opencontainers.image.description="Single-user text and file sharing app" \
      org.opencontainers.image.source="https://github.com/adnope/ephemeral"

RUN apk add --no-cache \
      ffmpeg \
      ca-certificates \
    && addgroup -S -g 10001 ephemeral \
    && adduser -S -D -H -h /app -s /sbin/nologin -G ephemeral -u 10001 ephemeral

WORKDIR /app

COPY --link --from=builder --chown=10001:10001 /out/ephemeral /app/ephemeral

RUN mkdir -p /app/data/uploads/thumbs \
    && chown -R 10001:10001 /app/data

USER 10001:10001

EXPOSE 8080

STOPSIGNAL SIGTERM

ENTRYPOINT ["/app/ephemeral"]
