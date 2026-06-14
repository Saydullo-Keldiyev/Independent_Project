# API Gateway — Central Nervous System

Single entry point: `https://api.auction.com` → routes to microservices.

## Flow

```
Client → Gateway (JWT, rate limit, headers) → User / Auction / Bid service
```

## Injected headers (downstream trust)

| Header | Purpose |
|--------|---------|
| `X-User-ID` | Authenticated user |
| `X-User-Role` | RBAC role |
| `X-Correlation-ID` | Distributed tracing |
| `X-Gateway-Trust` | `true` when validated at gateway |

## Features

- Central JWT validation + Redis blacklist
- Redis rate limit: **100 req/min** per IP (public) and per user (authenticated)
- Circuit breaker + retry (exponential backoff) per upstream
- WebSocket proxy `/api/v1/ws/:auction_id` → bid-service
- Prometheus metrics at `/metrics`
- Security headers (HSTS in production)

## Routes

| Path | Service |
|------|---------|
| `/api/v1/auth/*` | user-service |
| `/api/v1/users/*`, `/wallet/*` | user-service |
| `/api/v1/auctions/*`, `/seller/auctions` | auction-service |
| `/api/v1/bids/*`, `/api/v1/auctions/:id/bids` | bid-service |
| `/api/v1/ws/:auction_id` | bid-service (WebSocket) |
| `/api/v1/admin/*` | auction-service (admin) |

## Run

```bash
cp .env.example .env
go run ./cmd
```

Image: `ghcr.io/Saydullo-Keldiyev/api-gateway:sha-<commit>`
