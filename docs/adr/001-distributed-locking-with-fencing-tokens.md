# ADR-001: Distributed Locking with Fencing Tokens

## Date

2024-01-15

## Status

Accepted

## Context

The auction system requires distributed locking to prevent concurrent bid operations from corrupting auction state. Multiple Bid Service pods may attempt to modify the same auction simultaneously, requiring mutual exclusion.

A naive Redis lock (SETNX) is vulnerable to the following failure modes:
- **Stale locks:** A lock holder crashes after acquiring the lock but before releasing it. When the TTL expires, a new holder acquires the lock, but the original holder recovers and attempts to release it.
- **Clock drift:** If the holder's local clock drifts, it may believe the lock is still valid when it has already expired.
- **Split-brain:** Network partitions can lead to multiple processes believing they hold the lock.

We need a mechanism that provides stronger safety guarantees than basic Redis locks while maintaining acceptable performance for bid placement (sub-50ms overhead).

## Decision

We will implement distributed locks with monotonically increasing fencing tokens using Redis INCR on a per-resource counter key. Each lock acquisition increments the counter and associates the new token with the lock. Downstream operations (database writes) validate the fencing token to reject stale operations.

Key design choices:
- Fencing tokens are generated via `INCR lock:fencing_counter:{resource_type}` for strict monotonicity
- Lock ownership is verified on release by comparing both owner and fencing token
- TTL auto-extension occurs at 80% of TTL elapsed to prevent premature expiry
- Retry uses exponential backoff (50ms × 2^n, max 3 retries) to reduce contention

## Consequences

### Positive

- Stale lock operations are rejected by downstream services via fencing token comparison
- Monotonic tokens provide a total ordering of lock acquisitions
- TTL auto-extension reduces false lock expiry under load
- Implementation uses existing Redis infrastructure (no new dependencies)

### Negative

- Fencing token validation must be implemented at every state-changing downstream service
- Redis INCR counter grows indefinitely (mitigated by 64-bit integer space)
- TTL extension adds complexity and an additional Redis call per held lock

### Neutral

- Lock performance overhead is approximately 2-3 Redis RTTs per acquisition
- Lock library is implemented as a shared package (`pkg/lock`) reusable across services
