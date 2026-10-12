# syntax=docker/dockerfile:1

# ── build stage ───────────────────────────────────────────────────────────────
# Must satisfy the `go` directive in go.mod (currently 1.26).
FROM golang:1.26-alpine AS build

# Dependencies are their own layer so source edits do not re-download modules.
WORKDIR /src

COPY backend-go/go.mod backend-go/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY backend-go/ ./
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -o /out/api ./cmd/api

# ── runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.21

# wget backs the compose healthcheck; ca-certificates backs outbound TLS.
# Run as a non-root user: a container that is never root limits the blast radius
# of an escape.
RUN apk add --no-cache ca-certificates wget \
 && addgroup -S archon \
 && adduser -S -G archon -H -s /sbin/nologin archon

COPY --from=build /out/api /usr/local/bin/api

WORKDIR /workspace/backend-go
USER archon

EXPOSE 3001

HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=5 \
  CMD wget -qO- http://127.0.0.1:3001/health || exit 1

ENTRYPOINT ["/usr/local/bin/api"]