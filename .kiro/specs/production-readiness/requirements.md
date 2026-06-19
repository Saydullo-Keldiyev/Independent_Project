# Requirements Document

## Introduction

Ushbu hujjat auction-system microservices loyihasini production-readiness darajasiga olib chiqish uchun zarur bo'lgan barcha talablarni belgilaydi. Loyihada aniqlangan kamchiliklar va bo'shliqlar asosida xavfsizlik mustahkamligi, kuzatuvchanlik (observability), xatoliklarni boshqarish, ishlash samaradorligi, test qamrovi, deployment yetuklik darajasi va ma'lumotlar izchilligi bo'yicha talablar keltirilgan.

## Glossary

- **System**: Butun auction-system microservices tizimi (API Gateway, barcha servislar, infratuzilma)
- **API_Gateway**: Barcha tashqi so'rovlarni qabul qiluvchi va ichki servislarga yo'naltiruvcji gateway servisi
- **Bid_Service**: Bid (taklif) qo'yish va boshqarish logikasini amalga oshiruvchi servis
- **Auction_Service**: Auksionlarni yaratish, boshqarish va holat o'tkazish logikasini amalga oshiruvchi servis
- **User_Service**: Foydalanuvchi autentifikatsiyasi, avtorizatsiyasi va profil boshqaruvini amalga oshiruvchi servis
- **Notification_Service**: Xabarnomalarni real-vaqt va email orqali yetkazish servisi
- **Payment_Service**: To'lov va wallet boshqaruvini amalga oshiruvchi servis
- **Distributed_Lock**: Redis asosidagi taqsimlangan qulf mexanizmi
- **Circuit_Breaker**: Servislararo aloqada nosozlikni izolyatsiya qilish mexanizmi
- **DLQ**: Dead Letter Queue — qayta ishlanmagan xabarlar navbati
- **Outbox_Pattern**: Ma'lumotlar bazasi va event yetkazilishini atomik ravishda ta'minlash patterni
- **HPA**: Horizontal Pod Autoscaler — Kubernetes'da avtomatik masshtablash
- **PDB**: Pod Disruption Budget — servis mavjudligini ta'minlash
- **CI_Pipeline**: GitHub Actions asosidagi Continuous Integration pipeline
- **Monitoring_System**: Prometheus + Grafana + Alertmanager asosidagi monitoring tizimi

## Requirements

### Requirement 1: Dockerfile xavfsizligini mustahkamlash

**User Story:** As a DevOps engineer, I want all Dockerfiles to use pinned base image digests and non-root users, so that container images are reproducible and attack surface is minimized.

#### Acceptance Criteria

1. THE System SHALL use base images pinned with a SHA256 digest (e.g., `golang:1.22-alpine@sha256:<digest>`) in every FROM instruction across all Dockerfiles, including intermediate multi-stage build stages
2. THE System SHALL run the final runtime stage of all application containers as a non-root user with a dedicated UID in the range 10000–65534, created explicitly via a USER instruction in the Dockerfile
3. THE System SHALL include a HEALTHCHECK instruction in the final stage of each Dockerfile with an interval no greater than 30 seconds, a timeout no greater than 5 seconds, a start period no greater than 60 seconds, and a retries value between 3 and 5
4. IF a Dockerfile build produces a CVE of CRITICAL or HIGH severity during the CI image scan, THEN THE CI_Pipeline SHALL fail the build with a non-zero exit code and prevent the image from being promoted to any deployment environment
5. IF a base image digest referenced in a Dockerfile is no longer available in the upstream registry, THEN THE CI_Pipeline SHALL fail the build and report an error message indicating the unavailable digest and affected Dockerfile path

### Requirement 2: Secrets boshqaruvini ishlab chiqarish darajasiga olib chiqish

**User Story:** As a security engineer, I want secrets to be managed via HashiCorp Vault or Sealed Secrets exclusively, so that plaintext secrets never exist in the repository or environment variables.

#### Acceptance Criteria

