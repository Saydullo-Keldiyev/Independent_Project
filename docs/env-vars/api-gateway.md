# Environment Variables: API Gateway

## Server Configuration

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `PORT` | HTTP server listening port | `8080` | No | Integer 1-65535 |
| `ENV` | Deployment environment | `development` | Yes | One of: development, staging, production |
| `SERVICE_NAME` | Service identifier for logs/metrics | `api-gateway` | No | Non-empty string |
| `SERVICE_VERSION` | Application version (semver) | - | Yes | Semver format (e.g., 1.2.3) |
| `SHUTDOWN_TIMEOUT` | Graceful shutdown drain duration | `30s` | No | Go duration (1s-60s) |

## Authentication & Security

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `JWT_PUBLIC_KEY_PATH` | Path to JWT verification public key (from Vault) | - | Yes | Valid file path |
| `JWT_ROTATION_GRACE_PERIOD` | Grace period for previous JWT signing key | `15m` | No | Go duration (1m-60m) |
| `SERVICE_TOKEN_SECRET` | Shared secret for service-to-service auth (from Vault) | - | Yes | Non-empty, min 32 chars |
| `SERVICE_TOKEN_EXPIRY` | Service token maximum lifetime | `24h` | No | Go duration (1h-48h) |
| `SERVICE_TOKEN_ROTATION_DAYS` | Days between credential rotation | `7` | No | Integer 1-30 |

## Rate Limiting

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `RATE_LIMIT_AUTH_PER_MIN` | Requests/min for authenticated users | `200` | No | Integer 1-10000 |
| `RATE_LIMIT_ANON_PER_MIN` | Requests/min for anonymous users | `30` | No | Integer 1-10000 |
| `RATE_LIMIT_LOGIN_PER_MIN` | Login endpoint rate limit | `5` | No | Integer 1-100 |
| `RATE_LIMIT_REGISTER_PER_MIN` | Register endpoint rate limit | `3` | No | Integer 1-100 |
| `RATE_LIMIT_BID_PER_MIN` | Bid placement rate limit | `30` | No | Integer 1-1000 |
| `IP_BLOCK_THRESHOLD` | Violations before IP block | `10` | No | Integer 1-100 |
| `IP_BLOCK_DURATION` | Duration of IP block | `15m` | No | Go duration (1m-24h) |

## Redis (Rate Limiting & Cache)

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `REDIS_URL` | Redis connection URL (from Vault) | - | Yes | Valid Redis URL |
| `REDIS_PASSWORD` | Redis authentication password (from Vault) | - | Yes in prod | Non-empty string |
| `REDIS_MAX_CONNECTIONS` | Max connections in pool | `10` | No | Integer 3-100 |
| `REDIS_MIN_CONNECTIONS` | Min idle connections in pool | `3` | No | Integer 1-50 |
| `REDIS_IDLE_TIMEOUT` | Connection idle timeout | `5m` | No | Go duration (1m-30m) |

## Upstream Services

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `BID_SERVICE_URL` | Bid service base URL | `http://bid-service:8081` | Yes | Valid HTTP(S) URL |
| `USER_SERVICE_URL` | User service base URL | `http://user-service:8082` | Yes | Valid HTTP(S) URL |
| `AUCTION_SERVICE_URL` | Auction service base URL | `http://auction-service:8083` | Yes | Valid HTTP(S) URL |
| `NOTIFICATION_SERVICE_URL` | Notification service base URL | `http://notification-service:8084` | Yes | Valid HTTP(S) URL |
| `PAYMENT_SERVICE_URL` | Payment service base URL | `http://payment-service:8085` | Yes | Valid HTTP(S) URL |
| `SEARCH_SERVICE_URL` | Search service base URL | `http://search-service:8086` | Yes | Valid HTTP(S) URL |
| `UPSTREAM_TIMEOUT` | Timeout for upstream HTTP calls | `30s` | No | Go duration (5s-120s) |

## Circuit Breaker

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `CB_FAILURE_THRESHOLD` | Failures before circuit opens | `5` | No | Integer 1-100 |
| `CB_RESET_TIMEOUT` | Timeout before half-open probe | `30s` | No | Go duration (1s-300s) |
| `CB_SUCCESS_THRESHOLD` | Successes needed to close circuit | `1` | No | Integer 1-10 |
| `CB_CACHE_TTL` | Cached response max age in open state | `60s` | No | Go duration (10s-300s) |

## API Versioning

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `API_VERSIONS` | Comma-separated supported versions | `v1,v2` | No | CSV of version strings |
| `API_DEPRECATED_VERSIONS` | Deprecated versions with sunset dates | - | No | Format: `v1:2024-12-01` |

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
| `VAULT_ROLE` | Vault Kubernetes auth role | `api-gateway` | No | Non-empty string |
| `VAULT_SECRET_PATH` | Path to service secrets in Vault | `secret/data/auction/api-gateway` | No | Valid Vault path |
| `VAULT_RETRY_COUNT` | Retries on Vault unavailability | `5` | No | Integer 1-10 |
| `VAULT_RETRY_MAX_DURATION` | Max total retry duration | `60s` | No | Go duration (10s-120s) |

## Memory Pressure Protection

| Variable | Description | Default | Required | Validation |
|----------|-------------|---------|----------|------------|
| `MEMORY_PRESSURE_THRESHOLD` | Memory % to start rejecting requests | `85` | No | Integer 50-95 |
| `MEMORY_RECOVERY_THRESHOLD` | Memory % to resume accepting requests | `80` | No | Integer 50-90 |
