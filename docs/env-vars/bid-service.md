# Environment Variables: Bid Service

## Server Configuration

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `PORT` | HTTP server listening port | `8081` | No | Integer 1-65535 |
| `ENV` | Deployment environment | `development` | Yes | One of: development, staging, production |
| `SERVICE_NAME` | Service identifier for logs/metrics | `bid-service` | No | Non-empty string |
| `SERVICE_VERSION` | Application version (semver) | - | Yes | Semver format (e.g., 1.2.3) |
| `SHUTDOWN_TIMEOUT` | Graceful shutdown drain duration | `30s` | No | Go duration (1s-60s) |

## PostgreSQL

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `DATABASE_URL` | PostgreSQL connection string (from Vault) | - | Yes | Valid PostgreSQL URL |
| `DB_MAX_CONNECTIONS` | Maximum pool connections | `25` | No | Integer 5-100 |
| `DB_MIN_CONNECTIONS` | Minimum idle connections | `5` | No | Integer 1-50 |
| `DB_IDLE_TIMEOUT` | Connection idle timeout | `5m` | No | Go duration (1m-30m) |

## Redis

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `REDIS_URL` | Redis connection URL (from Vault) | - | Yes | Valid Redis URL |
| `REDIS_PASSWORD` | Redis authentication password (from Vault) | - | Yes in prod | Non-empty string |
| `REDIS_MAX_CONNECTIONS` | Max connections in pool | `10` | No | Integer 3-100 |
| `REDIS_MIN_CONNECTIONS` | Min idle connections in pool | `3` | No | Integer 1-50 |
| `REDIS_IDLE_TIMEOUT` | Connection idle timeout | `5m` | No | Go duration (1m-30m) |

## Kafka

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `KAFKA_BROKERS` | Comma-separated Kafka broker addresses | - | Yes | CSV of host:port |
| `KAFKA_CONSUMER_GROUP` | Consumer group ID | `bid-service` | No | Non-empty string |
| `KAFKA_TOPICS` | Topics to consume | `bid.events` | No | CSV of topic names |
| `KAFKA_DLQ_TOPIC` | Dead Letter Queue topic | `bid.events.dlq` | No | Non-empty string |
| `KAFKA_RETRY_COUNT` | Max retries before DLQ | `3` | No | Integer 1-10 |
| `KAFKA_IDEMPOTENCY_TTL` | Processed event_id TTL in Redis | `168h` | No | Go duration (24h-336h) |
| `KAFKA_FLUSH_TIMEOUT` | Max flush time during shutdown | `10s` | No | Go duration (5s-30s) |

## Distributed Lock

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `LOCK_TTL` | Default lock TTL | `5s` | No | Go duration (1s-60s) |
| `LOCK_RETRY_COUNT` | Lock acquisition retry count | `3` | No | Integer 1-10 |
| `LOCK_RETRY_BASE_DELAY` | Base delay for exponential backoff | `50ms` | No | Go duration (10ms-1s) |
| `LOCK_RETRY_MULTIPLIER` | Backoff multiplier | `2.0` | No | Float 1.5-4.0 |
| `LOCK_EXTEND_THRESHOLD` | TTL % threshold for auto-extension | `0.8` | No | Float 0.5-0.95 |

## WebSocket

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `WS_MAX_CONNECTIONS` | Max concurrent WebSocket connections per pod | `10000` | No | Integer 100-50000 |
| `WS_HEARTBEAT_TIMEOUT` | Max time without heartbeat before close | `5m` | No | Go duration (1m-30m) |
| `WS_CLOSE_TIMEOUT` | Time for client to acknowledge close frame | `10s` | No | Go duration (5s-30s) |

## Input Validation

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `MAX_PAYLOAD_SIZE` | Maximum request body size | `1048576` | No | Integer (bytes), max 10MB |
| `MAX_TEXT_LENGTH` | Default max text field length | `1000` | No | Integer 100-10000 |
| `MAX_BID_AMOUNT` | Maximum allowed bid amount | `999999999.99` | No | Positive decimal |

## Service Authentication

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `SERVICE_TOKEN_SECRET` | Service-to-service auth secret (from Vault) | - | Yes | Non-empty, min 32 chars |
| `PAYMENT_SERVICE_URL` | Payment service URL for fund holds | `http://payment-service:8085` | Yes | Valid HTTP(S) URL |

## Observability

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `LOG_LEVEL` | Minimum log level | `info` | No | One of: debug, info, warn, error |
| `PROMETHEUS_PORT` | Metrics endpoint port | `9090` | No | Integer 1-65535 |
| `OTEL_EXPORTER_ENDPOINT` | OpenTelemetry collector endpoint | - | No | Valid gRPC URL |
| `OTEL_SAMPLING_RATE` | Trace sampling ratio | `0.1` | No | Float 0.0-1.0 |

## Vault Integration

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `VAULT_ADDR` | HashiCorp Vault address | - | Yes in prod | Valid HTTPS URL |
| `VAULT_ROLE` | Vault Kubernetes auth role | `bid-service` | No | Non-empty string |
| `VAULT_SECRET_PATH` | Path to service secrets in Vault | `secret/data/auction/bid-service` | No | Valid Vault path |
| `VAULT_RETRY_COUNT` | Retries on Vault unavailability | `5` | No | Integer 1-10 |
| `VAULT_RETRY_MAX_DURATION` | Max total retry duration | `60s` | No | Go duration (10s-120s) |