1. THE System SHALL retrieve all sensitive credentials (database URLs, JWT signing keys, API keys, SMTP passwords, and Redis passwords) from HashiCorp Vault or Kubernetes SealedSecrets at runtime, with no plaintext secret values present in environment variables, ConfigMaps, or container images
2. THE System SHALL rotate JWT signing keys every 24 hours, accepting tokens signed with the previous key for a grace period of up to 15 minutes after rotation, with zero failed authentication requests during the rotation window
3. IF a secret is detected in a Git commit, THEN THE CI_Pipeline SHALL block the merge and send a notification to the security team within 5 minutes of detection
4. THE System SHALL use separate secret stores (distinct Kubernetes namespaces and distinct Vault paths or distinct SealedSecret resources) for development, staging, and production environments, with no cross-environment secret access permitted
5. THE System SHALL replace all secret values in application logs with a fixed redaction placeholder (e.g., "[REDACTED]"), ensuring that no substring of 4 or more consecutive characters from any secret value appears in log output
6. IF HashiCorp Vault or the SealedSecrets controller is unavailable when a pod starts, THEN THE System SHALL retry the secret retrieval up to 5 times with exponential backoff over a maximum of 60 seconds, and fail to start with an error indication if secrets cannot be retrieved

### Requirement 3: Distributed Lock mexanizmini ishonchli qilish

**User Story:** As a backend developer, I want the distributed lock to use fencing tokens and proper ownership verification, so that stale locks cannot corrupt auction state.

#### Acceptance Criteria

1. WHEN acquiring a lock, THE Distributed_Lock SHALL generate a monotonically increasing fencing token and associate it with the lock owner identifier
2. WHEN releasing a lock, THE Distributed_Lock SHALL verify the fencing token matches the current owner before deletion
3. IF the fencing token does not match the current owner during release, THEN THE Distributed_Lock SHALL reject the release operation, retain the existing lock, and return an error indicating ownership mismatch
4. IF a lock TTL expires before release, THEN THE Distributed_Lock SHALL log a warning with lock key, owner, and elapsed time
5. THE Distributed_Lock SHALL retry acquisition up to 3 times with exponential backoff starting at 50 milliseconds with a multiplier of 2 (50ms, 100ms, 200ms) before returning failure
6. WHEN a lock is held beyond 80% of its TTL, THE Distributed_Lock SHALL attempt to extend the TTL by the original TTL duration
7. IF TTL extension fails, THEN THE Distributed_Lock SHALL log a warning with lock key and remaining TTL, and leave the lock to expire naturally without interrupting the in-progress operation

### Requirement 4: Circuit Breaker ni to'liq amalga oshirish

**User Story:** As a platform engineer, I want the circuit breaker to track detailed failure metrics and support configurable thresholds, so that cascading failures are prevented across the service mesh.

#### Acceptance Criteria

1. THE Circuit_Breaker SHALL expose per-service Prometheus metrics: a gauge for current state (0=closed, 1=open, 2=half-open), a gauge for consecutive failure count, and a gauge for seconds since last failure, labeled by service name
2. WHILE the circuit is in half-open state, THE Circuit_Breaker SHALL allow exactly one probe request per reset timeout period and reject all other requests with a fallback response until the probe completes
3. IF a probe request succeeds while the circuit is in half-open state, THEN THE Circuit_Breaker SHALL transition to closed state only after receiving a configurable number of consecutive successful responses (minimum 1, maximum 10, default 1)
4. IF a probe request fails while the circuit is in half-open state, THEN THE Circuit_Breaker SHALL transition back to open state and restart the reset timeout
5. WHEN the circuit transitions between states, THE Circuit_Breaker SHALL emit a structured log event at WARN level containing: service name, previous state, new state, current failure count, and timestamp
6. THE Circuit_Breaker SHALL support per-service configurable parameters: failure threshold (1 to 100, default 5), reset timeout (1 second to 300 seconds, default 30 seconds), and success threshold for half-open-to-closed transition (1 to 10, default 1)
7. IF the circuit opens, THEN THE Circuit_Breaker SHALL return a cached response if one exists with age no greater than 60 seconds, otherwise return a graceful degradation response indicating service unavailability, within 50ms of receiving the request
8. THE Circuit_Breaker SHALL count a request as a failure when the upstream service returns an HTTP 5xx status code, a connection timeout occurs, or a connection is refused

### Requirement 5: Barcha servislar uchun yagona structured logging standartini joriy etish

**User Story:** As an SRE, I want all services to use a consistent structured logging format with correlation IDs, so that distributed request tracing is possible across the entire system.

#### Acceptance Criteria

