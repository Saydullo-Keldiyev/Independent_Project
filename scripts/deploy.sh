#!/bin/bash
set -euo pipefail

# ═══════════════════════════════════════════════════════════════════════════════
# Production Deployment Script
# Usage: ./scripts/deploy.sh [environment] [service]
# Examples:
#   ./scripts/deploy.sh prod          # Deploy all services to prod
#   ./scripts/deploy.sh staging       # Deploy all to staging
#   ./scripts/deploy.sh prod bid-service  # Deploy only bid-service to prod
# ═══════════════════════════════════════════════════════════════════════════════

ENV=${1:-prod}
SERVICE=${2:-all}
NAMESPACE="auction-system"
REGISTRY="ghcr.io/saydullo-keldiyev"
TAG=$(git rev-parse --short HEAD)

echo "═══════════════════════════════════════════════════════════"
echo "  Deploying to: ${ENV}"
echo "  Service:      ${SERVICE}"
echo "  Tag:          sha-${TAG}"
echo "  Namespace:    ${NAMESPACE}"
echo "═══════════════════════════════════════════════════════════"

# Validate environment
if [[ "$ENV" != "prod" && "$ENV" != "staging" && "$ENV" != "dev" ]]; then
  echo "❌ Invalid environment: $ENV (use prod, staging, or dev)"
  exit 1
fi

# Build and push images
build_and_push() {
  local svc=$1
  local dockerfile=$2
  echo "🔨 Building ${svc}..."
  docker build -t "${REGISTRY}/${svc}:sha-${TAG}" -f "${dockerfile}" .
  docker push "${REGISTRY}/${svc}:sha-${TAG}"
  echo "✅ ${svc} pushed: sha-${TAG}"
}

deploy_service() {
  local svc=$1
  echo "🚀 Deploying ${svc} to ${ENV}..."
  kubectl set image deployment/${svc} ${svc}="${REGISTRY}/${svc}:sha-${TAG}" \
    -n ${NAMESPACE} --record
  kubectl rollout status deployment/${svc} -n ${NAMESPACE} --timeout=120s
  echo "✅ ${svc} deployed successfully"
}

if [[ "$SERVICE" == "all" ]]; then
  SERVICES=("api-gateway" "user-service" "auction-service" "bid-service" "notification-service")
  for svc in "${SERVICES[@]}"; do
    deploy_service "$svc"
  done
else
  deploy_service "$SERVICE"
fi

echo ""
echo "═══════════════════════════════════════════════════════════"
echo "  ✅ Deployment complete!"
echo "  Run: ./scripts/healthcheck.sh to verify"
echo "═══════════════════════════════════════════════════════════"
