# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine@sha256:4ffd2a0e0bd4661501f75173d855a65335aa2f8841aa8531fd95f83195c804cf AS builder

WORKDIR /app

# Install dependencies first (layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o bid-service ./cmd/main.go

# ── Stage 2: Minimal runtime image ───────────────────────────────────────────
FROM alpine:3.19@sha256:12ce69bb53ae000d3cdc69a9651c45d0e72b71f753ac632b1226fe18a56475ec

RUN apk --no-cache add ca-certificates tzdata curl && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -H -D

WORKDIR /app

COPY --from=builder /app/bid-service .

USER 10001

EXPOSE 8082

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8082/health || exit 1

CMD ["./bid-service"]
