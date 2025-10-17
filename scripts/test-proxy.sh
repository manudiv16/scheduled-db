#!/bin/bash

# Test script for proxy functionality in scheduled-db cluster
# This script tests that followers correctly proxy requests to the leader

set -e

echo "🧪 Testing Proxy Functionality"
echo "=============================="

# Auto-detect environment (Docker or Kubernetes)
detect_environment() {
    if kubectl get pods -l app=scheduled-db >/dev/null 2>&1; then
        echo "kubernetes"
    elif curl -s --connect-timeout 2 http://127.0.0.1:80/health >/dev/null 2>&1; then
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
    local future_time=$(date -u -d "+30 seconds" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -v+30S +%Y-%m-%dT%H:%M:%SZ 2>/dev/null)

    curl -s -X POST http://127.0.0.1:$port/jobs \
        -H "Content-Type: application/json" \
        -d "{\"type\":\"unico\", \"timestamp\":\"$future_time\"}" \
        -w "HTTP_CODE:%{http_code}"
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
echo "📊 Checking cluster status..."

# Find leader and followers
leader_port=""
follower_ports=()

for port in "${test_ports[@]}"; do
    if check_port $port; then
        role=$(get_role $port)
        echo "Node (port $port): ✅ $role"

        if [ "$role" = "leader" ]; then
            leader_port=$port
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

echo ""
echo "📍 Cluster Configuration:"
echo "Leader: 127.0.0.1:$leader_port"
if [ ${#follower_ports[@]} -gt 0 ]; then
    echo "Followers: ${follower_ports[*]/#/127.0.0.1:}"
else
    echo "Followers: None detected"
fi
echo ""

# Test 1: Direct request to leader
echo "🧪 Test 1: Direct request to leader"
echo "====================================="

echo "Creating job directly on leader (port $leader_port)..."
response=$(create_test_job $leader_port)
http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d':' -f2)
body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')

if [ "$http_code" = "201" ]; then
    job_id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
    echo "✅ Success: Job created with ID $job_id"
else
    echo "❌ Failed: HTTP $http_code"
    echo "Response: $body"
fi

# Test 2: Request to follower (proxy test)
if [ ${#follower_ports[@]} -gt 0 ]; then
    echo ""
    echo "🧪 Test 2: Proxy request via follower"
    echo "======================================"

    follower_port=${follower_ports[0]}
    echo "Creating job via follower (port $follower_port) - should proxy to leader..."
    response=$(create_test_job $follower_port)
    http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d':' -f2)
    body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')

    if [ "$http_code" = "201" ]; then
        job_id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        echo "✅ Success: Job created via proxy with ID $job_id"
    else
        echo "❌ Failed: HTTP $http_code"
        echo "Response: $body"
    fi
else
    echo ""
    echo "⚠️ No followers found - skipping proxy test"
fi

# Test 3: Load balancer (Docker only)
if [ "$ENVIRONMENT" = "docker" ] && [ -n "$lb_port" ]; then
    echo ""
    echo "🧪 Test 3: Load balancer request"
    echo "================================="

    echo "Creating job via nginx load balancer (port $lb_port)..."
    response=$(create_test_job $lb_port)
    http_code=$(echo "$response" | grep -o "HTTP_CODE:[0-9]*" | cut -d':' -f2)
    body=$(echo "$response" | sed 's/HTTP_CODE:[0-9]*$//')

    if [ "$http_code" = "201" ]; then
        job_id=$(echo "$body" | grep -o '"id":"[^"]*"' | cut -d'"' -f4)
        echo "✅ Success: Job created via load balancer with ID $job_id"
    else
        echo "❌ Failed: HTTP $http_code"
        echo "Response: $body"
    fi
fi

echo ""
echo "🎯 Proxy functionality test completed!"
echo ""
echo "💡 To monitor job execution:"
if [ "$ENVIRONMENT" = "docker" ]; then
    echo "   docker logs scheduled-db-node-1 -f | grep 'Executing job'"
else
    echo "   kubectl logs -l app=scheduled-db -f | grep 'Executing job'"
fi
