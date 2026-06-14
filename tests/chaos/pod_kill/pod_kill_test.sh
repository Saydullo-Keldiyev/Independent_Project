#!/bin/bash
# ── CHAOS TEST: Pod Kill (Zero Downtime Validation) ────────────────────────────
# Verifies: Kubernetes HA — killing pods causes NO user-visible downtime
#
# Expected behavior:
#   - Pod killed → Kubernetes recreates immediately
#   - Other replicas handle traffic during restart
#   - PDB prevents killing too many pods at once
#   - Rolling update strategy ensures availability

set -euo pipefail

NAMESPACE="auction-system"
SERVICE="bid-service"
TOTAL_REQUESTS=100
FAILURES=0

echo "═══════════════════════════════════════════════════════════════"
echo "  CHAOS TEST: Pod Kill — Zero Downtime Validation"
echo "  Service: $SERVICE"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# ── Phase 1: Verify initial state ─────────────────────────────────────────────
echo "[1/4] Checking initial pod count..."
PODS=$(kubectl get pods -n $NAMESPACE -l app=$SERVICE --no-headers | wc -l)
echo "  Running pods: $PODS"

if [ "$PODS" -lt 2 ]; then
  echo "  ⚠️  Less than 2 pods — HA not possible. Skipping."
  exit 1
fi

# ── Phase 2: Start continuous traffic ──────────────────────────────────────────
echo ""
echo "[2/4] Starting continuous traffic (background)..."

# Send requests continuously in background
(
  for i in $(seq 1 $TOTAL_REQUESTS); do
    CODE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/v1/auctions/test/bids 2>/dev/null)
    if [ "$CODE" -ge 500 ] || [ "$CODE" = "000" ]; then
      echo "FAIL:$i:$CODE" >> /tmp/chaos_results.txt
    fi
    sleep 0.1
  done
) &
TRAFFIC_PID=$!

sleep 2  # let traffic start flowing

# ── Phase 3: Kill a pod ───────────────────────────────────────────────────────
echo ""
echo "[3/4] Killing a $SERVICE pod..."
POD_TO_KILL=$(kubectl get pods -n $NAMESPACE -l app=$SERVICE -o jsonpath='{.items[0].metadata.name}')
echo "  Killing: $POD_TO_KILL"
kubectl delete pod $POD_TO_KILL -n $NAMESPACE --grace-period=5

echo "  Waiting for replacement pod..."
sleep 10
kubectl wait --for=condition=ready pod -l app=$SERVICE -n $NAMESPACE --timeout=60s
echo "  ✅ Replacement pod ready"

# Wait for traffic to finish
wait $TRAFFIC_PID 2>/dev/null || true

# ── Phase 4: Analyze results ──────────────────────────────────────────────────
echo ""
echo "[4/4] Analyzing results..."

if [ -f /tmp/chaos_results.txt ]; then
  FAILURES=$(wc -l < /tmp/chaos_results.txt)
  rm /tmp/chaos_results.txt
else
  FAILURES=0
fi

FAILURE_RATE=$(echo "scale=2; $FAILURES * 100 / $TOTAL_REQUESTS" | bc)
echo "  Total requests: $TOTAL_REQUESTS"
echo "  Failures: $FAILURES"
echo "  Failure rate: ${FAILURE_RATE}%"

echo ""
if [ "$FAILURES" -eq 0 ]; then
  echo "  ✅ ZERO DOWNTIME — Perfect HA!"
elif [ "$FAILURES" -lt 3 ]; then
  echo "  ✅ Near-zero downtime ($FAILURES failures) — Acceptable"
else
  echo "  ❌ FAIL: $FAILURES failures detected during pod kill"
fi

# Final pod count
FINAL_PODS=$(kubectl get pods -n $NAMESPACE -l app=$SERVICE --no-headers | grep Running | wc -l)
echo ""
echo "  Final running pods: $FINAL_PODS (expected: $PODS)"

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  CHAOS TEST COMPLETE"
echo "═══════════════════════════════════════════════════════════════"
