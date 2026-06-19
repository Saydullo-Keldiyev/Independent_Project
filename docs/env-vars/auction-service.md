# Environment Variables: Auction Service

## Server Configuration

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `PORT` | HTTP server listening port | `8083` | No | Integer 1-65535 |
| `ENV` | Deployment environment | `development` | Yes | One of: development, staging, production |
| `SERVICE_NAME` | Service identifier for logs/metrics | `auction-service` | No | Non-empty string |
| `SERVICE_VERSION` | Application version (semver) | - | Yes | Semver format (e.g., 1.2.3) |
| `SHUTDOWN_TIMEOUT` | Graceful shutdown drain duration | `30s` | No | Go duration (1s-60s) |

## PostgreSQL

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `DATABASE_URL` | PostgreSQL connection string (from Vault) | - | Yes | Valid PostgreSQL URL |
| `DB_MAX_CONNECTIONS` | Maximum pool connections | `25` | No | Integer 5-100 |
| `DB_MIN_CONNECTIONS` | Minimum idle connections | `5` | No | Integer 1-50 |
| `DB_IDLE_TIMEOUT` | Connection idle timeout | `5m` | No | Go duration (1m-30m) |

## Kafka

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `KAFKA_BROKERS` | Comma-separated Kafka broker addresses | - | Yes | CSV of host:port |
| `KAFKA_CONSUMER_GROUP` | Consumer group ID | `auction-service` | No | Non-empty string |
| `KAFKA_TOPICS` | Topics to consume | `auction.events` | No | CSV of topic names |
| `KAFKA_DLQ_TOPIC` | Dead Letter Queue topic | `auction.events.dlq` | No | Non-empty string |
| `KAFKA_RETRY_COUNT` | Max retries before DLQ | `3` | No | Integer 1-10 |
| `KAFKA_IDEMPOTENCY_TTL` | Processed event_id TTL in Redis | `168h` | No | Go duration (24h-336h) |
| `KAFKA_FLUSH_TIMEOUT` | Max flush time during shutdown | `10s` | No | Go duration (5s-30s) |

## Redis

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `REDIS_URL` | Redis connection URL (from Vault) | - | Yes | Valid Redis URL |
| `REDIS_PASSWORD` | Redis authentication password (from Vault) | - | Yes in prod | Non-empty string |
| `REDIS_MAX_CONNECTIONS` | Max connections in pool | `10` | No | Integer 3-100 |
| `REDIS_MIN_CONNECTIONS` | Min idle connections in pool | `3` | No | Integer 1-50 |
| `REDIS_IDLE_TIMEOUT` | Connection idle timeout | `5m` | No | Go duration (1m-30m) |

## Service Authentication

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `SERVICE_TOKEN_SECRET` | Service-to-service auth secret (from Vault) | - | Yes | Non-empty, min 32 chars |

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
| `VAULT_ROLE` | Vault Kubernetes auth role | `auction-service` | No | Non-empty string |
| `VAULT_SECRET_PATH` | Path to service secrets in Vault | `secret/data/auction/auction-service` | No | Valid Vault path |
| `VAULT_RETRY_COUNT` | Retries on Vault unavailability | `5` | No | Integer 1-10 |
| `VAULT_RETRY_MAX_DURATION` | Max total retry duration | `60s` | No | Go duration (10s-120s) |
