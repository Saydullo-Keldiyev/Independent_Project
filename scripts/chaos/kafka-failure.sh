#!/bin/bash
set -euo pipefail

# Chaos Test: Simulate Kafka failure
# Usage: ./scripts/chaos/kafka-failure.sh [duration_seconds]

DURATION=${1:-60}
NAMESPACE="auction-system"

echo "🔥 Chaos: Simulating Kafka failure for ${DURATION}s..."

# Network policy to block Kafka traffic
cat <<EOF | kubectl apply -f -
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: chaos-block-kafka
  namespace: ${NAMESPACE}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: kafka
  policyTypes:
    - Ingress
  ingress: []
EOF

echo "  ⏳ Kafka is BLOCKED. Testing service resilience..."
echo "  Events should be buffered or retried by producers."

sleep ${DURATION}

echo "  🔄 Restoring Kafka connectivity..."
kubectl delete networkpolicy chaos-block-kafka -n ${NAMESPACE}

echo "  ⏳ Waiting for Kafka recovery and consumer catch-up..."
sleep 15

echo "  ✅ Kafka restored. Check consumer lag and DLQ."
