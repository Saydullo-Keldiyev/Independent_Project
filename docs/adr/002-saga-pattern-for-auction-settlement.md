# ADR-002: Saga Pattern for Auction Settlement

## Date

2024-01-15

## Status

Accepted

## Context

When an auction closes, the system must perform multiple coordinated operations across the Payment Service:
1. Charge the winner's held balance
2. Credit the seller's available balance
3. Release all losing bidders' holds
4. Publish settlement events

These operations span multiple database rows and involve Kafka event publication. A single database transaction cannot guarantee atomicity across all steps (especially Kafka publication). We need a pattern that provides eventual consistency with guaranteed completion or full rollback.

Options considered:
- **Two-phase commit (2PC):** Rejected — introduces a single point of failure (coordinator), high latency, and doesn't integrate cleanly with Kafka.
- **Choreography-based saga:** Rejected — event-driven choreography makes it difficult to reason about the settlement order and adds complexity for compensation.
- **Orchestration-based saga:** Selected — provides explicit ordering, centralized compensation logic, and clear auditability.

## Decision

We will implement an orchestration-based Saga for auction settlement within the Payment Service. The saga orchestrator:
- Persists saga state and step status in PostgreSQL (`sagas` and `saga_steps` tables)
- Executes steps in defined order with a 30-second timeout per step
- On step failure or timeout, compensates in reverse order within 60 seconds
- Uses the Outbox Pattern for event publication to guarantee atomicity with DB writes

## Consequences

### Positive

- Settlement is fully auditable via saga state tables
- Compensation logic is centralized and testable
- Step ordering is explicit and easy to reason about
- Saga state survives pod restarts (persisted in PostgreSQL)

### Negative

- Additional database tables and queries for saga state management
- Compensation logic must be carefully designed to be idempotent
- Saga adds latency compared to a single-transaction approach (multiple DB operations)

### Neutral

- Saga orchestrator is implemented within the Payment Service (not a separate service)
- Outbox pattern publisher runs as a background goroutine in the same process
- Monitoring provides visibility into saga step durations and failure rates
