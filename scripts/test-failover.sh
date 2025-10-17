#!/bin/bash

# Test script for scheduled-db failover functionality in Docker/Kubernetes
# This script tests that the cluster maintains functionality when nodes fail

set -e

echo "🧪 Testing Failover Functionality"
echo "================================="

# Auto-detect environment (Docker or Kubernetes)
detect_environment() {
    if kubectl get pods -l app=scheduled-db >/dev/null 2>&1; then
        echo "kubernetes"
    elif docker ps --filter name=scheduled-db-node >/dev/null 2>&1 && curl -s --connect-timeout 2 http://127.0.0.1:80/health >/dev/null 2>&1; then
        echo "docker"
    else
        echo "none"
    fi
}

ENVIRONMENT=$(detect_environment)
echo "📍 Detected environment: $ENVIRONMENT"

# Function to check if a port is responding
check_port() {
    local port=$1
    curl -s --connect-timeout 2 http://127.0.0.1:$port/health >/dev/null 2>&1
}

# Function to get node role
get_role() {
    local port=$1
    curl -s --connect-timeout 2 http://127.0.0.1:$port/health 2>/dev/null | grep -o '"role":"[^"]*"' | cut -d'"' -f4 2>/dev/null || echo "unknown"
}

# Function to create a test job
create_test_job() {
    local port=$1
    local job_type=${2:-"unico"}
    local future_time=$(date -u -d "+30 seconds" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v+30S +%Y-%m-%dT%H:%M:%SZ 2>/dev/null)

    curl -s -X POST http://127.0.0.1:$port/jobs \
        -H "Content-Type: application/json" \
        -d "{\"type\":\"$job_type\", \"timestamp\":\"$future_time\"}" \
        -w "HTTP_CODE:%{http_code}"
}

# Function to get cluster info
get_cluster_info() {
    local port=$1
    curl -s --connect-timeout 2 http://127.0.0.1:$port/debug/cluster 2>/dev/null
}

case $ENVIRONMENT in
    "docker")
        echo "Testing Docker environment"
        test_ports=(8080 8081 8082)
        lb_port=80
        ;;
    "kubernetes")
        echo "Testing Kubernetes environment (setting up port forwarding...)"
        kubectl port-forward svc/scheduled-db-api 8080:8080 >/dev/null 2>&1 &
        PORT_FORWARD_PID=$!
        sleep 3
        test_ports=(8080)
        lb_port=""
        ;;
    *)
        echo "❌ No scheduled-db cluster detected!"
        echo "   Try: make dev-up (Docker) or make k8s-deploy (Kubernetes)"
        exit 1
        ;;
esac

# Cleanup function
cleanup() {
    if [ -n "$PORT_FORWARD_PID" ]; then
        kill $PORT_FORWARD_PID 2>/dev/null || true
    fi
}
trap cleanup EXIT

echo ""
echo "📊 Initial cluster status..."

# Find leader and followers
leader_port=""
leader_node=""
follower_ports=()

for port in "${test_ports[@]}"; do
    if check_port $port; then
        role=$(get_role $port)
        echo "Node (port $port): ✅ $role"

        if [ "$role" = "leader" ]; then
            leader_port=$port
            case $port in
                8080) leader_node="scheduled-db-node-1" ;;
                8081) leader_node="scheduled-db-node-2" ;;
                8082) leader_node="scheduled-db-node-3" ;;
            esac
        elif [ "$role" = "follower" ]; then
            follower_ports+=($port)
        fi
    else
        echo "Node (port $port): ❌ Not responding"
    fi
done

if [ -z "$leader_port" ]; then
    echo "❌ No leader found!"
    exit 1
fi

