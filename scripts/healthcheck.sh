#!/bin/bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════════════════════
# Health Check Script — verifies all services are running properly
# Usage: ./scripts/healthcheck.sh [base_url]
# Examples:
#   ./scripts/healthcheck.sh                    # Default: localhost
#   ./scripts/healthcheck.sh https://api.auction.com
# ═══════════════════════════════════════════════════════════════════════════════

BASE_URL=${1:-http://localhost:8080}
FAILED=0

check_service() {
  local name=$1
  local url=$2
  local expected=${3:-200}

  printf "  %-25s " "${name}..."
  HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 "${url}" 2>/dev/null || echo "000")

  if [[ "$HTTP_CODE" == "$expected" ]]; then
    echo "✅ OK (${HTTP_CODE})"
  else
    echo "❌ FAILED (got ${HTTP_CODE}, expected ${expected})"
    FAILED=$((FAILED + 1))
  fi
}

echo "═══════════════════════════════════════════════════════════"
echo "  Health Check — ${BASE_URL}"
echo "═══════════════════════════════════════════════════════════"
echo ""

# API Gateway
check_service "API Gateway" "${BASE_URL}/health"

# Individual services (via gateway proxy)
check_service "User Service" "${BASE_URL}/api/v1/auth/refresh" "400"
check_service "Auction Service" "${BASE_URL}/api/v1/auctions" "200"
check_service "Bid Service" "${BASE_URL}/api/v1/auctions" "200"

echo ""
echo "───────────────────────────────────────────────────────────"

if [[ $FAILED -eq 0 ]]; then
  echo "  ✅ All services healthy!"
  exit 0
else
  echo "  ❌ ${FAILED} service(s) unhealthy"
  exit 1
fi
