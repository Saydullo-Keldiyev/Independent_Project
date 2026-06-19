# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY api-gateway/go.mod api-gateway/go.sum ./api-gateway/
COPY pkg/ ./pkg/
WORKDIR /app/api-gateway
ENV GOTOOLCHAIN=auto
RUN go mod download
COPY api-gateway/ /app/api-gateway/
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /bin/service ./cmd/main.go

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -H -D
WORKDIR /app
COPY --from=builder /bin/service .
COPY api-gateway/docs/ ./docs/

USER 10001

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

CMD ["./service"]
