#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="${KIND_CLUSTER_NAME:-scheduled-db-e2e}"
KIND_CONFIG="${SCRIPT_DIR}/../../k8s/kind-config.yaml"

echo "::group::Creating Kind cluster"
if kind get clusters 2>/dev/null | grep -q "$CLUSTER_NAME"; then
    echo "Kind cluster '$CLUSTER_NAME' already exists, deleting..."
    kind delete cluster --name "$CLUSTER_NAME"
fi

kind create cluster \
    --config "$KIND_CONFIG" \
    --name "$CLUSTER_NAME" \
    --wait 120s

echo "Kind cluster created successfully"
echo "::endgroup::"

echo "::group::Verifying cluster"
kubectl get nodes
echo "::endgroup::"
