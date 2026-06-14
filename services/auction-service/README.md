# Auction Service — Lifecycle Engine

Manages auction state machine, scheduling, search, and distributed events.

## State machine

```
DRAFT → SCHEDULED → ACTIVE → ENDING → ENDED → ARCHIVED
```

## Features

- Create / update / delete auctions (seller + admin RBAC)
- Scheduler (5s): auto-activate & auto-end with Redis distributed lock
- Winner selection from `bids` table (highest bid, reserve price)
- Kafka: `AUCTION_CREATED`, `AUCTION_STARTED`, `AUCTION_ENDED`, `AUCTION_DELETED`
- Redis cache `auction:{id}` + pub/sub `auction:events` for WebSocket bridge
- PostgreSQL full-text search
- Consumes `bid.placed` events to update `total_bids` / `current_price`

## API (`/api/v1`)

| Method | Path | Auth |
|--------|------|------|
| POST | `/auctions` | Seller/Admin |
| GET | `/auctions/:id` | Public |
| PUT | `/auctions/:id` | Owner |
| DELETE | `/auctions/:id` | Owner/Admin |
| GET | `/auctions/search?q=` | Public |
| GET | `/auctions/category/:id` | Public |
| GET | `/seller/auctions` | Seller |
| DELETE | `/admin/auctions/:id` | Admin |

## Run locally

```bash
cp .env.example .env
psql $DB_URL -f migrations/001_initial.sql
go run ./cmd
```

Image: `ghcr.io/Saydullo-Keldiyev/auction-service:sha-<commit>`
