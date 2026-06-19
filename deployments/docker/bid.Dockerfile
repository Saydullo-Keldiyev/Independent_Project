# ── Stage 1: Dependency cache ─────────────────────────────────────────────────
# Separate layer so deps are only re-downloaded when go.mod/go.sum change
FROM golang:1.22-alpine@sha256:4ffd2a0e0bd4661501f75173d855a65335aa2f8841aa8531fd95f83195c804cf AS deps
WORKDIR /app
COPY services/bid-service/go.mod services/bid-service/go.sum ./
RUN go mod download && go mod verify

# ── Stage 2: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine@sha256:4ffd2a0e0bd4661501f75173d855a65335aa2f8841aa8531fd95f83195c804cf AS builder
# Build args for reproducible builds and image labeling
ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION=dev

WORKDIR /app
COPY --from=deps /go/pkg/mod /go/pkg/mod
COPY --from=deps /app/go.mod /app/go.sum ./
COPY services/bid-service/ .

# CGO_ENABLED=0  → fully static binary (no libc dependency)
# -ldflags "-w -s" → strip debug info (smaller binary)
# -trimpath       → remove local paths from binary (security)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -trimpath \
    -o /bin/bid-service \
    ./cmd/main.go

# ── Stage 3: Security scan layer ─────────────────────────────────────────────
# Run govulncheck at build time — fail fast on known vulnerabilities
FROM builder AS scanner
RUN go install golang.org/x/vuln/cmd/govulncheck@latest && \
    govulncheck ./...

# ── Stage 4: Minimal runtime image ───────────────────────────────────────────
FROM alpine:3.19@sha256:12ce69bb53ae000d3cdc69a9651c45d0e72b71f753ac632b1226fe18a56475ec AS runtime

# OCI image labels (standard)
LABEL org.opencontainers.image.title="bid-service" \
      org.opencontainers.image.description="Auction System — Bid Service" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/Saydullo-Keldiyev/auction-system"

RUN apk --no-cache add ca-certificates curl && \
    addgroup -g 10001 -S appgroup && \
    adduser -u 10001 -S appuser -G appgroup -H -D

WORKDIR /app

# Copy only the binary — nothing else
COPY --from=builder /bin/bid-service .

# Non-root user with UID in 10000-65534 range
USER 10001

EXPOSE 8082

HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD curl -f http://localhost:8082/health || exit 1

ENTRYPOINT ["/app/bid-service"]
