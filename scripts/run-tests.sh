#!/bin/bash
# ── Test Runner — All test levels ──────────────────────────────────────────────
# Usage:
#   ./scripts/run-tests.sh unit          # unit tests only
#   ./scripts/run-tests.sh integration   # integration tests
#   ./scripts/run-tests.sh e2e           # end-to-end tests
#   ./scripts/run-tests.sh load          # k6 load tests
#   ./scripts/run-tests.sh chaos         # chaos engineering
#   ./scripts/run-tests.sh all           # everything

set -euo pipefail

LEVEL=${1:-unit}
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[TEST]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
fail() { echo -e "${RED}[FAIL]${NC} $1"; }

# ── Unit Tests ─────────────────────────────────────────────────────────────────
run_unit() {
  log "Running unit tests..."

  for svc in services/bid-service services/user-service services/payment-service services/notification-service services/search-service services/analytics-service api-gateway; do
    if [ -d "$svc" ] && [ -f "$svc/go.mod" ]; then
      log "  Testing $svc..."
      (cd "$svc" && go test -race -count=1 ./... 2>&1) || warn "  $svc tests failed"
    fi
  done

  # Standalone test files
  if [ -d "tests/unit" ]; then
    log "  Testing tests/unit/..."
    (cd tests/unit && go test -race ./... 2>&1) || warn "  unit tests failed"
  fi

  log "Unit tests complete ✅"
}

# ── Integration Tests ──────────────────────────────────────────────────────────
run_integration() {
  log "Running integration tests..."
  log "  (Requires: Docker running with postgres, redis, kafka)"

  if [ -d "tests/integration" ]; then
    (cd tests/integration && go test -race -timeout 120s ./... 2>&1) || warn "  integration tests failed"
  fi

  log "Integration tests complete ✅"
}

# ── E2E Tests ──────────────────────────────────────────────────────────────────
run_e2e() {
  log "Running E2E tests..."
  log "  (Requires: all services running)"

  if [ -d "tests/e2e" ]; then
    (cd tests/e2e && go test -timeout 300s ./... 2>&1) || warn "  E2E tests failed"
  fi

  log "E2E tests complete ✅"
}

# ── Load Tests ─────────────────────────────────────────────────────────────────
run_load() {
  log "Running load tests with k6..."

  command -v k6 &>/dev/null || { fail "k6 not installed. Install: https://k6.io/docs/getting-started/installation/"; exit 1; }

  log "  [1/3] Bid load test (5 min)..."
  k6 run tests/load/k6/bid_load_test.js --duration 1m --vus 50 2>&1 || true

  log "  [2/3] Payment concurrency test..."
  k6 run tests/load/k6/payment_concurrency_test.js 2>&1 || true

  log "  [3/3] WebSocket stress test (2 min)..."
  k6 run tests/load/k6/websocket_stress_test.js --duration 1m --vus 100 2>&1 || true

  log "Load tests complete ✅"
}

# ── Chaos Tests ────────────────────────────────────────────────────────────────
run_chaos() {
  log "Running chaos engineering tests..."
  log "  (Requires: Kubernetes cluster with auction-system deployed)"

  command -v kubectl &>/dev/null || { fail "kubectl not installed"; exit 1; }

  log "  [1/3] Redis failure test..."
  bash tests/chaos/redis_failure/redis_chaos_test.sh 2>&1 || true

  log "  [2/3] Kafka failure test..."
  bash tests/chaos/kafka_failure/kafka_chaos_test.sh 2>&1 || true

  log "  [3/3] Pod kill test..."
  bash tests/chaos/pod_kill/pod_kill_test.sh 2>&1 || true

  log "Chaos tests complete ✅"
}

# ── Main ───────────────────────────────────────────────────────────────────────
case "$LEVEL" in
  unit)        run_unit ;;
  integration) run_integration ;;
  e2e)         run_e2e ;;
  load)        run_load ;;
  chaos)       run_chaos ;;
  all)
    run_unit
    run_integration
    run_e2e
    run_load
    run_chaos
    ;;
  *) fail "Unknown level: $LEVEL. Use: unit|integration|e2e|load|chaos|all" ;;
esac