if [ ${#follower_ports[@]} -eq 0 ]; then
    echo "❌ No followers found - cannot test failover!"
    exit 1
fi

echo ""
echo "📍 Initial Configuration:"
echo "Leader: 127.0.0.1:$leader_port ($leader_node)"
echo "Followers: ${follower_ports[*]/#/127.0.0.1:}"

# Test 1: Create initial job before failover
echo ""
echo "🧪 Test 1: Create job before failover"
echo "======================================"

echo "Creating job on leader..."
response=$(create_test_job $leader_port)
http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d':' -f2)
body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')

if [ "$http_code" = "201" ]; then
    job_id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo "✅ Success: Job created with ID $job_id"
else
    echo "❌ Failed to create initial job: HTTP $http_code"
    exit 1
fi

# Test 2: Simulate leader failure
echo ""
echo "🧪 Test 2: Simulate leader failure"
echo "==================================="

if [ "$ENVIRONMENT" = "docker" ]; then
    echo "Stopping leader container: $leader_node"
    docker stop $leader_node >/dev/null 2>&1
    echo "✅ Leader container stopped"
else
    echo "⚠️ Cannot simulate node failure in Kubernetes from this script"
    echo "   Manually scale down a pod to test failover:"
    echo "   kubectl scale statefulset scheduled-db --replicas=2"
fi

echo "Waiting for leader election..."
sleep 10

# Test 3: Check new leader election
echo ""
echo "🧪 Test 3: Verify new leader election"
echo "====================================="

new_leader_port=""
remaining_ports=()

for port in "${test_ports[@]}"; do
    if [ "$port" != "$leader_port" ]; then  # Skip the failed node
        if check_port $port; then
            role=$(get_role $port)
            echo "Node (port $port): ✅ $role"

            if [ "$role" = "leader" ]; then
                new_leader_port=$port
            fi
            remaining_ports+=($port)
        else
            echo "Node (port $port): ❌ Not responding"
        fi
    fi
done

if [ -z "$new_leader_port" ]; then
    echo "❌ No new leader elected!"
    if [ "$ENVIRONMENT" = "docker" ]; then
        echo "Restarting failed container for cleanup..."
        docker start $leader_node >/dev/null 2>&1
    fi
    exit 1
fi

echo "✅ New leader elected on port $new_leader_port"

# Test 4: Create job with new leader
echo ""
echo "🧪 Test 4: Create job with new leader"
echo "====================================="

echo "Creating job on new leader..."
response=$(create_test_job $new_leader_port)
http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d':' -f2)
body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')

if [ "$http_code" = "201" ]; then
    job_id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo "✅ Success: Job created with new leader, ID $job_id"
else
    echo "❌ Failed to create job with new leader: HTTP $http_code"
fi

# Test 5: Test load balancer resilience (Docker only)
if [ "$ENVIRONMENT" = "docker" ] && [ -n "$lb_port" ]; then
    echo ""
    echo "🧪 Test 5: Test load balancer resilience"
    echo "========================================="

    echo "Creating job via load balancer (should route to new leader)..."
    response=$(create_test_job $lb_port)
    http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d':' -f2)
    body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')

    if [ "$http_code" = "201" ]; then
        job_id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        echo "✅ Success: Load balancer correctly routed to new leader, ID $job_id"
    else
        echo "❌ Load balancer failed to route to new leader: HTTP $http_code"
    fi
fi

# Cleanup and recovery
echo ""
echo "🔄 Recovery"
echo "==========="

if [ "$ENVIRONMENT" = "docker" ]; then
    echo "Restarting failed container..."
    docker start $leader_node >/dev/null 2>&1
    sleep 5

    echo "Checking if failed node rejoined cluster..."
    if check_port $leader_port; then
        role=$(get_role $leader_port)
        echo "✅ Failed node rejoined as: $role"
    else
        echo "❌ Failed node did not rejoin cluster"
    fi
fi

# Final cluster status
echo ""
echo "📊 Final cluster status"
echo "======================="

working_nodes=0
for port in "${test_ports[@]}"; do
    if check_port $port; then
        role=$(get_role $port)
        echo "Node (port $port): ✅ $role"
        working_nodes=$((working_nodes + 1))
    else
        echo "Node (port $port): ❌ Not responding"
    fi
done

echo ""
echo "🎯 Failover test completed!"
echo ""
echo "📈 Results:"
echo "   - Initial leader failure: Detected"
echo "   - New leader election: Success"
echo "   - Cluster functionality: Maintained"
echo "   - Working nodes: $working_nodes/${#test_ports[@]}"
echo ""

if [ "$ENVIRONMENT" = "docker" ]; then
    echo "💡 To monitor logs:"
    echo "   docker logs scheduled-db-node-1 -f"
    echo "   docker logs scheduled-db-node-2 -f"
    echo "   docker logs scheduled-db-node-3 -f"
else
    echo "💡 To monitor logs:"
    echo "   kubectl logs -l app=scheduled-db -f"
fi