1. WHILE the environment is production, THE System SHALL output all log entries in JSON format for every service
2. THE System SHALL propagate the X-Correlation-ID header in all inter-service HTTP requests and as a Kafka message header named "X-Correlation-ID" in all produced messages
3. IF an incoming request to THE API_Gateway does not contain an X-Correlation-ID header, THEN THE API_Gateway SHALL generate a correlation_id using a UUID v4 value and attach it to the request before forwarding
4. WHEN an incoming request to THE API_Gateway already contains an X-Correlation-ID header, THE API_Gateway SHALL preserve and forward the existing value without modification
5. THE System SHALL include the following fields in every log entry: service_name, environment, version, timestamp (ISO 8601 format), level, correlation_id, and trace_id
6. WHILE a service has emitted 10 or more log entries with the same log-level and message template combination within a 60-second window, THE System SHALL suppress further duplicate entries and emit a single summary entry indicating the count of suppressed messages when the window expires

### Requirement 6: Alerting tizimini sozlash

**User Story:** As an SRE, I want production alerting rules with multi-channel notification, so that incidents are detected and escalated within defined SLOs.

#### Acceptance Criteria

1. WHEN a service error rate exceeds 5% over a 5-minute window, THE Monitoring_System SHALL fire a critical alert within 60 seconds of threshold breach
2. WHEN a service P99 latency exceeds 2 seconds over a 5-minute window, THE Monitoring_System SHALL fire a warning alert within 60 seconds of threshold breach
3. WHEN a Kafka consumer lag exceeds 10000 messages for 5 minutes, THE Monitoring_System SHALL fire a critical alert within 60 seconds of threshold breach
4. THE Monitoring_System SHALL deliver alert notifications to at least two channels (Slack and PagerDuty) within 30 seconds of alert firing
5. WHEN a pod restart count exceeds 3 within 10 minutes, THE Monitoring_System SHALL fire a warning alert within 60 seconds of threshold breach
6. THE Monitoring_System SHALL include service name, alert severity, threshold value, current value, timestamp, and a runbook link in every alert notification
7. IF a notification channel is unreachable, THEN THE Monitoring_System SHALL retry delivery up to 3 times with 10-second intervals and deliver via the remaining available channel
8. THE Monitoring_System SHALL group repeated occurrences of the same alert into a single notification per 5-minute window to prevent notification flooding

### Requirement 7: Database migration va backup strategiyasini ishonchli qilish

**User Story:** As a DBA, I want automated database migrations with rollback capability and verified backups, so that data integrity is maintained during deployments and disaster recovery is possible.

#### Acceptance Criteria

1. WHEN a new application version is deployed, THE System SHALL run database migrations as a separate Kubernetes Job that must complete successfully within 300 seconds before application pods are started
2. THE System SHALL support both forward and rollback migrations for every schema change, where each migration file contains an explicit reversible "down" operation that restores the previous schema state
3. THE System SHALL execute automated PostgreSQL backups every 6 hours using pg_dump with point-in-time recovery enabled via WAL archiving, retaining WAL segments for a minimum of 7 days
4. THE System SHALL verify backup integrity by performing an automated restore test weekly, where success is defined as a completed restore to a temporary database instance and validation that all tables and row counts match the source within 1% variance
5. IF a migration Job fails or exceeds 300 seconds, THEN THE CI_Pipeline SHALL halt the deployment, execute the corresponding rollback migration within 120 seconds, and report the failure status with the migration file name that caused the error
6. THE System SHALL retain database backups for a minimum of 30 days and a maximum of 90 days, automatically deleting backups older than the maximum retention period
7. IF a backup Job fails to complete within 60 minutes, THEN THE System SHALL send an alert notification to the configured DBA channel and retry the backup once within 15 minutes

### Requirement 8: Kafka event ishonchliligi va idempotencyni ta'minlash

**User Story:** As a backend developer, I want Kafka consumers to be idempotent with exactly-once processing semantics, so that duplicate events do not corrupt system state.

#### Acceptance Criteria

