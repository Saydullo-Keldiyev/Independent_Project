# Changelog — Notification Service

All notable changes to the Notification Service are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.0] - 2024-01-15

### Added
- Notification history with pagination and unread filter
- Mark notification as read (individual and bulk)
- Real-time delivery via WebSocket connections
- Kafka event consumption (bid_placed, bid_outbid, auction_won, auction_ended, payment_received)
- Notification types: bid_placed, bid_outbid, auction_won, auction_ended, payment_received, system
- Unread count in list response
- Idempotent event processing (duplicate detection via event_id)
- Structured JSON logging with correlation ID propagation
- Prometheus metrics and health endpoints
- OpenAPI 3.0 specification (docs/openapi.yaml)
