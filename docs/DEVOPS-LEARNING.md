# DevOps o‘quv yo‘riqnomasi (loyiha tugaganda)

Bu hujjat [Saydullo-Keldiyev](https://github.com/Saydullo-Keldiyev) auction-system loyihasidagi **enterprise DevOps** qatlamlarini bosqichma-bosqich tushuntiradi.

## 1. CI/CD (GitHub Actions)

| Fayl | Nima qiladi |
|------|-------------|
| `.github/workflows/user-service.yml` | Test → build → GHCR push → Kustomize tag yangilash |
| `.github/workflows/security-scan.yml` | Gitleaks, Trivy, OPA, SBOM |

**O‘rganish tartibi:** `reusable-go-service.yml` → `develop` = dev overlay, `main` = prod/staging.

## 2. Container registry

- Image: `ghcr.io/Saydullo-Keldiyev/<service>:sha-abc1234`
- **Immutable tag** — `latest` ishlatilmaydi (Kyverno ham bloklaydi).

## 3. GitOps (ArgoCD)

```
Git (manifest) → ArgoCD sync → Kubernetes
```

| App | Path |
|-----|------|
| `auction-system-dev` | `deployments/k8s/overlays/dev` |
| `auction-system-prod` | `deployments/k8s/overlays/prod` |

`selfHeal: true` — kubectl bilan qo‘lda o‘zgartirsangiz, Git holatiga qaytadi.

## 4. Multi-environment

- **dev** — `develop` branch, 1 replica, HPA o‘chirilgan
- **staging** — `main`, 2 replica
- **prod** — `main`, 3 replica + HPA

## 5. Secrets

- Plain `Secret` Git’da **yo‘q**
- **Sealed Secrets** — `kubeseal` bilan shifrlash, clusterda ochiladi
- `.env` faqat lokal — `.env.example` dan nusxa oling

## 6. Policy (Kyverno)

`deployments/k8s/platform/kyverno/` — privileged, `latest` tag, resource limits, Cosign (Audit).

## 7. Istio & monitoring

- `argocd/apps/istio.yaml` — mesh traffic
- `argocd/apps/monitoring.yaml` — Prometheus/Grafana

## 8. Advanced deploy

- Rolling update — default (`deployment.yaml`)
- Blue/Green, Canary — `deployments/k8s/advanced/` (qo‘lda)

## 9. Auction Service (lifecycle)

| Komponent | Vazifa |
|-----------|--------|
| State machine | draft → scheduled → active → ending → ended |
| Scheduler | 5s tick + Redis lock (duplicate end oldini oladi) |
| Kafka | AUCTION_* events + bid.placed consumer |
| Redis | `auction:{id}` cache + `auction:events` pub/sub |

## 10. API Gateway (central entry)

| Komponent | Vazifa |
|-----------|--------|
| JWT (bir marta) | `X-User-ID`, `X-User-Role`, `X-Correlation-ID` inject |
| Redis rate limit | 100 req/min — IP (public) va user (auth) |
| Reverse proxy | user / auction / bid servislarga |
| WebSocket | `/api/v1/ws/:auction_id` → bid-service |
| Circuit breaker + retry | Cascade failure oldini oladi |
| `/metrics` | Prometheus — `http_requests_total`, `gateway_latency`, … |

Client faqat gateway’ga murojaat qiladi; ichki DNS: `*.svc.cluster.local`.

## 11. User Service (identity)

| Komponent | Vazifa |
|-----------|--------|
| JWT 15m / Refresh 7d | Access qisqa, refresh rotation |
| Redis blacklist | Logout |
| bcrypt cost 14 | Parol xavfsizligi |
| Kafka | `USER_REGISTERED`, `USER_LOGGED_IN`, `WALLET_CREATED` |

Batafsil deploy: [ENTERPRISE-DEPLOY.md](./ENTERPRISE-DEPLOY.md)

## Amaliy mashqlar (tavsiya)

1. `kubectl kustomize --load-restrictor LoadRestrictionsNone deployments/k8s/overlays/dev`
2. Lokal: `cd services/user-service && cp .env.example .env && go run ./cmd`
3. `curl -X POST localhost:8081/api/v1/auth/register -H "Content-Type: application/json" -d '{...}'`
4. ArgoCD UI da sync status kuzatish
5. `kubectl rollout undo deployment/user-service -n auction-system` — rollback
