# ADR-005: Circuit Breaker with Cached Fallback

## Date

2024-01-15

## Status

Accepted

## Context

In a microservices architecture, cascading failures occur when one service's downtime causes its callers to accumulate timeouts, exhausting their own resources and failing in turn. The auction system has multiple inter-service communication paths (API Gateway → Bid Service → Payment Service, etc.) where this cascade risk is present.

We need a circuit breaker that:
- Prevents cascading failures by fast-failing when a downstream service is unhealthy
- Provides degraded but functional responses rather than hard errors
- Allows controlled recovery via half-open state probing
- Exposes state for observability and alerting

## Decision

We will implement a circuit breaker library (`pkg/circuitbreaker`) with three-state machine (closed → open → half-open) and cached response fallback:

1. **Closed state:** Normal operation, count consecutive failures
2. **Open state:** Return cached response (if ≤60s old) or graceful degradation response within 50ms
3. **Half-open state:** Allow exactly one probe request, reject all others with fallback

Configuration is per-service with sensible defaults:
- Failure threshold: 5 consecutive failures to open
- Reset timeout: 30 seconds before half-open probe
- Success threshold: 1 successful probe to close

Failures are counted on: HTTP 5xx, connection timeout, connection refused.

## Consequences

### Positive

- Fast failure (≤50ms) when downstream is known-unhealthy, preserving resources
- Cached responses provide partial functionality during outages
- Prometheus metrics enable alerting on circuit state changes
- Configurable per-service thresholds allow tuning for different criticality levels

### Negative

- Cached responses may be stale (up to 60s old)
- Half-open single-probe means slow recovery if probe fails repeatedly
- Circuit breaker adds complexity to error handling in callers

### Neutral

- State transitions logged at WARN level for operational visibility
- Implementation is stateless across pods (each pod has its own circuit breaker instance)
- Library is shared via `pkg/circuitbreaker` across all services
