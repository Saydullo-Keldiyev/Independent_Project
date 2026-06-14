#!/bin/bash
set -euo pipefail

# Chaos Test: Simulate Redis failure
# Usage: ./scripts/chaos/redis-failure.sh [duration_seconds]

DURATION=${1:-30}
NAMESPACE="auction-system"

echo "🔥 Chaos: Simulating Redis failure for ${DURATION}s..."

# Scale Redis to 0
kubectl scale statefulset redis-master -n ${NAMESPACE} --replicas=0 2>/dev/null || \
kubectl scale deployment redis -n ${NAMESPACE} --replicas=0

echo "  ⏳ Redis is DOWN. Testing service resilience..."
echo "  Services should fall back to DB queries and degrade gracefully."

sleep ${DURATION}

echo "  🔄 Restoring Redis..."
kubectl scale statefulset redis-master -n ${NAMESPACE} --replicas=1 2>/dev/null || \
kubectl scale deployment redis -n ${NAMESPACE} --replicas=1

echo "  ⏳ Waiting for Redis recovery..."
sleep 10

echo "  ✅ Redis restored. Check service logs for fallback behavior."
