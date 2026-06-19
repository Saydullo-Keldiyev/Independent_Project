# Changelog — Bid Service

All notable changes to the Bid Service are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.0] - 2024-01-15

### Added
- Bid placement with distributed lock (fencing tokens)
- Wallet fund hold integration (atomic with bid creation)
- Bid history per user and per auction
- Full bid timeline endpoint
- Minimum valid bid calculation
- WebSocket real-time bid updates (max 10000 connections/pod, 5-min idle timeout)
- Pagination on list endpoints
- Input validation (UUID format, bid amount: positive, max 2 decimals, ≤999,999,999.99)
- Rate limiting: bid placement (30 req/min)
- Kafka event publishing via outbox pattern
- Idempotent event processing
- Structured JSON logging with correlation ID propagation
- Prometheus metrics and health endpoints
- OpenAPI 3.0 specification (docs/openapi.yaml)
