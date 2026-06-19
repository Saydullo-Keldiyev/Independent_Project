# Data Layer — High Availability & Disaster Recovery

This directory contains Kubernetes manifests for the production data layer with HA and disaster recovery capabilities.

## Components

### PostgreSQL HA (`postgresql-ha.yaml`)
- Uses **CloudNativePG** operator CRD (`Cluster` resource)
- Primary + 1 synchronous replica in a different AZ
- Automatic failover with promotion within 30 seconds
- WAL archiving for point-in-time recovery
- Topology constraints ensure replicas span availability zones

### Redis Cluster (`redis-cluster.yaml`)
- 6-node Redis Cluster (3 masters + 3 replicas)
- Masters distributed across 3 availability zones
- Automatic failover within 15 seconds via Redis Cluster protocol
- Persistent storage with SSD-backed volumes

### Kafka HA (`kafka-ha.yaml`)
- 3 brokers distributed across 2+ availability zones
- Replication factor 3, min.insync.replicas 2
- ZooKeeper ensemble (3 nodes) across AZs
- Rack-awareness enabled for cross-AZ replica placement

## Prerequisites

- CloudNativePG operator installed: `kubectl apply -f https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/release-1.22/releases/cnpg-1.22.0.yaml`
- Storage class `fast-ssd` available (or adjust `storageClassName` in manifests)
- Kubernetes nodes labeled with `topology.kubernetes.io/zone`
- Namespace `auction-system` created

## Deployment Order

1. PostgreSQL HA (primary dependency for services)
2. Redis Cluster (required for distributed locking and caching)
3. Kafka HA (required for event streaming)

## Verification

```bash
# PostgreSQL - check cluster status
kubectl get cluster -n auction-system
kubectl get pods -l cnpg.io/cluster=auction-db -n auction-system

# Redis - check cluster info
kubectl exec -n auction-system redis-cluster-0 -- redis-cli -a $REDIS_PASSWORD cluster info

# Kafka - check broker status
kubectl exec -n auction-system kafka-0 -- kafka-metadata.sh --snapshot /var/lib/kafka/data/__cluster_metadata-0/00000000000000000000.log --cluster-id <cluster-id>
```
