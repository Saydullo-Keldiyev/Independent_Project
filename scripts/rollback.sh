#!/bin/bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════════════════════
# Rollback Script — reverts a deployment to its previous revision
# Usage: ./scripts/rollback.sh [service] [revision]
# Examples:
#   ./scripts/rollback.sh bid-service        # Rollback to previous version
#   ./scripts/rollback.sh bid-service 3      # Rollback to specific revision
# ═══════════════════════════════════════════════════════════════════════════════

SERVICE=${1:?Usage: rollback.sh <service> [revision]}
REVISION=${2:-}
NAMESPACE="auction-system"

echo "═══════════════════════════════════════════════════════════"
echo "  Rolling back: ${SERVICE}"
echo "  Namespace:    ${NAMESPACE}"
echo "═══════════════════════════════════════════════════════════"

# Show rollout history
echo ""
echo "📋 Rollout history:"
kubectl rollout history deployment/${SERVICE} -n ${NAMESPACE}

# Perform rollback
if [[ -n "$REVISION" ]]; then
  echo ""
  echo "⏪ Rolling back to revision ${REVISION}..."
  kubectl rollout undo deployment/${SERVICE} -n ${NAMESPACE} --to-revision=${REVISION}
else
  echo ""
  echo "⏪ Rolling back to previous version..."
  kubectl rollout undo deployment/${SERVICE} -n ${NAMESPACE}
fi

# Wait for rollback to complete
echo "⏳ Waiting for rollout..."
kubectl rollout status deployment/${SERVICE} -n ${NAMESPACE} --timeout=120s

echo ""
echo "✅ Rollback complete!"
echo ""
echo "Current pods:"
kubectl get pods -n ${NAMESPACE} -l app=${SERVICE} -o wide
