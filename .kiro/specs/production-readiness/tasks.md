# Implementation Plan: Production Readiness

## Overview

This plan implements production-readiness enhancements for the auction-system microservices. Tasks are organized from foundational shared packages through service-specific enhancements to infrastructure and testing. All code is in Go 1.22 using the existing technology stack (Gin, Zap, Prometheus, Kafka/Sarama, Redis, PostgreSQL/pgx, Kubernetes + Istio).

## Tasks

- [x] 1. Implement shared packages (`pkg/`) — Core libraries
  - [x] 1.1 Create `pkg/lock` — Production-grade distributed lock with fencing tokens
    - Implement `LockConfig`, `Lock`, and `LockManager` interface
    - Use Redis INCR for monotonically increasing fencing tokens
    - Implement ownership verification on release (compare fencing token + owner)
    - Implement retry with exponential backoff (50ms × 2^n, max 3 retries)
    - Implement TTL auto-extension at 80% threshold
    - Log warnings on TTL expiry and failed extensions
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6, 3.7_

  - [ ]* 1.2 Write property tests for distributed lock (Properties 3, 4)
    - **Property 3: Fencing Token Monotonicity** — verify N acquisitions produce strictly increasing tokens
    - **Property 4: Lock Ownership Verification** — verify correct owner can release, incorrect owner is rejected
    - **Validates: Requirements 3.1, 3.2, 3.3**

  - [x] 1.3 Create `pkg/circuitbreaker` — Enhanced circuit breaker with metrics
    - Implement `Config`, `State`, `CircuitBreaker` interface, and `Metrics` struct
    - Implement state machine: closed → open → half-open → closed/open
    - In half-open: allow exactly one probe, reject others with fallback
    - Configurable thresholds: failure (1-100), reset timeout (1s-300s), success threshold (1-10)
    - Emit structured WARN log on state transitions
    - Return cached response (≤60s) when open, else graceful degradation response within 50ms
    - Count failures on HTTP 5xx, connection timeout, or connection refused
    - Expose Prometheus metrics: state gauge, consecutive failures gauge, seconds since failure gauge
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5, 4.6, 4.7, 4.8_

  - [ ]* 1.4 Write property test for circuit breaker (Property 5)
    - **Property 5: Circuit Breaker State Machine** — verify state transitions, probe behavior, and failure counting
    - **Validates: Requirements 4.2, 4.3, 4.4, 4.8**

  - [x] 1.5 Create `pkg/logger` — Structured logging with deduplication and redaction
    - Implement `LogConfig` and `Logger` interface wrapping Zap
    - JSON output in production, include required fields: service_name, environment, version, timestamp (ISO 8601), level, correlation_id, trace_id
    - Implement log deduplication: suppress after 10 entries of same (level, message) within 60s window, emit summary on window close
    - Implement secret redaction: replace any 4+ char substring of configured secrets with "[REDACTED]"
    - Implement `WithCorrelationID` for context propagation
    - _Requirements: 5.1, 5.5, 5.6, 2.5_

  - [ ]* 1.6 Write property tests for logger (Properties 2, 7, 8)
    - **Property 2: Secret Redaction Completeness** — verify no 4+ char secret substring survives redaction
    - **Property 7: Structured Log Completeness** — verify all required fields present in JSON output
    - **Property 8: Log Deduplication** — verify suppression after 10 entries within 60s window
    - **Validates: Requirements 2.5, 5.5, 5.6**

  - [x] 1.7 Create `pkg/kafka` — Idempotent producer/consumer with DLQ
    - Implement `ProducerConfig`, `ConsumerConfig`, `Message`, `Consumer` interface, `MessageHandler`
    - Assign UUID v4 event_id to every message at production time
    - Check event_id against Redis processed-events store (TTL: 7 days) before processing
    - Skip duplicate events, acknowledge, and increment `duplicate_events_total` Prometheus counter
    - Retry failed processing up to 3 times with exponential backoff (1s, 2s, 4s), then move to DLQ
    - Expose Prometheus metrics: consumer lag, processing duration histogram, DLQ size, retry count
    - If processed-events store unavailable, process anyway + log warning
    - Propagate X-Correlation-ID as Kafka header
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5, 8.7, 5.2_

  - [ ]* 1.8 Write property test for Kafka idempotency (Property 9)
    - **Property 9: Kafka Message Idempotency** — verify duplicate event_id causes skip with no state change
    - **Validates: Requirements 8.1, 8.2, 8.3**

  - [x] 1.9 Create `pkg/ratelimit` — Sliding window rate limiter with IP blocking
    - Implement `RateLimitConfig`, `EndpointLimit`, `RateLimiter` interface
    - Implement sliding window algorithm: current_count + (previous_count × remaining_fraction)
    - Separate limits: authenticated (200/min), anonymous (30/min)
    - Endpoint-specific limits: login (5/min), register (3/min), bid placement (30/min)
    - IP blocking: block for 15 minutes after 10 violations within 1 hour
    - Enforce most restrictive limit when global and endpoint-specific both apply
    - Fallback to in-memory per-instance limiting when Redis unavailable
    - Return Retry-After header on HTTP 429
    - _Requirements: 9.1, 9.2, 9.3, 9.4, 9.5, 9.6, 9.7_

  - [ ]* 1.10 Write property tests for rate limiter (Properties 10, 11, 12)
    - **Property 10: Rate Limit Enforcement** — verify effective limit is min(global, endpoint-specific)
    - **Property 11: Sliding Window Accuracy** — verify weighted calculation across window boundaries
    - **Property 12: IP Blocking After Violations** — verify blocking after 10 violations for 15 minutes
    - **Validates: Requirements 9.1, 9.2, 9.4, 9.5, 9.6**

  - [x] 1.11 Create `pkg/validation` — Input validation and sanitization
    - Implement `Validator` interface, `FieldError` struct
    - Validate struct fields against DTO struct tags
    - Reject payloads >1MB with HTTP 413
    - Sanitize HTML special characters (& < > " ')
    - Validate UUID v4 format for entity IDs
    - Enforce max 1000 chars on free-text fields (unless field-specific limit defined)
    - Validate query parameters and path parameters in addition to body
    - Bid amount validation: positive, max 2 decimal places, ≤999,999,999.99
    - _Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7, 11.8_

  - [ ]* 1.12 Write property tests for validation (Properties 13, 14, 15, 16)
    - **Property 13: Input Validation Correctness** — verify DTO constraint violations produce HTTP 400 with field details
    - **Property 14: HTML Sanitization** — verify round-trip: unescape(sanitize(input)) == input
    - **Property 15: Size and Length Bounds** — verify >1MB → 413, >1000 chars → rejected
    - **Property 16: Format and Range Validation** — verify UUID v4 format and bid amount constraints
    - **Validates: Requirements 11.1, 11.2, 11.3, 11.5, 11.6, 11.7, 11.8**

- [x] 2. Checkpoint — Shared packages complete
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 3. API Gateway enhancements
  - [x] 3.1 Integrate sliding window rate limit middleware into API Gateway
    - Wire `pkg/ratelimit` into Gin middleware chain
    - Apply authenticated vs anonymous limits based on JWT presence
    - Apply endpoint-specific limits for /login, /register, /bids
    - Return HTTP 429 with Retry-After header
    - Return HTTP 403 for blocked IPs with reason and remaining duration
    - Log warning when falling back to in-memory limiting
    - _Requirements: 9.1, 9.2, 9.3, 9.5, 9.7_

  - [x] 3.2 Implement API versioning router in API Gateway
    - Implement `VersionRouter` with support for 2-3 concurrent versions (/api/v1, /api/v2)
    - Route requests based on URL path version prefix
    - Return HTTP 404 with supported versions list for unrecognized versions
    - Add Deprecation header (RFC 7231 date format) for deprecated versions for 90+ days
    - Return HTTP 410 after sunset date with current versions list
    - _Requirements: 15.1, 15.2, 15.3, 15.4_

  - [ ]* 3.3 Write property test for version routing (Property 17)
    - **Property 17: Version Routing Correctness** — verify supported versions route correctly, unsupported return 404
    - **Validates: Requirements 15.1, 15.2**

  - [x] 3.4 Enhance correlation ID middleware
    - Generate UUID v4 when X-Correlation-ID header is absent
    - Preserve and forward existing X-Correlation-ID without modification
    - Propagate to all downstream service calls and Kafka headers
    - _Requirements: 5.2, 5.3, 5.4_

  - [ ]* 3.5 Write property test for correlation ID handling (Property 6)
    - **Property 6: Correlation ID Handling** — verify generation when absent, preservation when present
    - **Validates: Requirements 5.3, 5.4**

  - [x] 3.6 Implement service-to-service authentication middleware
    - Authenticate all inter-service HTTP calls using signed service tokens (expiration ≤24h)
    - Return HTTP 403 on auth failure, log attempt with source IP, service identity, timestamp
    - Reject all requests when auth subsystem unavailable (fail-closed)
    - Auto-rotate credentials every 7 days with 24h overlap period
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.7_

  - [x] 3.7 Implement memory pressure protection middleware
    - Monitor pod memory usage via runtime metrics
    - Return HTTP 503 + Retry-After when memory >85% of pod limit
    - Resume accepting requests when memory drops below 80%
    - _Requirements: 16.5_

- [ ] 4. Payment Service — Saga orchestrator and wallet operations
  - [x] 4.1 Implement wallet transaction with SELECT FOR UPDATE locking
    - Execute all balance changes within PostgreSQL transaction
    - Use SELECT FOR UPDATE on wallet row for row-level locking
    - Implement fund hold: atomic within single transaction, reject bid on failure
    - Enforce idempotency via idempotency_key check before processing
    - Log all financial state transitions with full audit fields
    - _Requirements: 18.1, 18.2, 18.5, 18.6_

  - [ ]* 4.2 Write property tests for wallet operations (Properties 18, 20, 21)
    - **Property 18: Wallet Operation Atomicity** — verify hold + bid persist together or neither
    - **Property 20: Financial Audit Log Completeness** — verify all required fields logged, balance_after = balance_before ± amount
    - **Property 21: Wallet Operation Idempotency** — verify N executions with same key produce exactly one change
    - **Validates: Requirements 18.1, 18.2, 18.5, 18.6**

  - [x] 4.3 Implement Saga orchestrator for auction settlement
    - Create `SagaStep`, `Saga`, and `SagaOrchestrator` interfaces
    - Implement settlement saga: charge winner → credit seller → release loser holds → publish events via outbox
    - On step failure: compensate in reverse order within 60 seconds
    - On step timeout (30s): mark failed and initiate compensation
    - Persist saga state and step status in `sagas` and `saga_steps` tables
    - _Requirements: 18.3, 18.4, 18.7_

  - [ ]* 4.4 Write property test for saga execution (Property 19)
    - **Property 19: Saga Execution Correctness** — verify ordered execution and reverse compensation on failure
    - **Validates: Requirements 18.3, 18.4**

  - [x] 4.5 Implement Outbox pattern for Payment Service
    - Create `outbox_events` table with migration
    - Write outbox entries within same DB transaction as financial state changes
    - Implement outbox publisher: poll unpublished entries and publish to Kafka
    - Handle retry_count and last_error tracking
    - _Requirements: 8.6_

- [~] 5. Checkpoint — Core service logic complete
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 6. Graceful shutdown and resource management
  - [x] 6.1 Implement graceful shutdown handler for all services
    - Stop accepting new connections on SIGTERM
    - Wait up to 30s for in-flight HTTP requests to complete
    - Send WebSocket close frames to all connected clients (Bid Service), allow 10s for acknowledgment
    - Flush pending Kafka messages within 10s, log unflushed count if timeout
    - Close database connection pools
    - _Requirements: 10.1, 10.2, 10.3, 10.4_

  - [x] 6.2 Implement connection pool management and resource limits
    - Configure PostgreSQL pool: max 25, min 5, idle timeout 5 min per instance
    - Configure Redis pool: max 10, min 3, idle timeout 5 min per instance
    - Set 30s timeout for all outbound HTTP calls between services
    - Limit WebSocket connections to 10000 per Bid Service pod
    - Terminate WebSocket connections with no heartbeat for 5 minutes
    - Expose pool utilization and WebSocket count as Prometheus metrics
    - Log warning when pool has no available connections for >5s
    - _Requirements: 16.1, 16.2, 16.3, 16.4, 16.6, 16.7_

  - [ ]* 6.3 Write unit tests for graceful shutdown and resource management
    - Test SIGTERM handling with in-flight requests
    - Test WebSocket close frame sending and timeout
    - Test Kafka flush with timeout exceeded scenario
    - Test connection pool limit enforcement
    - _Requirements: 10.1, 10.2, 10.3, 10.4, 16.1, 16.2_

- [ ] 7. Secrets management and JWT key rotation
  - [x] 7.1 Implement Vault/SealedSecrets integration for secret retrieval
    - Retrieve all credentials from Vault/SealedSecrets at runtime
    - Retry up to 5 times with exponential backoff (max 60s total) if Vault unavailable at startup
    - Fail pod startup with error indication if secrets cannot be retrieved
    - Use separate Vault paths/namespaces per environment (dev, staging, prod)
    - _Requirements: 2.1, 2.4, 2.6_

  - [x] 7.2 Implement JWT signing key rotation with grace period
    - Rotate JWT signing keys every 24 hours
    - Accept tokens signed with previous key for up to 15 minutes after rotation
    - Ensure zero failed authentication requests during rotation window
    - _Requirements: 2.2_

  - [ ]* 7.3 Write property test for JWT key rotation (Property 1)
    - **Property 1: JWT Key Rotation Grace Period** — verify previous key accepted within 15 min, rejected after
    - **Validates: Requirements 2.2**

- [~] 8. Checkpoint — Application layer complete
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 9. Dockerfile hardening and CI security
  - [x] 9.1 Harden all Dockerfiles with pinned digests, non-root user, and healthchecks
    - Pin all base images with SHA256 digest in every FROM instruction (including multi-stage)
    - Run final stage as non-root user with UID in 10000–65534 range
    - Add HEALTHCHECK: interval ≤30s, timeout ≤5s, start-period ≤60s, retries 3-5
    - _Requirements: 1.1, 1.2, 1.3_

  - [x] 9.2 Configure CI pipeline for image security scanning
    - Fail build on CRITICAL/HIGH CVE from Trivy scan
    - Fail build if base image digest is unavailable in upstream registry
    - Integrate gitleaks for secret detection: block merge + notify security team within 5 min
    - _Requirements: 1.4, 1.5, 2.3_

- [ ] 10. Database migrations and backup strategy
  - [x] 10.1 Implement database migration framework with rollback support
    - Run migrations as Kubernetes Job completing within 300s before app pods start
    - Every migration file must have explicit reversible "down" operation
    - On failure/timeout: halt deployment, execute rollback within 120s, report failure with file name
    - Create `schema_migrations` tracking table
    - _Requirements: 7.1, 7.2, 7.5_

  - [x] 10.2 Create database schema migrations for new tables
    - Create `outbox_events` table with indexes
    - Create `wallet_transactions` table with indexes
    - Create `sagas` and `saga_steps` tables
    - Create `schema_migrations` table
    - All migrations must include forward and rollback scripts
    - _Requirements: 7.2_

  - [x] 10.3 Implement automated backup and restore verification
    - Configure PostgreSQL backup every 6 hours via pg_dump + WAL archiving
    - Enable point-in-time recovery with WAL retention ≥7 days
    - Implement weekly automated restore test: restore to temp DB, validate tables and row counts (within 1% variance)
    - Retain backups 30-90 days, auto-delete beyond max retention
    - Alert DBA on backup timeout (>60 min), retry once within 15 min
    - _Requirements: 7.3, 7.4, 7.6, 7.7_

- [ ] 11. Kubernetes infrastructure — HA, security, and deployment
  - [~] 11.1 Create Kubernetes manifests for PDB, HPA, and pod anti-affinity
    - PodDisruptionBudget: minAvailable=1 per service
    - HPA: min 2 replicas, scale on CPU (70%) and memory (75%)
    - Pod anti-affinity: prevent >50% of replicas in single zone
    - preStop hook with sleep ≥5s for load balancer deregistration
    - _Requirements: 10.5, 10.6, 17.1, 17.7_

  - [~] 11.2 Implement NetworkPolicies and pod security
    - Default-deny ingress and egress for auction-system namespace
    - Explicit allow rules for declared service dependencies only
    - Pod Security Standards "restricted" level on namespace
    - runAsNonRoot: true, drop all capabilities, allowPrivilegeEscalation: false
    - readOnlyRootFilesystem: true, writable only emptyDir for /tmp
    - _Requirements: 20.1, 20.2, 20.3, 20.4, 20.5, 20.7_

  - [x] 11.3 Configure Istio mTLS and AuthorizationPolicy
    - PeerAuthentication in STRICT mode for auction-system namespace
    - Default-deny AuthorizationPolicy with explicit service-to-service paths
    - _Requirements: 12.5, 12.6_

  - [x] 11.4 Configure PostgreSQL HA and Redis cluster for disaster recovery
    - PostgreSQL synchronous replica in different AZ, auto-promotion within 30s
    - Redis cluster: 3+ master nodes across AZs, failover within 15s
    - Kafka: replication factor 3, min.insync.replicas 2, across 2+ AZs
    - _Requirements: 17.2, 17.3, 17.6_

- [ ] 12. Monitoring, alerting, and dashboards
  - [~] 12.1 Create Prometheus alerting rules
    - HighErrorRate: >5% error rate over 5 min → critical alert within 60s
    - HighLatency: P99 >2s over 5 min → warning alert within 60s
    - KafkaConsumerLag: >10000 messages for 5 min → critical alert within 60s
    - PodRestartLoop: >3 restarts in 10 min → warning alert within 60s
    - Include: service name, severity, threshold, current value, timestamp, runbook link
    - Configure multi-channel delivery (Slack + PagerDuty) within 30s
    - Retry delivery 3 times at 10s intervals, use remaining channel if one unreachable
    - Group repeated alerts into single notification per 5-min window
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 6.8_

  - [~] 12.2 Create Grafana dashboards as code
    - Service-level dashboard: request rate, error rate, latency (P50/P95/P99), resource saturation
    - Infrastructure dashboard: CPU, memory, disk, network per pod (1h default, 30s refresh)
    - Kafka dashboard: consumer lag, throughput/topic, partition count, DLQ count
    - Business metrics: active auctions, bids/min, transaction volume, WebSocket connections
    - Security dashboard: rate limit hits, auth failures, blocked IPs (15 min sliding window)
    - Auto-provision via ConfigMaps/provisioning directory
    - Display "No Data" within 60s when data source unreachable
    - Minimum 15 days data retention for all queries
    - _Requirements: 14.1, 14.2, 14.3, 14.4, 14.5, 14.6, 14.7, 14.8_

- [~] 13. Checkpoint — Infrastructure and monitoring complete
  - Ensure all tests pass, ask the user if questions arise.

- [ ] 14. Documentation and API specifications
  - [~] 14.1 Create/update OpenAPI 3.0 specifications for all services
    - Maintain OpenAPI 3.0 spec per service
    - Add CI validation: fail build on undocumented endpoints or schema mismatches
    - Document API versioning and deprecation in specs
    - Maintain CHANGELOG.md per service for additions, deprecations, and breaking changes
    - _Requirements: 19.1, 19.6, 15.5, 15.6_

  - [~] 14.2 Create operational runbooks and documentation structure
    - Runbook per alert: description, symptoms, 3+ diagnosis steps, resolution, escalation
    - Environment variables documentation per service (descriptions, defaults, validation rules)
    - ADR directory with consistent template (title, date, status, context, decision, consequences)
    - Top-level documentation index with all resources and relative paths
    - Architecture diagram in version-controlled format
    - _Requirements: 19.2, 19.3, 19.4, 19.5, 19.7_

- [ ] 15. Integration and wiring — Wire all components together
  - [x] 15.1 Integrate shared packages into all services
    - Wire `pkg/logger` into all services replacing existing logger calls
    - Wire `pkg/lock` into Bid Service for auction lock acquisition
    - Wire `pkg/circuitbreaker` into all inter-service HTTP clients
    - Wire `pkg/kafka` into all Kafka producers and consumers
    - Wire `pkg/validation` into all HTTP handlers
    - Ensure structured logging with correlation ID propagation across all services
    - _Requirements: 3.1-3.7, 4.1-4.8, 5.1-5.6, 8.1-8.7, 11.1-11.8_

  - [x] 15.2 Configure CI pipeline with full quality gates
    - golangci-lint + Dockerfile lint + OPA policy checks
    - Unit test with coverage gate (fail <80% per service)
    - Property-based tests (rapid, 100+ iterations)
    - Security scan (trivy for CVEs, gitleaks for secrets)
    - Weekly vulnerability scan for running images: alert security team within 60 min on CRITICAL findings
    - _Requirements: 13.5, 1.4, 2.3, 20.6_

  - [ ]* 15.3 Write integration tests for inter-service communication
    - Test service-to-service auth token validation
    - Test circuit breaker behavior under failure conditions
    - Test rate limiting under concurrent load
    - Test Kafka event flow with idempotency
    - Use testcontainers-go for PostgreSQL, Redis, Kafka
    - Timeout: 120s per test suite
    - _Requirements: 13.2_

  - [ ]* 15.4 Write contract tests between API Gateway and backend services
    - Consumer-driven contract tests for: bid-service, user-service, auction-service, payment-service, notification-service, search-service
    - Verify request path routing, header forwarding, response schema compatibility
    - Use Pact for contract verification
    - _Requirements: 13.6_

  - [ ]* 15.5 Write end-to-end test for full auction lifecycle
    - Register users → create auction → place bids → verify bid in auction → verify wallet hold → close auction → settle payment
    - Timeout: 300s per test
    - _Requirements: 13.3_

  - [ ]* 15.6 Write chaos tests for dependency failure scenarios
    - Test Kafka unavailability: verify no HTTP 5xx, outbox pattern persists locally
    - Test Redis unavailability: verify rate limiter fallback and degraded idempotency
    - Test PostgreSQL unavailability: verify graceful degradation
    - Verify recovery within 60s with zero data loss
    - _Requirements: 13.4_

- [~] 16. Final checkpoint — Ensure all tests pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Tasks marked with `*` are optional and can be skipped for faster MVP
- Each task references specific requirements for traceability
- Checkpoints ensure incremental validation between major phases
- Property tests validate the 21 universal correctness properties defined in the design document
- Unit tests validate specific examples and edge cases
- All code is Go 1.22 using existing stack: Gin, Zap, Prometheus, Kafka/Sarama, Redis/go-redis, PostgreSQL/pgx
- Infrastructure tasks produce Kubernetes YAML manifests, Prometheus rules, and Grafana dashboard JSON
- The Outbox pattern implementation (task 4.5) is critical for financial data consistency
- Disaster recovery configuration (task 11.4) requires coordination with infrastructure team for AZ setup

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "1.5", "1.9", "1.11"] },
    { "id": 1, "tasks": ["1.2", "1.3", "1.6", "1.7", "1.10", "1.12"] },
    { "id": 2, "tasks": ["1.4", "1.8", "3.4", "7.1", "9.1"] },
    { "id": 3, "tasks": ["3.1", "3.2", "3.5", "3.6", "3.7", "7.2"] },
    { "id": 4, "tasks": ["3.3", "7.3", "4.1", "4.5", "6.1", "6.2"] },
    { "id": 5, "tasks": ["4.2", "4.3", "6.3", "9.2", "10.1"] },
    { "id": 6, "tasks": ["4.4", "10.2", "10.3", "11.1", "11.2"] },
    { "id": 7, "tasks": ["11.3", "11.4", "12.1", "12.2"] },
    { "id": 8, "tasks": ["14.1", "14.2", "15.1"] },
    { "id": 9, "tasks": ["15.2", "15.3", "15.4"] },
    { "id": 10, "tasks": ["15.5", "15.6"] }
  ]
}
```
