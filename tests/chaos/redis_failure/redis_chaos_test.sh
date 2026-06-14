#!/bin/bash
# ── CHAOS TEST: Redis Failure ──────────────────────────────────────────────────
# Verifies: graceful degradation when Redis is unavailable
#
# Expected behavior:
#   - Bid service continues working (falls back to DB for highest bid)
#   - Rate limiting degrades gracefully (fail-open)
#   - Token blacklist unavailable (accept tokens — security tradeoff)
#   - WebSocket hub continues locally (no cross-pod broadcast)
#   - System recovers automatically when Redis returns

set -euo pipefail

NAMESPACE="auction-system"
REDIS_POD=$(kubectl get pods -n $NAMESPACE -l app=redis -o jsonpath='{.items[0].metadata.name}')

echo "═══════════════════════════════════════════════════════════════"
echo "  CHAOS TEST: Redis Failure"
echo "═══════════════════════════════════════════════════════════════"
echo ""

# ── Phase 1: Baseline ─────────────────────────────────────────────────────────
echo "[1/5] Collecting baseline metrics..."
BASELINE_BID_RATE=$(curl -s http://localhost:8082/metrics | grep 'bid_requests_total' | tail -1 | awk '{print $2}')
echo "  Baseline bid rate: $BASELINE_BID_RATE"

# ── Phase 2: Kill Redis ───────────────────────────────────────────────────────
echo ""
echo "[2/5] Killing Redis pod..."
kubectl delete pod $REDIS_POD -n $NAMESPACE --grace-period=0 --force
echo "  Redis pod deleted. Waiting 5s..."
sleep 5

# ── Phase 3: Verify system still works ────────────────────────────────────────
echo ""
echo "[3/5] Testing bid placement without Redis..."

# Place a bid — should still work (DB fallback)
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8080/api/v1/bids \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TEST_TOKEN" \
  -d '{"auction_id":"test-auction","amount":999}')

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -1)

if [ "$HTTP_CODE" -lt 500 ]; then
  echo "  ✅ Bid service responded (HTTP $HTTP_CODE) — graceful degradation working"
else
  echo "  ❌ FAIL: Bid service returned 5xx — not degrading gracefully"
fi

# Health check
HEALTH=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/health)
echo "  Health endpoint: HTTP $HEALTH"

READY=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/ready)
echo "  Ready endpoint: HTTP $READY (expected 503 — Redis down)"

# ── Phase 4: Wait for Redis recovery ─────────────────────────────────────────
echo ""
echo "[4/5] Waiting for Redis pod to restart (Kubernetes auto-recovery)..."
kubectl wait --for=condition=ready pod -l app=redis -n $NAMESPACE --timeout=120s
echo "  ✅ Redis pod recovered"

# ── Phase 5: Verify full recovery ─────────────────────────────────────────────
echo ""
echo "[5/5] Verifying full system recovery..."
sleep 5

READY_AFTER=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/ready)
echo "  Ready endpoint after recovery: HTTP $READY_AFTER"

if [ "$READY_AFTER" = "200" ]; then
  echo "  ✅ System fully recovered"
else
  echo "  ⚠️  System not yet ready — may need more time"
fi

echo ""
echo "═══════════════════════════════════════════════════════════════"
echo "  CHAOS TEST COMPLETE"
echo "═══════════════════════════════════════════════════════════════"
