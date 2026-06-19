# Changelog — Payment Service

All notable changes to the Payment Service are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.0] - 2024-01-15

### Added
- Wallet balance management (get balance, deposit, withdraw)
- Fund hold/release for bid placement (SELECT FOR UPDATE locking)
- Charge and credit operations for auction settlement
- Saga orchestrator for settlement flow (charge → credit → release → publish)
- Outbox pattern for guaranteed event delivery
- Transaction history with type filtering and pagination
- Idempotency-Key header support on all mutations
- Financial audit logging (balance_before, balance_after, operation_type)
- Service-to-service authentication for internal payment operations
- Input validation (amount: positive, max 2 decimals, ≤999,999,999.99)
- Structured JSON logging with correlation ID propagation
- Prometheus metrics and health endpoints
- OpenAPI 3.0 specification (docs/openapi.yaml)
