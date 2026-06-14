#!/bin/bash
set -euo pipefail

# Chaos Test: Kill a random pod of a service
# Usage: ./scripts/chaos/kill-pod.sh <service-name>

SERVICE=${1:?Usage: kill-pod.sh <service-name>}
NAMESPACE="auction-system"

echo "🔥 Chaos: Killing random ${SERVICE} pod..."

POD=$(kubectl get pods -n ${NAMESPACE} -l app=${SERVICE} -o jsonpath='{.items[0].metadata.name}')

if [[ -z "$POD" ]]; then
  echo "❌ No pods found for ${SERVICE}"
  exit 1
fi

echo "  Target: ${POD}"
kubectl delete pod ${POD} -n ${NAMESPACE} --grace-period=0 --force

echo "  ⏳ Waiting for recovery..."
sleep 5
kubectl get pods -n ${NAMESPACE} -l app=${SERVICE}

echo ""
echo "  ✅ Pod killed. Check if service recovered (PDB should keep min available)."
