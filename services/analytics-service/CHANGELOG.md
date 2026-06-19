# Changelog — Analytics Service

All notable changes to the Analytics Service are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.0] - 2024-01-15

### Added
- Admin dashboard with platform-wide KPIs (revenue, bids, active users)
- Revenue metrics with date range filtering (defaults to last 30 days)
- Seller-specific analytics (total auctions, revenue, avg bids)
- Trending leaderboards (top sellers, bidders, categories)
- Per-auction analytics (total bids, unique bidders, price increase)
- Kafka event consumption for real-time metrics aggregation
- Redis caching for frequently accessed metrics
- Structured JSON logging with correlation ID propagation
- Prometheus metrics and health endpoints
- OpenAPI 3.0 specification (docs/openapi.yaml)
