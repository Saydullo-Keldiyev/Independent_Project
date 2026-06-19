# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY services/notification-service/go.mod services/notification-service/go.sum ./services/notification-service/
COPY pkg/ ./pkg/
WORKDIR /app/services/notification-service
ENV GOTOOLCHAIN=auto
RUN go mod download
COPY services/notification-service/ /app/services/notification-service/
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /bin/service ./cmd/main.go

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -H -D
WORKDIR /app
COPY --from=builder /bin/service .

USER 10001

EXPOSE 8084

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8084/health || exit 1

CMD ["./service"]
