# Advanced deployment strategies (manual / optional)

These manifests are **not** synced by default ArgoCD apps (avoids conflicting with rolling-update `bid-service`).

## Blue/Green

```bash
kubectl apply -f deployments/k8s/advanced/bid-service/blue-green.yaml -n auction-system
kubectl patch service bid-service -n auction-system -p '{"spec":{"selector":{"slot":"green"}}}'
```

## Canary (~10% traffic via replica ratio)

```bash
kubectl apply -f deployments/k8s/advanced/bid-service/canary.yaml -n auction-system
```

For Istio-weighted canary, use `deployments/k8s/istio/virtual-services.yaml` weights instead.

## Optional ArgoCD app

Uncomment / apply `argocd/apps/bid-service-advanced.yaml` only when running blue/green or canary.
