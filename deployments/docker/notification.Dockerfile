# ── Stage 1: Dependency cache ─────────────────────────────────────────────────
FROM golang:1.22-alpine@sha256:4ffd2a0e0bd4661501f75173d855a65335aa2f8841aa8531fd95f83195c804cf AS deps
WORKDIR /app
COPY services/notification-service/go.mod services/notification-service/go.sum ./
RUN go mod download && go mod verify

# ── Stage 2: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine@sha256:4ffd2a0e0bd4661501f75173d855a65335aa2f8841aa8531fd95f83195c804cf AS builder
ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION=dev

WORKDIR /app
COPY --from=deps /go/pkg/mod /go/pkg/mod
COPY --from=deps /app/go.mod /app/go.sum ./
COPY services/notification-service/ .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${GIT_COMMIT}" \
    -trimpath \
    -o /bin/notification-service \
    ./cmd/main.go

# ── Stage 3: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19@sha256:12ce69bb53ae000d3cdc69a9651c45d0e72b71f753ac632b1226fe18a56475ec AS runtime

LABEL org.opencontainers.image.title="notification-service"

RUN apk --no-cache add ca-certificates curl && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -H -D

WORKDIR /app
COPY --from=builder /bin/notification-service .

USER 10001

EXPOSE 8084

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8084/health || exit 1

ENTRYPOINT ["/app/notification-service"]
