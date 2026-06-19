# Changelog

All notable changes to the Auction System platform are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2024-01-15

### Added
- API Gateway with rate limiting, circuit breaking, and API versioning (v1)
- User Service: registration, authentication, session management, profile CRUD
- Auction Service: auction lifecycle (create, publish, bid, settle)
- Bid Service: real-time bidding with distributed locks and WebSocket updates
- Payment Service: wallet operations, fund holds, saga-based settlement
- Notification Service: real-time notifications via WebSocket and Kafka events
- Search Service: full-text search with Elasticsearch, autocomplete, trending
- Analytics Service: platform KPIs, revenue metrics, seller dashboards
- Shared packages: distributed lock, circuit breaker, rate limiter, structured logger, Kafka client, input validation
- OpenAPI 3.0 specifications for all services
- Kubernetes deployment manifests with HA configuration
- Prometheus alerting rules and Grafana dashboards
- CI/CD pipeline with security scanning and quality gates

### Security
- Pinned Docker base images with SHA256 digests
- Non-root container execution
- HashiCorp Vault integration for secrets management
- JWT key rotation with 15-minute grace period
- Service-to-service authentication with signed tokens
- Network policies with default-deny
- Pod security standards (restricted level)
- Input validation and HTML sanitization on all endpoints

## Service-Specific Changelogs

Each service maintains its own CHANGELOG.md for granular tracking:

- [API Gateway](./api-gateway/CHANGELOG.md)
- [User Service](./services/user-service/CHANGELOG.md)
- [Auction Service](./services/auction-service/CHANGELOG.md)
- [Bid Service](./services/bid-service/CHANGELOG.md)
- [Payment Service](./services/payment-service/CHANGELOG.md)
- [Notification Service](./services/notification-service/CHANGELOG.md)
- [Search Service](./services/search-service/CHANGELOG.md)
- [Analytics Service](./services/analytics-service/CHANGELOG.md)
