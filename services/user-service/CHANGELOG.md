# Changelog — User Service

All notable changes to the User Service are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.0] - 2024-01-15

### Added
- User registration with email verification
- Login with JWT access/refresh token pair
- Password reset and change flows
- Session management (list, revoke individual, revoke all)
- User profile CRUD (get, update, delete)
- Avatar upload and deletion
- Public profile endpoint
- Rate limiting: register (3 req/min), login (5 req/min)
- Input validation with field-level error details
- Structured JSON logging with correlation ID propagation
- Prometheus metrics and health endpoints
- OpenAPI 3.0 specification (docs/openapi.yaml)
