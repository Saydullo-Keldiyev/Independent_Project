#!/usr/bin/env bash
# Update image newTag in a Kustomize overlay (used by GitHub Actions)
# Usage: ./scripts/gitops-update-image.sh <service-name> <overlay-env> <git-sha> <github-org>
set -euo pipefail

SERVICE="${1:?service name required}"
ENV="${2:?overlay env required (dev|staging|prod)}"
SHA="${3:?git sha required}"
ORG="${4:?github org required}"

NEW_TAG="sha-${SHA:0:7}"
FILE="deployments/k8s/overlays/${ENV}/kustomization.yaml"

if [[ ! -f "$FILE" ]]; then
  echo "Overlay not found: $FILE" >&2
  exit 1
fi

sed -i "s|Saydullo-Keldiyev|${ORG}|g" "$FILE"

# Update newTag for the service (portable sed — runs on GitHub ubuntu)
awk -v svc="$SERVICE" -v tag="$NEW_TAG" '
  $0 ~ "name: ghcr.io/[^/]+/" svc "$" { inblock=1 }
  inblock && /newTag:/ { sub(/newTag: .*/, "newTag: " tag); inblock=0 }
  { print }
' "$FILE" > "${FILE}.tmp" && mv "${FILE}.tmp" "$FILE"

echo "Updated ${SERVICE} → ${NEW_TAG} in ${FILE}"
