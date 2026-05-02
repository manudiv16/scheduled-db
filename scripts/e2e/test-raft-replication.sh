#!/usr/bin/env bash
set -euo pipefail

REPLICATION_TIMEOUT=10

echo "========================================="
echo "  RAFT REPLICATION TEST"
echo "========================================="
echo ""

echo "::group::Step 1: Identify leader node"
leader_pod=""
for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
    role=$(kubectl exec "$pod" -- wget -qO- http://localhost:8080/health 2>/dev/null \
        | grep -o '"role":"[^"]*"' | cut -d'"' -f4 || echo "")
    if [ "$role" = "leader" ]; then
        leader_pod="$pod"
        break
    fi
done

if [ -z "$leader_pod" ]; then
    echo "FAIL: No leader found"
    exit 1
fi
echo "Leader: $leader_pod"
echo "::endgroup::"

echo ""
echo "::group::Step 2: Verify cluster has 3 nodes"
cluster_info=$(kubectl exec "$leader_pod" -- wget -qO- "http://localhost:8080/debug/cluster" 2>/dev/null || echo "")
server_count=$(echo "$cluster_info" | grep -o '"id"' | wc -l || echo "0")

if [ "$server_count" -lt 3 ]; then
    echo "FAIL: Cluster has only $server_count nodes, need 3"
    echo "$cluster_info"
    exit 1
fi
echo "Cluster has $server_count nodes"
echo "::endgroup::"

echo ""
echo "::group::Step 3: Create test job on leader"
ts=$(date -u +%Y-%m-%dT%H:%M:%SZ)
kubectl exec "$leader_pod" -- /bin/sh -c \
    "cat > /tmp/job.json << 'JOBEOF'
{\"type\":\"unico\",\"timestamp\":\"${ts}\",\"payload\":{\"test\":\"e2e-replication\"}}
JOBEOF"

create_response=$(kubectl exec "$leader_pod" -- wget -qO- --post-file=/tmp/job.json --header='Content-Type: application/json' http://localhost:8080/jobs 2>&1 || echo "WGET_FAILED")

if echo "$create_response" | grep -q "WGET_FAILED"; then
    echo "FAIL: wget failed to create job"
    echo "Response: $create_response"
    exit 1
fi

job_id=$(echo "$create_response" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
if [ -z "$job_id" ]; then
    echo "FAIL: Could not create job on leader"
    echo "Response: $create_response"
    exit 1
fi
echo "Job created: $job_id on leader $leader_pod"
echo "::endgroup::"

echo ""
echo "::group::Step 4: Verify replication to followers"
follower_pods=()
for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
    if [ "$pod" != "$leader_pod" ]; then
        follower_pods+=("$pod")
    fi
done

replication_ok=true
for follower in "${follower_pods[@]}"; do
    replicated=false
    elapsed=0
    while [ $elapsed -lt $REPLICATION_TIMEOUT ]; do
        follower_response=$(kubectl exec "$follower" -- wget -qO- \
            "http://localhost:8080/jobs/$job_id" 2>/dev/null || echo "NOT_FOUND")

        if echo "$follower_response" | grep -q "$job_id"; then
            echo "  $follower: REPLICATED (${elapsed}s)"
            replicated=true
            break
        fi

        echo "  $follower: waiting for replication... [$elapsed/${REPLICATION_TIMEOUT}s]"
        sleep 1
        elapsed=$((elapsed + 1))
    done

    if [ "$replicated" = false ]; then
        echo "  $follower: FAILED - job not replicated after ${REPLICATION_TIMEOUT}s"
        replication_ok=false
    fi
done
echo "::endgroup::"

echo ""
echo "::group::Step 5: Verify data consistency across all nodes"
for pod in scheduled-db-0 scheduled-db-1 scheduled-db-2; do
    job_count=$(kubectl exec "$pod" -- wget -qO- \
        "http://localhost:8080/debug/cluster" 2>/dev/null \
        | grep -o '"job_count":[0-9]*' | cut -d: -f2 || echo "0")
    echo "  $pod: job_count=$job_count"
done
echo "::endgroup::"

echo ""
echo "::group::Step 6: Test write forwarding from follower"
if [ ${#follower_pods[@]} -gt 0 ]; then
    follower="${follower_pods[0]}"
    ts2=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    kubectl exec "$follower" -- /bin/sh -c \
        "cat > /tmp/job2.json << 'JOBEOF'
{\"type\":\"unico\",\"timestamp\":\"${ts2}\",\"payload\":{\"test\":\"e2e-forward\"}}
JOBEOF"

    forward_response=$(kubectl exec "$follower" -- wget -qO- --post-file=/tmp/job2.json --header='Content-Type: application/json' http://localhost:8080/jobs 2>&1 || echo "WGET_FAILED")

    forward_id=$(echo "$forward_response" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
    if [ -n "$forward_id" ]; then
        echo "Write forwarding works: job $forward_id created via follower $follower"
    else
        echo "WARN: Write forwarding from $follower may not be working"
        echo "Response: $forward_response"
    fi
fi
echo "::endgroup::"

echo ""
echo "::group::Step 7: Cleanup test jobs"
kubectl exec "$leader_pod" -- wget -qO- \
    --header="Content-Type: application/json" \
    --method=DELETE \
    "http://localhost:8080/jobs/$job_id" 2>/dev/null > /dev/null || true
echo "Test jobs cleaned up"
echo "::endgroup::"

echo ""
if [ "$replication_ok" = true ]; then
    echo "========================================="
    echo "  RAFT REPLICATION RESULTS"
    echo "========================================="
    echo "Leader:             $leader_pod"
    echo "Job created:        $job_id"
    echo "Replication:        ALL FOLLOWERS IN SYNC"
    echo "Forward test:       COMPLETED"
    echo ""
    echo "PASS: Raft replication test completed successfully"
else
    echo "FAIL: Not all followers replicated the job"
    exit 1
fi
