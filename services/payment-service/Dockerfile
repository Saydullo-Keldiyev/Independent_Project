# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY services/payment-service/go.mod services/payment-service/go.sum ./services/payment-service/
COPY pkg/ ./pkg/
WORKDIR /app/services/payment-service
ENV GOTOOLCHAIN=auto
RUN go mod download
COPY services/payment-service/ /app/services/payment-service/
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /bin/service ./cmd/main.go

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19
RUN apk --no-cache add ca-certificates curl && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -H -D
WORKDIR /app
COPY --from=builder /bin/service .

USER 10001

EXPOSE 8085

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8085/health || exit 1

CMD ["./service"]
