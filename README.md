# 🔨 AuctionHub — Real-Time Auction Platform

A production-grade, microservices-based auction platform with real-time bidding via WebSocket, ACID-compliant payments, and event-driven architecture.

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go&logoColor=white)
![Next.js](https://img.shields.io/badge/Next.js-15-black?logo=next.js)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql&logoColor=white)
![Redis](https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white)
![Kafka](https://img.shields.io/badge/Kafka-7.5-231F20?logo=apachekafka&logoColor=white)
![Docker](https://img.shields.io/badge/Docker-Compose-2496ED?logo=docker&logoColor=white)
![Kubernetes](https://img.shields.io/badge/Kubernetes-Ready-326CE5?logo=kubernetes&logoColor=white)

---

## 🏗️ Architecture

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Frontend  │────▶│  API Gateway │────▶│  Microservices  │
│  (Next.js)  │     │  (Go + Gin)  │     │    (Go + Gin)   │
└─────────────┘     └──────────────┘     └─────────────────┘
                           │                       │
                    ┌──────┴──────┐         ┌──────┴──────┐
                    │    Redis    │         │  PostgreSQL  │
                    │  (Cache +   │         │   (ACID DB)  │
                    │   Pub/Sub)  │         └─────────────┘
                    └─────────────┘                │
                           │              ┌────────┴────────┐
                    ┌──────┴──────┐       │     Kafka       │
                    │  WebSocket  │       │ (Event Stream)  │
                    │  (Realtime) │       └─────────────────┘
                    └─────────────┘
```

## 🚀 Services

| Service | Port | Description |
|---------|------|-------------|
| **API Gateway** | 8080 | JWT auth, RBAC, rate limiting, reverse proxy |
| **User Service** | 8081 | Auth, profiles, sessions, wallet |
| **Bid Service** | 8082 | Real-time bidding, WebSocket, distributed locks |
| **Auction Service** | 8083 | CRUD, state machine, scheduler |
| **Notification Service** | 8084 | Kafka consumer, WebSocket push, email |
| **Payment Service** | 8085 | ACID transactions, hold/release, outbox pattern |
| **Search Service** | 8086 | Elasticsearch, autocomplete, trending |
| **Analytics Service** | 8087 | Revenue, dashboards, leaderboards |
| **Frontend** | 3000 | Next.js 15, React Query, Zustand, Tailwind |

## ✨ Key Features

### Real-Time Bidding
- WebSocket-powered live bid updates
- Distributed locking (Redis) prevents race conditions
- Optimistic UI with instant feedback

### Payment System (ACID)
- Hold balance on bid → Release on loss → Charge on win
- Idempotency keys prevent double-charges
- Transactional Outbox pattern for Kafka consistency
- `SELECT FOR UPDATE` row-level locking

### Auction Lifecycle
- State machine: `draft → scheduled → active → ended → archived`
- Background scheduler activates/ends auctions automatically
- Winner selection with reserve price validation

### Security
- JWT access tokens (15 min) + refresh tokens (7 days)
- Token blacklist (Redis) for logout
- bcrypt password hashing (cost 14)
- RBAC: admin / seller / bidder
- Rate limiting (Redis sliding window)
- Security headers (CSP, HSTS, X-Frame-Options)

### Observability
- Prometheus metrics on every service
- OpenTelemetry distributed tracing (Jaeger)
- Structured JSON logging (zap)
- Correlation IDs across services
- Grafana dashboards

### DevOps
- Docker Compose (one-command local setup)
- Kubernetes manifests (HPA, PDB, anti-affinity)
- Helm charts (multi-environment)
- ArgoCD GitOps (auto-deploy)
- GitHub Actions CI/CD (test → build → scan → deploy)
- Istio service mesh (mTLS, circuit breaker)

## 🛠️ Tech Stack

| Layer | Technology |
|-------|-----------|
| Backend | Go 1.22+, Gin, pgx, kafka-go |
| Frontend | Next.js 15, TypeScript, Tailwind CSS, Zustand, React Query |
| Database | PostgreSQL 16 |
| Cache | Redis 7 |
| Message Queue | Apache Kafka |
| Search | Elasticsearch 8 |
| Container | Docker, Docker Compose |
| Orchestration | Kubernetes, Helm, ArgoCD |
| CI/CD | GitHub Actions |
| Monitoring | Prometheus, Grafana, Jaeger |
| Service Mesh | Istio |

## 🚀 Quick Start

### Prerequisites
- Docker Desktop
- Go 1.22+ (for local development)
- Node.js 20+ (for frontend)

### One-Command Start (Docker)

```bash
git clone https://github.com/your-org/auction-system.git
cd auction-system
docker-compose up --build -d
```

Then open:
- **Frontend:** http://localhost:3000
- **API Gateway:** http://localhost:8080
- **Swagger UI:** http://localhost:8080/swagger

### Local Development

```bash
# 1. Start infrastructure
docker-compose up -d postgres redis zookeeper kafka

# 2. Start services (Windows)
start-all.bat

# 3. Start frontend
cd frontend && npm run dev
```

## 📡 API Endpoints

### Auth
```
POST /api/v1/auth/register
POST /api/v1/auth/login
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
```

### Auctions
```
GET  /api/v1/auctions
GET  /api/v1/auctions/:id
POST /api/v1/auctions          (seller)
PUT  /api/v1/auctions/:id      (owner)
DELETE /api/v1/auctions/:id    (owner/admin)
```

### Bids
```
POST /api/v1/bids              (authenticated)
GET  /api/v1/bids/me
GET  /api/v1/auctions/:id/bids
WS   /api/v1/ws/:auction_id   (real-time)
```

### Wallet
```
GET  /api/v1/wallet
POST /api/v1/wallet/deposit
GET  /api/v1/wallet/history
```

Full API documentation: http://localhost:8080/swagger

## 🧪 Testing

```bash
# Unit tests
cd services/bid-service && go test ./...

# Load tests (k6)
k6 run tests/load/k6/bid_load_test.js

# Chaos tests (Kubernetes)
bash tests/chaos/redis_failure/redis_chaos_test.sh
```

## 📁 Project Structure

```
auction-system/
├── api-gateway/           # API Gateway (Go)
├── services/
│   ├── user-service/      # Auth, profiles, wallet
│   ├── auction-service/   # Auction CRUD, scheduler
│   ├── bid-service/       # Bidding, WebSocket
│   ├── payment-service/   # ACID payments, outbox
│   ├── notification-service/ # Events, email, WS push
│   ├── search-service/    # Elasticsearch
│   └── analytics-service/ # Dashboards, metrics
├── frontend/              # Next.js 15
├── deployments/           # Docker, K8s, Helm
├── infra/                 # Observability stack
├── tests/                 # Unit, load, chaos
├── argocd/                # GitOps
├── .github/workflows/     # CI/CD
└── docker-compose.yml     # One-command start
```

## 🔥 Production Features

- [x] Microservices architecture (8 services)
- [x] Event-driven (Kafka)
- [x] Real-time WebSocket bidding
- [x] ACID payment transactions
- [x] Distributed locking (Redis)
- [x] Transactional Outbox pattern
- [x] JWT + Refresh token rotation
- [x] RBAC (admin/seller/bidder)
- [x] Rate limiting (Redis sliding window)
- [x] Horizontal Pod Autoscaler
- [x] Zero-downtime deployments
- [x] Circuit breaker (Istio)
- [x] Distributed tracing (OpenTelemetry)
- [x] Prometheus + Grafana monitoring
- [x] Chaos engineering tests
- [x] Load testing (k6)
- [x] GitOps (ArgoCD)
- [x] Sealed Secrets
- [x] Multi-environment Helm charts

## 📊 Performance Targets

| Metric | Target |
|--------|--------|
| Bid placement latency | < 100ms p95 |
| WebSocket broadcast | < 30ms |
| Search response | < 50ms |
| Concurrent WebSocket | 50k+ |
| Kafka throughput | 100k events/min |

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Open a Pull Request

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.
