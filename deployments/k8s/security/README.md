# Kubernetes Network Policies & Pod Security

This directory contains Kubernetes NetworkPolicy manifests and Pod Security configuration for the `auction-system` namespace.

## Overview

Implements defense-in-depth network segmentation following the principle of least privilege:

1. **Default-deny** all ingress and egress traffic
2. **Explicit allow** rules only for declared service dependencies
3. **Pod Security Standards** at "restricted" level via namespace labels
4. **DNS egress** allowed for all pods (required for service discovery)

## Files

| File | Purpose |
|------|---------|
| `namespace.yaml` | Namespace with PSS restricted labels |
| `default-deny.yaml` | Default-deny ingress + egress NetworkPolicies |
| `allow-dns.yaml` | DNS egress for all pods |
| `netpol-api-gateway.yaml` | API Gateway ingress/egress rules |
| `netpol-bid-service.yaml` | Bid Service ingress/egress rules |
| `netpol-auction-service.yaml` | Auction Service ingress/egress rules |
| `netpol-user-service.yaml` | User Service ingress/egress rules |
| `netpol-notification-service.yaml` | Notification Service ingress/egress rules |
| `netpol-payment-service.yaml` | Payment Service ingress/egress rules |
| `netpol-search-service.yaml` | Search Service ingress/egress rules |
| `pod-security-context.yaml` | Reference template for container security context |
| `kustomization.yaml` | Kustomize configuration |

## Service Communication Matrix

Based on Requirement 20.1, the following pod-to-pod communication is allowed:

```
api-gateway  →  bid-service
api-gateway  →  user-service
api-gateway  →  auction-service
api-gateway  →  notification-service
api-gateway  →  payment-service
api-gateway  →  search-service
bid-service  →  auction-service
bid-service  →  payment-service
auction-service  →  notification-service
```

## Data Layer Access

| Service | PostgreSQL | Redis | Kafka |
|---------|:----------:|:-----:|:-----:|
| api-gateway | - | ✓ | ✓ |
| bid-service | ✓ | ✓ | ✓ |
| auction-service | ✓ | - | ✓ |
| user-service | ✓ | ✓ | - |
| notification-service | - | ✓ | ✓ |
| payment-service | ✓ | - | ✓ |
| search-service | ✓ | - | ✓ |

## Pod Security Standards (Restricted)

All containers in the namespace MUST comply with:

- `runAsNonRoot: true` — containers cannot run as root
- `runAsUser: 10000` — dedicated non-root UID
- `allowPrivilegeEscalation: false` — no privilege escalation
- `capabilities.drop: ["ALL"]` — all Linux capabilities dropped
- `readOnlyRootFilesystem: true` — immutable filesystem
- `seccompProfile.type: RuntimeDefault` — seccomp enabled
- Only `/tmp` is writable via emptyDir volume

## Deployment

```bash
# Apply using kustomize
kubectl apply -k deployments/k8s/security/

# Verify NetworkPolicies
kubectl get networkpolicies -n auction-system

# Verify namespace labels
kubectl get namespace auction-system --show-labels
```

## Requirements Traceability

- **Req 20.1**: Service communication restrictions → per-service NetworkPolicies
- **Req 20.2**: Default-deny ingress + explicit allow → `default-deny.yaml` + `netpol-*.yaml`
- **Req 20.3**: PSS restricted level → namespace labels in `namespace.yaml`
- **Req 20.4**: runAsNonRoot, drop capabilities, no privilege escalation → deployment securityContext
- **Req 20.5**: readOnlyRootFilesystem + emptyDir for /tmp → deployment volumeMounts
- **Req 20.7**: Default-deny egress + explicit allow → `default-deny.yaml` + `netpol-*.yaml`
