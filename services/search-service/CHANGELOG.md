# Changelog — Search Service

All notable changes to the Search Service are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [1.0.0] - 2024-01-15

### Added
- Full-text auction search with Elasticsearch backend
- Filtering by category, state, price range
- Sorting: price_asc, price_desc, newest, ending_soon, most_bids
- Autocomplete suggestions (minimum 2 characters, returns up to 10 results)
- Trending search queries (top 10 by popularity)
- Redis caching for search results
- Kafka event consumption for real-time index updates
- Pagination support (max 100 results per page)
- Structured JSON logging with correlation ID propagation
- Prometheus metrics and health endpoints
- OpenAPI 3.0 specification (docs/openapi.yaml)