1. THE System SHALL assign a unique event_id in UUID v4 format to every Kafka message at the point of production (before publishing to the broker)
2. WHEN a consumer receives a message, THE System SHALL check the event_id against a Redis-based processed-events store before processing, where each entry has a TTL of 7 days
3. IF a duplicate event_id is detected, THEN THE System SHALL skip processing, acknowledge the message, and increment a `duplicate_events_total` Prometheus counter
4. THE System SHALL retry failed message processing up to 3 times with exponential backoff (1s, 2s, 4s) before moving the message to a Dead Letter Queue
5. THE System SHALL emit Prometheus metrics for consumer lag (gauge per consumer group), processing duration (histogram per topic), DLQ size (gauge per topic), and retry count (counter per topic)
6. THE Payment_Service SHALL use the Outbox Pattern for all financial state changes to guarantee atomicity between DB writes and event publication
7. IF the processed-events store is unavailable, THEN THE System SHALL proceed with processing the message and log a warning indicating degraded idempotency protection

### Requirement 9: API rate limiting va DDoS himoyasini kuchaytirish

**User Story:** As a security engineer, I want tiered rate limiting with IP reputation scoring, so that legitimate users are not impacted while malicious traffic is blocked.

#### Acceptance Criteria

1. THE API_Gateway SHALL enforce separate rate limits for authenticated users (200 req/min) and anonymous users (30 req/min)
2. THE API_Gateway SHALL enforce endpoint-specific rate limits for sensitive operations: login (5 req/min), register (3 req/min), bid placement (30 req/min)
3. WHEN a client exceeds the rate limit, THE API_Gateway SHALL return HTTP 429 with a Retry-After header indicating the number of seconds until the rate limit window resets
4. THE API_Gateway SHALL implement sliding window rate limiting algorithm where the current window's count is combined with a weighted fraction of the previous window's count
5. IF a single IP exceeds rate limits 10 times within 1 hour, THEN THE API_Gateway SHALL temporarily block that IP for 15 minutes, returning HTTP 403 with a message indicating the block reason and remaining duration
6. WHEN endpoint-specific and global rate limits both apply to a request, THE API_Gateway SHALL enforce the more restrictive limit
7. IF the Redis rate limit store is unavailable, THEN THE API_Gateway SHALL fall back to in-memory per-instance rate limiting and log a warning indicating degraded mode

### Requirement 10: Graceful shutdown va zero-downtime deployment

**User Story:** As a platform engineer, I want all services to drain connections gracefully during shutdown, so that in-flight requests and WebSocket connections are not dropped during deployments.

#### Acceptance Criteria

1. WHEN a SIGTERM signal is received, THE System SHALL stop accepting new connections and wait up to 30 seconds for in-flight requests to complete before forcefully terminating remaining connections
2. WHEN a SIGTERM signal is received, THE Bid_Service SHALL send a WebSocket close frame to all connected clients and allow up to 10 seconds for clients to acknowledge the close before terminating the connections
3. WHEN a SIGTERM signal is received, THE System SHALL flush all pending Kafka messages within 10 seconds before shutdown
4. IF pending Kafka messages cannot be flushed within 10 seconds, THEN THE System SHALL log the count of unflushed messages and proceed with shutdown
5. THE Kubernetes deployment SHALL use a preStop hook with a sleep of at least 5 seconds to allow load balancer deregistration before the application shutdown sequence begins
6. THE System SHALL configure PodDisruptionBudget to maintain at least 1 available replica for each service during voluntary disruptions

### Requirement 11: Input validation va sanitization standartlari

**User Story:** As a security engineer, I want all user inputs to be validated and sanitized at every service boundary, so that injection attacks and data corruption are prevented.

#### Acceptance Criteria

