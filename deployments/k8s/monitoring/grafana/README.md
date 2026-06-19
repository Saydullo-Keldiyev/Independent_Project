# Grafana Dashboards - Auction System

## Overview

This directory contains Grafana dashboards as code for the auction system.
Dashboards are automatically provisioned on deployment via Kubernetes ConfigMaps.

## Dashboards

| Dashboard | UID | Description |
|-----------|-----|-------------|
| Service Level | `auction-service-level` | Request rate, error rate, latency (P50/P95/P99), resource saturation |
| Infrastructure | `auction-infrastructure` | CPU, memory, disk, network per pod (1h default, 30s refresh) |
| Kafka | `auction-kafka` | Consumer lag, throughput/topic, partition count, DLQ count |
| Business Metrics | `auction-business` | Active auctions, bids/min, transaction volume, WebSocket connections |
| Security | `auction-security` | Rate limit hits, auth failures, blocked IPs (15m sliding window) |

## Auto-Provisioning

Dashboards are auto-provisioned using Grafana's file-based provisioning:

1. `configmap-provisioning.yaml` — tells Grafana to load dashboards from `/etc/grafana/dashboards`
2. `kustomization.yaml` — generates a ConfigMap from the JSON dashboard files
3. The Grafana deployment mounts both ConfigMaps as volumes

### Deployment

```bash
# Apply using kustomize
kubectl apply -k deployments/k8s/monitoring/grafana/

# Or apply individually
kubectl apply -f deployments/k8s/monitoring/grafana/configmap-provisioning.yaml
kubectl create configmap grafana-dashboards \
  --from-file=deployments/k8s/monitoring/grafana/dashboards/ \
  -n monitoring
```

## Requirements Coverage

- **Req 14.1**: Service-level dashboard with request rate, error rate, latency P50/P95/P99, resource saturation
- **Req 14.2**: Infrastructure dashboard with CPU, memory, disk, network per pod (1h default, 30s refresh)
- **Req 14.3**: Kafka dashboard with consumer lag, throughput, partitions, DLQ count
- **Req 14.4**: Business metrics with active auctions, bids/min, transaction volume, WebSocket connections
- **Req 14.5**: Security dashboard with rate limit hits, auth failures, blocked IPs (15m sliding window)
- **Req 14.6**: Auto-provisioning via ConfigMaps/provisioning directory
- **Req 14.7**: "No Data" display within 60s (configured via `noValue: "No Data"` on all panels)
- **Req 14.8**: 15-day minimum data retention (Prometheus `--storage.tsdb.retention.time=15d`)

## Panel Features

- All panels display **"No Data"** when the data source is unreachable (via `noValue` field config)
- Default time range is **1 hour** with **30-second refresh**
- Time picker supports ranges up to **15 days** for historical analysis
- Threshold-based coloring for saturation and error indicators
- Template variables for filtering by service, pod, topic, etc.
