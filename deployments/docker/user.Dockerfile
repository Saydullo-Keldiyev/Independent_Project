FROM golang:1.22-alpine AS deps
WORKDIR /app
COPY services/user-service/go.mod services/user-service/go.sum ./
RUN go mod download && go mod verify

FROM deps AS builder
ARG BUILD_DATE
ARG GIT_COMMIT
ARG VERSION=dev
COPY services/user-service/ .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build \
    -ldflags="-w -s -X main.version=${VERSION} -X main.commit=${GIT_COMMIT}" \
    -trimpath \
    -o /bin/user-service \
    ./cmd/main.go

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
LABEL org.opencontainers.image.title="user-service"
WORKDIR /app
COPY --from=builder /bin/user-service .
USER nonroot:nonroot
EXPOSE 8081
ENTRYPOINT ["/app/user-service"]
