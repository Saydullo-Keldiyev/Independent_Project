# Environment Variables: Notification Service

## Server Configuration

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `PORT` | HTTP server listening port | `8084` | No | Integer 1-65535 |
| `ENV` | Deployment environment | `development` | Yes | One of: development, staging, production |
| `SERVICE_NAME` | Service identifier for logs/metrics | `notification-service` | No | Non-empty string |
| `SERVICE_VERSION` | Application version (semver) | - | Yes | Semver format (e.g., 1.2.3) |
| `SHUTDOWN_TIMEOUT` | Graceful shutdown drain duration | `30s` | No | Go duration (1s-60s) |

## Kafka

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `KAFKA_BROKERS` | Comma-separated Kafka broker addresses | - | Yes | CSV of host:port |
| `KAFKA_CONSUMER_GROUP` | Consumer group ID | `notification-service` | No | Non-empty string |
| `KAFKA_TOPICS` | Topics to consume | `bid.events,auction.events,payment.events` | No | CSV of topic names |
| `KAFKA_DLQ_TOPIC` | Dead Letter Queue topic | `notification.events.dlq` | No | Non-empty string |
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

## Email (SMTP)

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `SMTP_HOST` | SMTP server hostname | - | Yes | Valid hostname |
| `SMTP_PORT` | SMTP server port | `587` | No | Integer (25, 465, 587) |
| `SMTP_USERNAME` | SMTP authentication username (from Vault) | - | Yes | Non-empty string |
| `SMTP_PASSWORD` | SMTP authentication password (from Vault) | - | Yes | Non-empty string |
| `SMTP_FROM` | Default sender email address | - | Yes | Valid email address |
| `SMTP_TLS_ENABLED` | Enable TLS for SMTP | `true` | No | Boolean |

## WebSocket

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `WS_MAX_CONNECTIONS` | Max concurrent WebSocket connections per pod | `10000` | No | Integer 100-50000 |
| `WS_HEARTBEAT_TIMEOUT` | Max time without heartbeat before close | `5m` | No | Go duration (1m-30m) |

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
| `VAULT_ROLE` | Vault Kubernetes auth role | `notification-service` | No | Non-empty string |
| `VAULT_SECRET_PATH` | Path to service secrets in Vault | `secret/data/auction/notification-service` | No | Valid Vault path |
| `VAULT_RETRY_COUNT` | Retries on Vault unavailability | `5` | No | Integer 1-10 |
| `VAULT_RETRY_MAX_DURATION` | Max total retry duration | `60s` | No | Go duration (10s-120s) |
