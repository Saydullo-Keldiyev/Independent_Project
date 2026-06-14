# User Service — Identity, Auth, Wallet

Production-ready identity provider for the auction platform.

## Features

- Register / Login / Refresh / Logout
- JWT (15m) + refresh token rotation (7d)
- bcrypt cost 14
- RBAC (`admin`, `seller`, `bidder`)
- Wallet (ACID deposit, hold/release for bids)
- Session tracking (max 5 per user)
- Redis token blacklist
- Audit logs + Kafka events

## Quick start

```bash
cp .env.example .env
psql $DB_URL -f migrations/001_initial.sql
go run ./cmd
```

## API (`/api/v1`)

| Method | Path | Auth |
|--------|------|------|
| POST | `/auth/register` | No |
| POST | `/auth/login` | No |
| POST | `/auth/refresh` | No |
| POST | `/auth/logout` | Bearer |
| GET | `/users/me` | Bearer |
| PUT | `/users/me` | Bearer |
| GET | `/wallet` | Bearer |
| GET | `/wallet/history` | Bearer |
| POST | `/wallet/deposit` | Bearer |

Health: `GET /health`, `GET /ready`, metrics: `GET /metrics`

## DevOps

- Image: `ghcr.io/Saydullo-Keldiyev/user-service:sha-<commit>`
- Learning guide: [`../../docs/DEVOPS-LEARNING.md`](../../docs/DEVOPS-LEARNING.md)
