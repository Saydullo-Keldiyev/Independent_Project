# ── Stage 1: Dependency cache ─────────────────────────────────────────────────
# Separate layer so deps are only re-downloaded when go.mod/go.sum change
FROM golang:1.22-alpine AS deps
WORKDIR /app
COPY services/bid-service/go.mod services/bid-service/go.sum ./
RUN go mod download && go mod verify

# ── Stage 2: Build ────────────────────────────────────────────────────────────
FROM deps AS builder
# Build args for reproducible builds and image labeling
ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION=dev

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
# distroless: no shell, no package manager, minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# OCI image labels (standard)
LABEL org.opencontainers.image.title="bid-service" \
      org.opencontainers.image.description="Auction System — Bid Service" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}" \
      org.opencontainers.image.source="https://github.com/Saydullo-Keldiyev/auction-system"

WORKDIR /app

# Copy only the binary — nothing else
COPY --from=builder /bin/bid-service .

# nonroot user (uid 65532) — already set by distroless:nonroot
USER nonroot:nonroot

EXPOSE 8082

ENTRYPOINT ["/app/bid-service"]
