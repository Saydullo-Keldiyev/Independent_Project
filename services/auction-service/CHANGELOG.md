# Changelog — Auction Service

All notable changes to the Auction Service are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.0] - 2024-01-15

### Added
- Auction CRUD operations (create, read, update, delete)
- Auction lifecycle state machine (draft → scheduled → active → ending → ended → archived)
- Publish and cancel operations
- Image management (add, delete per auction)
- Category listing
- Seller auction listing with state filter
- Watchlist management (add, remove, list)
- Pagination support on list endpoints
- Input validation (UUID format, price range, string lengths)
- Structured JSON logging with correlation ID propagation
- Prometheus metrics and health endpoints
- OpenAPI 3.0 specification (docs/openapi.yaml)
