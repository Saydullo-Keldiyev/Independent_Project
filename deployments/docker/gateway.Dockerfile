# ── Stage 1: Dependency cache ─────────────────────────────────────────────────
FROM golang:1.22-alpine AS deps
WORKDIR /app
COPY api-gateway/go.mod api-gateway/go.sum ./
RUN go mod download && go mod verify

# ── Stage 2: Build ────────────────────────────────────────────────────────────
FROM deps AS builder
ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION=dev

COPY api-gateway/ .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${GIT_COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -trimpath \
    -o /bin/api-gateway \
    ./cmd/main.go

# ── Stage 3: Runtime ──────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

LABEL org.opencontainers.image.title="api-gateway" \
      org.opencontainers.image.description="Auction System — API Gateway" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${GIT_COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"

WORKDIR /app
COPY --from=builder /bin/api-gateway .
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/app/api-gateway"]