1. THE System SHALL validate all request body fields against the schema defined by each endpoint's DTO struct tags before processing
2. THE System SHALL reject request payloads exceeding 1MB with HTTP 413 status code
3. THE System SHALL sanitize all string inputs by escaping HTML special characters (&, <, >, ", ') before database storage
4. WHEN validation fails, THE System SHALL return a structured error response with HTTP 400 status code, containing field-level error messages and the invalid field names
5. IF an entity ID path or query parameter does not match UUID v4 format, THEN THE System SHALL return HTTP 400 with an error message indicating the malformed parameter name
6. THE Bid_Service SHALL validate that bid amounts are positive numbers greater than 0, with a maximum of 2 decimal places, and not exceeding 999,999,999.99
7. THE System SHALL enforce a maximum length of 1000 characters on all free-text string input fields unless a field-specific limit is defined in the endpoint schema
8. THE System SHALL validate all query parameters and path parameters in addition to request body fields against their defined type and format constraints

### Requirement 12: Service-to-service autentifikatsiya

**User Story:** As a security engineer, I want mutual TLS or service tokens for inter-service communication, so that internal APIs cannot be accessed by unauthorized parties.

#### Acceptance Criteria

1. THE System SHALL authenticate all inter-service HTTP calls using either mTLS certificates or signed service tokens
2. THE User_Service internal wallet API SHALL require a valid service authentication token with an expiration time of no more than 24 hours on every request
3. WHEN an inter-service authentication fails, THE System SHALL return HTTP 403 and log the attempt with source IP, service identity, and timestamp
4. THE System SHALL rotate service-to-service credentials automatically every 7 days with a minimum overlap period of 24 hours during which both old and new credentials are accepted
5. THE System SHALL implement Istio PeerAuthentication in STRICT mode for the auction-system namespace
6. THE System SHALL enforce a default-deny AuthorizationPolicy for the auction-system namespace, permitting only explicitly declared service-to-service communication paths
7. IF the authentication subsystem is unavailable, THEN THE System SHALL reject all inter-service requests rather than allowing unauthenticated access

### Requirement 13: Test qamrovini oshirish

**User Story:** As a QA engineer, I want comprehensive test coverage including unit, integration, contract, and chaos tests, so that regressions are caught before production deployment.

#### Acceptance Criteria

1. THE System SHALL maintain a minimum of 80% line-level unit test code coverage for all services, measured using `go test -covermode=atomic`
2. THE System SHALL include integration tests that verify inter-service communication by asserting correct response status codes, response body schema conformance, and request timeout handling within 120 seconds per test suite
3. THE System SHALL include end-to-end tests covering the complete auction lifecycle: register users, create auction, place bids, verify bid appears in auction, verify wallet hold, close auction, and settle payment, with each end-to-end test completing within 300 seconds
4. WHEN Kafka, Redis, or PostgreSQL becomes unavailable during a chaos test, THE System SHALL continue accepting write requests without returning HTTP 5xx errors by persisting data locally (outbox pattern), and SHALL process all pending events within 60 seconds after the dependency recovers with zero data loss
5. IF unit test line coverage drops below 80% on any service, THEN THE CI_Pipeline SHALL fail the build and prevent merge to main or develop branches
6. THE System SHALL include consumer-driven contract tests between API Gateway and each of the following backend services: bid-service, user-service, auction-service, payment-service, notification-service, and search-service, verifying request path routing, request header forwarding, and response schema compatibility

### Requirement 14: Monitoring dashboardlarini to'liq sozlash

**User Story:** As an SRE, I want pre-built Grafana dashboards covering all critical system metrics, so that operational health is visible at a glance.

#### Acceptance Criteria

1. THE Monitoring_System SHALL provide a service-level dashboard showing request rate, error rate, latency percentiles (P50, P95, P99), and resource saturation (CPU usage above 80%, memory usage above 80%, or thread pool exhaustion) for each service
2. THE Monitoring_System SHALL provide an infrastructure dashboard showing CPU, memory, disk, and network utilization per pod with a default time range of 1 hour and a minimum panel refresh interval of 30 seconds
3. THE Monitoring_System SHALL provide a Kafka dashboard showing consumer lag per consumer group, message throughput (messages/second per topic), partition count per broker, and DLQ message count per topic
4. THE Monitoring_System SHALL provide a business metrics dashboard showing active auctions count, bids per minute, total transaction volume, and active WebSocket connections
5. THE Monitoring_System SHALL provide a security dashboard showing rate limit hits, authentication failures, and blocked IPs aggregated over a configurable sliding window with a default of 15 minutes
6. THE Monitoring_System SHALL auto-provision all dashboards via Grafana dashboard-as-code (ConfigMaps or provisioning directory) so that dashboards are recreated automatically on deployment without manual import
7. IF a configured data source becomes unreachable, THEN THE Monitoring_System SHALL display a "No Data" indicator on affected panels within 60 seconds rather than showing stale values
8. THE Monitoring_System SHALL support a minimum data retention period of 15 days for all dashboard metric queries

### Requirement 15: API versioning va backward compatibility

**User Story:** As a backend developer, I want API versioning with deprecation policies, so that breaking changes do not impact existing clients.

#### Acceptance Criteria

1. THE API_Gateway SHALL route requests based on URL path version prefix (/api/v1, /api/v2), supporting a minimum of 2 and a maximum of 3 concurrently active versions
2. IF a request targets a version prefix that is not recognized or has been removed, THEN THE API_Gateway SHALL respond with a 404 status and an error message indicating the version is not available, including a list of currently supported versions
3. WHEN an API version is deprecated, THE API_Gateway SHALL include a "Deprecation" response header containing the sunset date in RFC 7231 date format on every response for that version, for a minimum of 90 days before removal
4. IF the sunset date of a deprecated version has passed, THEN THE API_Gateway SHALL reject all requests to that version with a 410 status and an error message indicating the version has been removed, including the currently supported versions
5. THE System SHALL maintain backward compatibility within a major version for at least 6 months, where backward compatibility means no removal of existing endpoints, no removal of existing response fields, no change to existing field data types, and no change to required request parameters
6. WHEN an API endpoint is added, modified, or removed, THE System SHALL document the change in a CHANGELOG.md file for the affected service, including the version number, date, and whether the change is an addition, deprecation, or breaking change

### Requirement 16: Xotira va resurs boshqaruvi

**User Story:** As a platform engineer, I want all services to have bounded resource consumption and proper cleanup, so that memory leaks and resource exhaustion do not cause production incidents.

#### Acceptance Criteria

1. THE System SHALL configure connection pool limits for PostgreSQL (max 25 connections, min 5 connections per service instance) and Redis (max 10 connections, min 3 connections per service instance) with a maximum connection idle time of 5 minutes
2. THE System SHALL implement request timeout of 30 seconds for all outbound HTTP calls between services, and IF a timeout occurs, THEN THE System SHALL cancel the request, release associated resources, and return an error response indicating timeout to the caller
3. THE Bid_Service SHALL limit concurrent WebSocket connections to 10000 per pod instance, and IF the limit is reached, THEN THE Bid_Service SHALL reject new connection attempts with a WebSocket close frame indicating capacity exceeded
4. WHEN a WebSocket connection receives no heartbeat ping/pong exchange for 5 minutes, THE System SHALL send a close frame and terminate the connection
5. WHEN memory usage exceeds 85% of the pod memory limit, THE System SHALL reject new incoming requests with HTTP 503 status code and a Retry-After header until memory usage drops below 80%
6. THE System SHALL expose connection pool utilization (active connections, idle connections, waiting acquires) and WebSocket connection count as Prometheus metrics per service instance
7. IF the connection pool has no available connections for more than 5 seconds, THEN THE System SHALL log a warning with pool name, current active count, and waiting request count

### Requirement 17: Disaster Recovery va High Availability

**User Story:** As a platform engineer, I want multi-zone deployment with automated failover, so that the system survives single zone failures with minimal downtime.

#### Acceptance Criteria

1. THE System SHALL deploy all stateless services across a minimum of 2 availability zones using pod anti-affinity rules that prevent scheduling more than 50% of replicas in a single zone
2. THE System SHALL configure PostgreSQL with at least one synchronous replica in a different availability zone, with automatic promotion completing within 30 seconds of primary failure detection
3. THE System SHALL configure Redis in cluster mode with at least 3 master nodes distributed across availability zones, with automatic failover completing within 15 seconds
4. WHEN a complete availability zone fails, THE System SHALL continue serving requests within 30 seconds of failover with a maximum error rate of 5% during the transition period
5. THE System SHALL document and test disaster recovery procedures quarterly with a target RTO of 15 minutes and RPO of 1 minute, and each DR drill SHALL produce a post-mortem report documenting actual RTO/RPO achieved
6. THE Kafka cluster SHALL run with a minimum replication factor of 3 and min.insync.replicas of 2, distributed across at least 2 availability zones
7. THE System SHALL configure HPA for all stateless services with a minimum of 2 replicas and maximum defined per service, scaling on CPU (target 70%) and memory (target 75%) utilization

### Requirement 18: Wallet operatsiyalari uchun tranzaksion yaxlitlik

**User Story:** As a backend developer, I want wallet operations to be fully ACID-compliant with saga pattern for cross-service transactions, so that financial data is never inconsistent.

#### Acceptance Criteria

1. THE Payment_Service SHALL execute all wallet balance changes within a PostgreSQL database transaction using SELECT FOR UPDATE row-level locking on the wallet row
2. WHEN a bid is placed, THE System SHALL hold funds atomically within a single database transaction — if the hold fails due to insufficient balance or database error, the bid placement SHALL be rejected and no balance change SHALL persist
3. WHEN an auction closes, THE System SHALL settle funds using a Saga with the following ordered steps: (1) charge winner's held balance, (2) credit seller's available balance, (3) release all loser holds, (4) publish settlement events via outbox
4. IF any step of the Saga fails, THEN THE System SHALL execute compensating transactions in reverse order to restore the previous state within 60 seconds
5. THE System SHALL log all financial state transitions with transaction_id, wallet_id, operation_type, amount, balance_before, balance_after, and timestamp for audit purposes
6. THE System SHALL enforce idempotency on all wallet operations by checking the idempotency_key against the transactions table before processing, returning the existing result for duplicate keys
7. IF a Saga step times out (no response within 30 seconds), THEN THE System SHALL mark the saga as failed and initiate compensating transactions

### Requirement 19: Hujjatlashtirish standartlarini yakunlash

**User Story:** As a developer, I want comprehensive documentation including API specs, architecture diagrams, and runbooks, so that new team members can onboard quickly and incidents can be resolved efficiently.

#### Acceptance Criteria

1. THE System SHALL maintain an OpenAPI 3.0 specification for every service, and THE CI_Pipeline SHALL validate the specification against the implemented endpoints on every pull request, failing the build if undocumented endpoints or request/response schema mismatches are detected
2. THE System SHALL include a system architecture diagram in a version-controlled format showing all service dependencies, data flows, and infrastructure components, updated within 5 working days of any service addition or removal
3. THE System SHALL provide an operational runbook for each alert defined in the monitoring system (as specified in Requirement 6), where each runbook contains at minimum: alert description, symptoms, at least 3 diagnosis steps, resolution procedure, and escalation path
4. THE System SHALL document all environment variables with descriptions, defaults, and validation rules per service in a dedicated configuration reference file within each service directory
5. THE System SHALL include an ADR (Architecture Decision Record) directory where each ADR follows a consistent template containing: title, date, status (proposed/accepted/deprecated/superseded), context, decision, and consequences
6. WHEN a new service is added or an existing service API is modified, THE CI_Pipeline SHALL verify that the corresponding OpenAPI specification file exists and passes schema validation
7. THE System SHALL include a top-level documentation index file listing all available documentation resources with descriptions and relative paths

### Requirement 20: Kubernetes Network Policies va Pod Security

**User Story:** As a security engineer, I want network segmentation between services and strict pod security standards, so that lateral movement is limited in case of a compromise.

#### Acceptance Criteria

1. THE System SHALL implement Kubernetes NetworkPolicies that restrict pod-to-pod communication to only the following declared dependencies: api-gateway to bid-service, api-gateway to user-service, api-gateway to auction-service, api-gateway to notification-service, bid-service to auction-service, and auction-service to notification-service
2. THE System SHALL apply a default-deny-all-ingress NetworkPolicy to the auction-system namespace and define explicit ingress allow rules for each service limited to the declared dependency paths in criterion 1
3. THE System SHALL enforce Pod Security Standards at the "restricted" level for the auction-system namespace by applying the pod-security.kubernetes.io/enforce: restricted label to the namespace resource
4. THE System SHALL prevent containers from running as root (runAsNonRoot: true), drop all Linux capabilities, and disallow privilege escalation (allowPrivilegeEscalation: false) for every container in the auction-system namespace
5. THE System SHALL mount all container filesystems as read-only (readOnlyRootFilesystem: true) and limit writable volume mounts to emptyDir volumes for temporary runtime data such as /tmp
6. WHEN the scheduled weekly vulnerability scan completes, THE System SHALL alert the security team within 60 minutes on any CRITICAL-severity findings detected in running container images, and the identified vulnerabilities SHALL be patched or mitigated within 7 calendar days
7. THE System SHALL apply a default-deny-all-egress NetworkPolicy to the auction-system namespace and define explicit egress allow rules limited to declared internal service dependencies and required external endpoints such as container registries and DNS
