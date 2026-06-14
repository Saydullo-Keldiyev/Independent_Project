# Sealed Secrets

Sealed Secrets encrypts Kubernetes Secrets so they can be safely committed to Git.
Only the cluster's Sealed Secrets controller can decrypt them.

## Per environment

| Environment | Namespace              | Sealed secret file                                      |
|-------------|------------------------|---------------------------------------------------------|
| Production  | `auction-system`       | `auction-system-sealed-secret.yaml`                     |
| Staging     | `auction-system-staging` | `../overlays/staging/sealed-secret.yaml`              |
| Dev         | `auction-system-dev`   | `../overlays/dev/sealed-secret.yaml`                    |

## Setup

```bash
# Install Sealed Secrets controller
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm install sealed-secrets sealed-secrets/sealed-secrets \
  --namespace kube-system \
  --version 2.15.0

# Install kubeseal CLI
brew install kubeseal   # macOS
# or: https://github.com/bitnami-labs/sealed-secrets/releases
```

## Sealing a Secret

```bash
# 1. Create a plain Secret (DO NOT commit this)
kubectl create secret generic auction-system-secrets \
  --namespace auction-system \
  --from-literal=DB_URL="postgres://auction:password@postgres:5432/auction_db" \
  --from-literal=JWT_SECRET="your-super-secret-jwt-key-min-32-chars" \
  --from-literal=REDIS_PASSWORD="redis-password" \
  --from-literal=POSTGRES_PASSWORD="postgres-password" \
  --dry-run=client \
  -o yaml > /tmp/secret.yaml

# 2. Seal it (safe to commit)
kubeseal \
  --controller-namespace kube-system \
  --controller-name sealed-secrets \
  --format yaml \
  < /tmp/secret.yaml \
  > deployments/k8s/sealed-secrets/auction-system-sealed-secret.yaml

# 3. Delete the plain secret file
rm /tmp/secret.yaml

# 4. Commit the sealed secret
git add deployments/k8s/sealed-secrets/auction-system-sealed-secret.yaml
git commit -m "chore(secrets): update sealed secrets"
```

## Rotation

```bash
# Re-seal with new values and commit
# The controller automatically decrypts and applies the new Secret
```
