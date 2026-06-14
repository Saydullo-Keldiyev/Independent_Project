# API Gateway deployments

Kubernetes manifests live in the monorepo:

- `deployments/k8s/gateway/` — Deployment, Service, HPA, Ingress
- `deployments/k8s/overlays/{dev,staging,prod}/` — env-specific replicas and URLs
- `helm/api-gateway/` — Helm chart (optional)

CI: `.github/workflows/gateway.yml` builds and updates image tags via GitOps.
