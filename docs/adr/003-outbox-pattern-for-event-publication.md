# ADR-003: Outbox Pattern for Event Publication

## Date

2024-01-15

## Status

Accepted

## Context

Financial operations in the Payment Service require atomicity between database state changes and Kafka event publication. Without a transactional guarantee, the system can enter inconsistent states:
- **Lost events:** DB commits but Kafka publish fails — downstream services never learn about the state change.
- **Ghost events:** Kafka publish succeeds but DB transaction rolls back — downstream services process events for non-existent state.

The system processes critical financial events (fund holds, charges, credits) where data loss or duplication has direct monetary impact.

## Decision

We will implement the Outbox Pattern for all financial state changes in the Payment Service:
1. Write outbox entries within the same PostgreSQL transaction as the state change
2. A background publisher polls the `outbox_events` table for unpublished entries
3. Published entries are marked with `published_at` timestamp
4. Failed publications track `retry_count` and `last_error` for observability

The outbox table includes an `idempotency_key` column (UNIQUE constraint) to prevent duplicate entries for the same logical operation.

## Consequences

### Positive

- Atomic guarantee: events are published if and only if the state change persists
- Resilient to Kafka outages — events queue in PostgreSQL until publishable
- Idempotency key prevents duplicate outbox entries from retried operations
- Full auditability via outbox table (who, what, when)

### Negative

- Adds latency between state change and event delivery (polling interval)
- Outbox table grows and requires periodic cleanup of old published entries
- Additional database load from publisher polling queries

### Neutral

- Publisher runs as a goroutine within the Payment Service process
- Poll interval defaults to 1 second (configurable)
- Consumer-side idempotency (via `pkg/kafka`) provides defense-in-depth
