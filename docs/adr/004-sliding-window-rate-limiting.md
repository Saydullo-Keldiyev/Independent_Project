# ADR-004: Sliding Window Rate Limiting

## Date

2024-01-15

## Status

Accepted

## Context

The API Gateway requires rate limiting to protect backend services from overload and mitigate DDoS attacks. Common algorithms include:
- **Fixed window:** Simple but allows burst at window boundaries (2x effective rate).
- **Token bucket:** Smooth but complex to implement in distributed form with Redis.
- **Sliding window log:** Precise but memory-intensive (stores every request timestamp).
- **Sliding window counter:** Good balance of accuracy and efficiency.

The system needs:
- Separate limits for authenticated (200/min) and anonymous (30/min) users
- Endpoint-specific limits (login: 5/min, register: 3/min, bid: 30/min)
- IP blocking after repeated violations (10 violations → 15 min block)
- Graceful fallback when Redis is unavailable

## Decision

We will implement sliding window counter rate limiting using Redis sorted sets:
- Formula: `effective_count = current_count + (previous_count × remaining_fraction)`
- Two Redis keys per window per identifier (current + previous)
- When multiple limits apply (global + endpoint), enforce the most restrictive
- Fallback to in-memory per-instance limiting when Redis is unavailable
- IP violation tracking in Redis with 1-hour TTL, blocking for 15 minutes after 10 violations

## Consequences

### Positive

- Smooths out boundary bursts compared to fixed window
- Low memory usage (two counters per window vs. per-request timestamps)
- Redis-based implementation shares state across all Gateway pods
- Fallback prevents complete service denial during Redis outages

### Negative

- Sliding window counter is an approximation (not exact like sliding log)
- Per-instance fallback allows total rate to exceed global limit during Redis outage
- IP blocking state is lost if Redis is flushed

### Neutral

- Rate limit responses include Retry-After header for client backoff
- Blocked IPs receive HTTP 403 with reason and remaining duration
- Metrics exposed: rate_limit_hits_total, ip_blocks_total
