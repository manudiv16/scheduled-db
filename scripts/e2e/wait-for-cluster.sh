#!/usr/bin/env bash
set -euo pipefail

POD_COUNT="${1:-3}"
MAX_WAIT="${2:-300}"

echo "Waiting for $POD_COUNT scheduled-db pods to be ready (max ${MAX_WAIT}s)..."

elapsed=0
while [ $elapsed -lt $MAX_WAIT ]; do
    ready=$(kubectl get pods -l app=scheduled-db \
        -o jsonpath='{.items[?(@.status.phase=="Running")].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null \
        | grep -c "True" || true)

    if [ "$ready" -ge "$POD_COUNT" ]; then
        echo "All $POD_COUNT pods are ready after ${elapsed}s"
        kubectl get pods -l app=scheduled-db -o wide
        exit 0
    fi

    echo "  [$elapsed/${MAX_WAIT}s] $ready/$POD_COUNT pods ready"
    sleep 5
    elapsed=$((elapsed + 5))
done

echo "ERROR: Timeout waiting for pods to be ready after ${MAX_WAIT}s"
kubectl get pods -l app=scheduled-db -o wide
for pod in $(kubectl get pods -l app=scheduled-db -o jsonpath='{.items[*].metadata.name}'); do
    echo "--- $pod ---"
    kubectl logs "$pod" --tail=30
done
exit 1
