# Enterprise CI/CD + GitOps + DevSecOps

## Architecture

```
git push (develop) → CI → GHCR → update overlays/dev → ArgoCD → auction-system-dev
git push (main)    → CI → GHCR → update overlays/prod+staging → ArgoCD → prod/staging
```

## One-time cluster setup

1. Install [ArgoCD](https://argo-cd.readthedocs.io/), [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets), [Kyverno](https://kyverno.io/), [Istio](https://istio.io/).
2. Replace `Saydullo-Keldiyev` in `argocd/**/*.yaml` with your GitHub org/username.
3. Apply root app: `kubectl apply -f argocd/project.yaml -f argocd/application.yaml`
4. Generate real sealed secrets per namespace (see `deployments/k8s/sealed-secrets/README.md`).

## Environments

| Env       | Namespace               | ArgoCD app               | Git branch |
|-----------|-------------------------|--------------------------|------------|
| Dev       | `auction-system-dev`    | `auction-system-dev`     | `develop`  |
| Staging   | `auction-system-staging`| `auction-system-staging` | `main`     |
| Production| `auction-system`        | `auction-system-prod`    | `main`     |

## Kustomize overlays

- Base: `deployments/k8s/base`
- Overlays: `deployments/k8s/overlays/{dev,staging,prod}`
- Advanced (manual): `deployments/k8s/advanced/bid-service`

Validate locally:

```bash
kubectl kustomize --load-restrictor LoadRestrictionsNone deployments/k8s/overlays/prod
```

ArgoCD apps set the same `buildOptions` (shared manifests live outside overlay root).

## Rollback

```bash
# Kubernetes rollout
kubectl rollout undo deployment/bid-service -n auction-system

# GitOps — revert overlay image tag commit, ArgoCD self-heals
git revert <commit-sha>
```

## Policies (Kyverno)

Cluster policies live in `deployments/k8s/platform/kyverno/`. Cosign verification starts in **Audit** — set `validationFailureAction: Enforce` when ready.

## Helm (optional)

`helm/bid-service` + `argocd/apps/bid-service-helm-prod.yaml` for Helm-based prod deploy. Do **not** enable together with Kustomize `bid-service` in the same namespace.
