#!/usr/bin/env bash
set -euo pipefail

METRICS_FILE="${GITHUB_STEP_SUMMARY:-/dev/stdout}"

{
    echo "## Cluster E2E Test Metrics"
    echo ""
    echo "| Metric | Value |"
    echo "|--------|-------|"
} >> "$METRICS_FILE"

pod_ready_times=()
for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
    ready_at=$(kubectl get pod "$pod" -o jsonpath='{.status.conditions[?(@.type=="Ready")].lastTransitionTime}' 2>/dev/null || echo "N/A")
    echo "| $pod ready | $ready_at |" >> "$METRICS_FILE"
done

echo "" >> "$METRICS_FILE"
echo "### Raft Cluster State" >> "$METRICS_FILE"
echo "" >> "$METRICS_FILE"

for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
    role=$(kubectl exec "$pod" -- wget -qO- http://localhost:8080/health 2>/dev/null \
        | grep -o '"role":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
    node_id=$(kubectl exec "$pod" -- wget -qO- http://localhost:8080/health 2>/dev/null \
        | grep -o '"node_id":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
    leader=$(kubectl exec "$pod" -- wget -qO- http://localhost:8080/health 2>/dev/null \
        | grep -o '"leader":"[^"]*"' | cut -d'"' -f4 || echo "")
    echo "| $pod | role=$role, leader=$leader |" >> "$METRICS_FILE"
done

echo "" >> "$METRICS_FILE"
echo "### Cluster Configuration" >> "$METRICS_FILE"
echo "" >> "$METRICS_FILE"

leader_pod=""
for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
    role=$(kubectl exec "$pod" -- wget -qO- http://localhost:8080/health 2>/dev/null \
        | grep -o '"role":"[^"]*"' | cut -d'"' -f4 || echo "")
    if [ "$role" = "leader" ]; then
        leader_pod="$pod"
        break
    fi
done

if [ -n "$leader_pod" ]; then
    cluster_debug=$(kubectl exec "$leader_pod" -- wget -qO- "http://localhost:8080/debug/cluster" 2>/dev/null || echo "{}")
    echo '```json' >> "$METRICS_FILE"
    echo "$cluster_debug" >> "$METRICS_FILE"
    echo '```' >> "$METRICS_FILE"
fi

echo "" >> "$METRICS_FILE"
echo "### Pod Resource Usage" >> "$METRICS_FILE"
echo "" >> "$METRICS_FILE"
kubectl get pods -l app=scheduled-db -o wide >> "$METRICS_FILE" 2>/dev/null || true

echo "Metrics written to summary"
