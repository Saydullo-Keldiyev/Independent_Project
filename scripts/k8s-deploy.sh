#!/bin/bash
# Production Kubernetes deployment script
# Usage: ./scripts/k8s-deploy.sh [apply|delete|status]

set -euo pipefail

ACTION=${1:-apply}
K8S_DIR="deployments/k8s"
NAMESPACE="auction-system"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log()  { echo -e "${GREEN}[INFO]${NC}  $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC}  $1"; }
err()  { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

# ── Validate kubectl is available ────────────────────────────────────────────
command -v kubectl &>/dev/null || err "kubectl not found"

# ── Apply in correct dependency order ────────────────────────────────────────
apply_all() {
  log "Deploying auction-system to Kubernetes..."

  # 1. Namespace first
  log "Creating namespace..."
  kubectl apply -f "$K8S_DIR/namespace.yaml"

  # 2. RBAC
  log "Applying RBAC..."
  kubectl apply -f "$K8S_DIR/rbac.yaml"

  # 3. Config & Secrets
  log "Applying ConfigMap and Secrets..."
  kubectl apply -f "$K8S_DIR/configmap.yaml"
  kubectl apply -f "$K8S_DIR/secrets.yaml"

  # 4. Infrastructure (stateful — deploy first, wait for ready)
  log "Deploying Redis..."
  kubectl apply -f "$K8S_DIR/redis/redis.yaml"
  kubectl rollout status statefulset/redis -n "$NAMESPACE" --timeout=120s

  log "Deploying Kafka + Zookeeper..."
  kubectl apply -f "$K8S_DIR/kafka/kafka.yaml"
  kubectl rollout status statefulset/zookeeper -n "$NAMESPACE" --timeout=120s
  kubectl rollout status statefulset/kafka -n "$NAMESPACE" --timeout=180s

  # 5. Application services
  log "Deploying user-service..."
  kubectl apply -f "$K8S_DIR/user-service/deployment.yaml"

  log "Deploying auction-service..."
  kubectl apply -f "$K8S_DIR/auction-service/deployment.yaml"

  log "Deploying bid-service..."
  kubectl apply -f "$K8S_DIR/bid-service/deployment.yaml"
  kubectl apply -f "$K8S_DIR/bid-service/hpa.yaml"
  kubectl apply -f "$K8S_DIR/bid-service/pdb.yaml"

  log "Deploying notification-service..."
  kubectl apply -f "$K8S_DIR/notification-service/deployment.yaml"

  # 6. API Gateway (last — depends on all services)
  log "Deploying api-gateway..."
  kubectl apply -f "$K8S_DIR/gateway/deployment.yaml"
  kubectl apply -f "$K8S_DIR/gateway/service.yaml"
  kubectl apply -f "$K8S_DIR/gateway/hpa.yaml"
  kubectl apply -f "$K8S_DIR/gateway/pdb.yaml"

  # 7. Ingress
  log "Applying Ingress..."
  kubectl apply -f "$K8S_DIR/ingress/ingress.yaml"

  # 8. Monitoring
  log "Deploying monitoring stack..."
  kubectl apply -f "$K8S_DIR/monitoring/prometheus.yaml"
  kubectl apply -f "$K8S_DIR/monitoring/grafana.yaml"

  log "✅ Deployment complete!"
  log "Run './scripts/k8s-deploy.sh status' to check pod health"
}

# ── Status check ──────────────────────────────────────────────────────────────
status_all() {
  echo ""
  log "=== Pod Status ==="
  kubectl get pods -n "$NAMESPACE" -o wide

  echo ""
  log "=== HPA Status ==="
  kubectl get hpa -n "$NAMESPACE"

  echo ""
  log "=== Services ==="
  kubectl get svc -n "$NAMESPACE"

  echo ""
  log "=== Ingress ==="
  kubectl get ingress -n "$NAMESPACE"
}

# ── Rollback ──────────────────────────────────────────────────────────────────
rollback() {
  SERVICE=${2:-bid-service}
  log "Rolling back $SERVICE..."
  kubectl rollout undo deployment/"$SERVICE" -n "$NAMESPACE"
  kubectl rollout status deployment/"$SERVICE" -n "$NAMESPACE"
}

# ── Delete ────────────────────────────────────────────────────────────────────
delete_all() {
  warn "This will delete all auction-system resources. Are you sure? (yes/no)"
  read -r confirm
  if [[ "$confirm" != "yes" ]]; then
    log "Aborted."
    exit 0
  fi
  kubectl delete namespace "$NAMESPACE"
  log "Namespace deleted."
}

# ── Main ──────────────────────────────────────────────────────────────────────
case "$ACTION" in
  apply)    apply_all ;;
  status)   status_all ;;
  rollback) rollback "$@" ;;
  delete)   delete_all ;;
  *)        err "Unknown action: $ACTION. Use: apply | status | rollback | delete" ;;
esac
