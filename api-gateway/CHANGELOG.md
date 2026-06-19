# Changelog — API Gateway

All notable changes to the API Gateway service are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.0] - 2024-01-15

### Added
- API versioning router supporting /api/v1 (max 3 concurrent versions)
- Sliding window rate limiting (200 req/min authenticated, 30 req/min anonymous)
- Endpoint-specific rate limits: login (5/min), register (3/min), bids (30/min)
- IP blocking after 10 violations within 1 hour (15-minute block duration)
- Correlation ID middleware (generates UUID v4 if absent, forwards if present)
- Service-to-service authentication with signed tokens (24h expiry, 7-day rotation)
- Memory pressure protection (HTTP 503 when pod memory > 85%)
- Circuit breaker integration for downstream service calls
- Prometheus metrics and structured JSON logging
- OpenAPI 3.0 specification (docs/swagger.yaml)
- Deprecation header support (RFC 7231 date format) for deprecated versions
- HTTP 404 with supported versions list for unrecognized version prefixes
- HTTP 410 after sunset date with current versions list

### Security
- JWT validation with key rotation grace period
- RBAC middleware for role-based access control
- Security headers (HSTS, X-Content-Type-Options, X-Frame-Options)
- Request payload size limit (1MB)
