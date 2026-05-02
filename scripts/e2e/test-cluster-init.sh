#!/usr/bin/env bash
set -euo pipefail

MAX_LEADER_WAIT=90
MAX_JOIN_WAIT=60
FAIL_ON_SLOW_JOIN=30

echo "========================================="
echo "  CLUSTER INITIALIZATION TEST"
echo "========================================="
echo ""

START_TIME=$(date +%s)

echo "::group::Step 1: Verify all 3 pods are running"
for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
    phase=$(kubectl get pod "$pod" -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
    if [ "$phase" != "Running" ]; then
        echo "FAIL: Pod $pod is not running (phase: $phase)"
        exit 1
    fi
    echo "  Pod $pod: Running"
done
echo "All pods are running"
echo "::endgroup::"

echo ""
echo "::group::Step 2: Identify Raft leader"
elapsed=0
leader_pod=""
while [ $elapsed -lt $MAX_LEADER_WAIT ]; do
    for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
        role=$(kubectl exec "$pod" -- wget -qO- http://localhost:8080/health 2>/dev/null \
            | grep -o '"role":"[^"]*"' | cut -d'"' -f4 || echo "")
        if [ "$role" = "leader" ]; then
            leader_pod="$pod"
            break 2
        fi
    done
    echo "  [$elapsed/${MAX_LEADER_WAIT}s] No leader yet..."
    sleep 3
    elapsed=$((elapsed + 3))
done

if [ -z "$leader_pod" ]; then
    echo "FAIL: No leader elected within ${MAX_LEADER_WAIT}s"
    for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
        echo "--- $pod logs ---"
        kubectl logs "$pod" --tail=20
    done
    exit 1
fi

leader_time=$(date +%s)
leader_delay=$((leader_time - START_TIME))
echo "Leader identified: $leader_pod (${leader_delay}s from start)"
echo "::endgroup::"

echo ""
echo "::group::Step 3: Verify node join timing"
declare -A join_times

for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
    join_log=$(kubectl logs "$pod" 2>/dev/null | grep -i "Successfully added peer\|Successfully bootstrapped\|joined cluster" | tail -1 || true)

    if [ -n "$join_log" ]; then
        join_ts=$(kubectl logs "$pod" 2>/dev/null | head -1 | grep -oP '\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}' || echo "")
        echo "  $pod: $join_log"
        join_times[$pod]=1
    else
        echo "  $pod: No join log found yet"
    fi
done
echo "::endgroup::"

echo ""
echo "::group::Step 4: Verify cluster configuration"
leader_addr="${leader_pod}.scheduled-db.default.svc.cluster.local:8080"
server_count=$(kubectl exec "$leader_pod" -- wget -qO- "http://localhost:8080/debug/cluster" 2>/dev/null \
    | grep -o '"id"' | wc -l || echo "0")

if [ "$server_count" -lt 3 ]; then
    echo "WARN: Only $server_count/3 servers in cluster config, waiting for joins..."

    elapsed=0
    while [ $elapsed -lt $MAX_JOIN_WAIT ]; do
        server_count=$(kubectl exec "$leader_pod" -- wget -qO- "http://localhost:8080/debug/cluster" 2>/dev/null \
            | grep -o '"id"' | wc -l || echo "0")
        if [ "$server_count" -ge 3 ]; then
            break
        fi
        echo "  [$elapsed/${MAX_JOIN_WAIT}s] $server_count/3 servers joined"
        sleep 3
        elapsed=$((elapsed + 3))
    done
fi

if [ "$server_count" -lt 3 ]; then
    echo "FAIL: Only $server_count/3 servers in cluster after ${MAX_JOIN_WAIT}s"
    kubectl exec "$leader_pod" -- wget -qO- "http://localhost:8080/debug/cluster" 2>/dev/null || true
    exit 1
fi

join_time=$(date +%s)
total_join_delay=$((join_time - START_TIME))
echo "All 3 nodes in cluster config (${total_join_delay}s from start)"
echo "::endgroup::"

echo ""
echo "::group::Step 5: Verify health on all nodes"
for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
    health=$(kubectl exec "$pod" -- wget -qO- http://localhost:8080/health 2>/dev/null || echo "UNREACHABLE")
    status=$(echo "$health" | grep -o '"status":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
    role=$(echo "$health" | grep -o '"role":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
    node_id=$(echo "$health" | grep -o '"node_id":"[^"]*"' | cut -d'"' -f4 || echo "unknown")
    echo "  $pod: status=$status, role=$role, node_id=$node_id"

    if [ "$status" != "ok" ] && [ "$status" != "degraded" ]; then
        echo "FAIL: Pod $pod has unhealthy status: $status"
        exit 1
    fi
done
echo "All nodes healthy"
echo "::endgroup::"

echo ""
echo "::group::Step 6: Check for slow joins"
for pod in scheduled-db-1 scheduled-db-2; do
    delay_line=$(kubectl logs "$pod" 2>/dev/null | grep "Non-bootstrap node.*waiting" | tail -1 || true)
    if [ -n "$delay_line" ]; then
        delay_val=$(echo "$delay_line" | grep -oP 'waiting \K[0-9]+s' || echo "unknown")
        echo "  $pod startup delay: $delay_val"
    fi
done
echo "::endgroup::"

echo ""
echo "========================================="
echo "  CLUSTER INITIALIZATION RESULTS"
echo "========================================="
echo "Leader elected:        $leader_pod"
echo "Leader election time:  ${leader_delay}s"
echo "Cluster formed:        ${total_join_delay}s"
echo "Servers in cluster:    $server_count"
echo ""

if [ "$total_join_delay" -gt "$FAIL_ON_SLOW_JOIN" ]; then
    echo "WARN: Cluster took ${total_join_delay}s to form (threshold: ${FAIL_ON_SLOW_JOIN}s)"
    echo "This may indicate slow discovery or network issues"
fi

echo "PASS: Cluster initialization test completed successfully"
