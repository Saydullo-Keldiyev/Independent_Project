# Auction System Documentation

## Overview

This directory contains all documentation for the auction-system microservices platform. The system is built with Go 1.22 and deployed on Kubernetes with Istio service mesh.

## Documentation Index

### Architecture

| Resource | Path | Description |
|----------|------|-------------|
| System Architecture | [architecture/system-architecture.mmd](architecture/system-architecture.mmd) | Full system architecture diagram (Mermaid format) |
| Data Flow | [architecture/data-flow.mmd](architecture/data-flow.mmd) | Bid placement data flow sequence diagram |
| Deployment Architecture | [architecture/deployment.mmd](architecture/deployment.mmd) | Multi-AZ deployment topology |

### API Specifications

| Resource | Path | Description |
|----------|------|-------------|
| API Gateway OpenAPI | [../api-gateway/docs/swagger.yaml](../api-gateway/docs/swagger.yaml) | API Gateway routes and schemas |
| Bid Service OpenAPI | [../services/bid-service/docs/swagger.yaml](../services/bid-service/docs/swagger.yaml) | Bid Service endpoints |
| Auction Service OpenAPI | [../services/auction-service/docs/swagger.yaml](../services/auction-service/docs/swagger.yaml) | Auction Service endpoints |
| User Service OpenAPI | [../services/user-service/docs/swagger.yaml](../services/user-service/docs/swagger.yaml) | User Service endpoints |
| Payment Service OpenAPI | [../services/payment-service/docs/swagger.yaml](../services/payment-service/docs/swagger.yaml) | Payment Service endpoints |
| Notification Service OpenAPI | [../services/notification-service/docs/swagger.yaml](../services/notification-service/docs/swagger.yaml) | Notification Service endpoints |

### Operational Runbooks

| Alert | Path | Severity |
|-------|------|----------|
| High Error Rate | [runbooks/high-error-rate.md](runbooks/high-error-rate.md) | Critical |
| High Latency | [runbooks/high-latency.md](runbooks/high-latency.md) | Warning |
| Kafka Consumer Lag | [runbooks/kafka-consumer-lag.md](runbooks/kafka-consumer-lag.md) | Critical |
| Pod Restart Loop | [runbooks/pod-restart-loop.md](runbooks/pod-restart-loop.md) | Warning |
| Backup Failure | [runbooks/backup-failure.md](runbooks/backup-failure.md) | Critical |

### Environment Variables

| Service | Path | Description |
|---------|------|-------------|
| API Gateway | [env-vars/api-gateway.md](env-vars/api-gateway.md) | Gateway configuration, rate limiting, upstream URLs |
| Bid Service | [env-vars/bid-service.md](env-vars/bid-service.md) | WebSocket, distributed lock, Kafka config |
| Auction Service | [env-vars/auction-service.md](env-vars/auction-service.md) | Auction lifecycle configuration |
| User Service | [env-vars/user-service.md](env-vars/user-service.md) | Authentication, JWT rotation |
| Payment Service | [env-vars/payment-service.md](env-vars/payment-service.md) | Saga, outbox, wallet operations |
| Notification Service | [env-vars/notification-service.md](env-vars/notification-service.md) | SMTP, WebSocket, Kafka consumers |
| Search Service | [env-vars/search-service.md](env-vars/search-service.md) | Search indexing configuration |

### Architecture Decision Records (ADRs)

| ADR | Path | Status |
|-----|------|--------|
| Template | [adr/000-template.md](adr/000-template.md) | — |
| ADR-001: Distributed Locking with Fencing Tokens | [adr/001-distributed-locking-with-fencing-tokens.md](adr/001-distributed-locking-with-fencing-tokens.md) | Accepted |
| ADR-002: Saga Pattern for Auction Settlement | [adr/002-saga-pattern-for-auction-settlement.md](adr/002-saga-pattern-for-auction-settlement.md) | Accepted |
| ADR-003: Outbox Pattern for Event Publication | [adr/003-outbox-pattern-for-event-publication.md](adr/003-outbox-pattern-for-event-publication.md) | Accepted |
| ADR-004: Sliding Window Rate Limiting | [adr/004-sliding-window-rate-limiting.md](adr/004-sliding-window-rate-limiting.md) | Accepted |
| ADR-005: Circuit Breaker with Cached Fallback | [adr/005-circuit-breaker-with-cached-fallback.md](adr/005-circuit-breaker-with-cached-fallback.md) | Accepted |

### Infrastructure & Deployment

| Resource | Path | Description |
|----------|------|-------------|
| Helm Charts | [../helm/](../helm/) | Kubernetes deployment charts |
| ArgoCD Applications | [../argocd/](../argocd/) | GitOps deployment configuration |
| Kubernetes Manifests | [../deployments/](../deployments/) | Raw Kubernetes manifests |
| Docker Compose | [../docker-compose.yml](../docker-compose.yml) | Local development environment |
| CI Workflows | [../.github/workflows/](../.github/workflows/) | GitHub Actions pipelines |

### Monitoring & Observability

| Resource | Path | Description |
|----------|------|-------------|
| Prometheus Alert Rules | [../infra/monitoring/prometheus/alerts/](../infra/monitoring/prometheus/alerts/) | Alerting rules configuration |
| Grafana Dashboards | [../infra/monitoring/grafana/dashboards/](../infra/monitoring/grafana/dashboards/) | Dashboard JSON definitions |
| Alertmanager Config | [../infra/monitoring/alertmanager/](../infra/monitoring/alertmanager/) | Notification routing |

### Additional Resources

| Resource | Path | Description |
|----------|------|-------------|
| Project Report | [INDEPENDENT_PROJECT_REPORT.md](INDEPENDENT_PROJECT_REPORT.md) | Project overview and goals |
| Enterprise Deployment Guide | [ENTERPRISE-DEPLOY.md](ENTERPRISE-DEPLOY.md) | Production deployment instructions |
| DevOps Learning | [DEVOPS-LEARNING.md](DEVOPS-LEARNING.md) | DevOps practices reference |
| Root README | [../README.md](../README.md) | Getting started guide |

## Rendering Architecture Diagrams

Architecture diagrams are stored as Mermaid `.mmd` files for version control. To render:

1. **GitHub/GitLab:** Mermaid diagrams render natively in markdown files
2. **VS Code:** Install the "Mermaid Preview" extension
3. **CLI:** Use `mmdc` (mermaid-cli) to export to PNG/SVG:
   ```bash
   npx @mermaid-js/mermaid-cli mmdc -i docs/architecture/system-architecture.mmd -o docs/architecture/system-architecture.svg
   ```

## Contributing to Documentation

- **Runbooks:** Follow the template structure (Description → Symptoms → Diagnosis → Resolution → Escalation)
- **ADRs:** Copy `adr/000-template.md` and increment the number. Set status to "Proposed" initially
- **Environment variables:** Document all new env vars with description, default, required flag, and validation rules
- **Architecture diagrams:** Use Mermaid format and include a `Last Updated` comment header
