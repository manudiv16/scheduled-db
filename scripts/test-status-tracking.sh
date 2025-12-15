#!/bin/bash
# Integration test for job status tracking

set -e

BASE_URL="${BASE_URL:-http://localhost:8080}"

echo "=== Job Status Tracking Integration Test ==="
echo "Testing against: $BASE_URL"
echo ""

# Test 1: Create a job
echo "Test 1: Creating a job..."
JOB_ID="test-job-$(date +%s)"
CREATE_RESPONSE=$(curl -s -X POST "$BASE_URL/jobs" \
  -H "Content-Type: application/json" \
  -d "{
    \"id\": \"$JOB_ID\",
    \"type\": \"unico\",
    \"webhook_url\": \"http://httpbin.org/post\",
    \"timestamp\": $(($(date +%s) + 3600))
  }")

echo "Created job: $CREATE_RESPONSE"
echo ""

# Test 2: Query job status (should be pending)
echo "Test 2: Querying job status..."
STATUS_RESPONSE=$(curl -s "$BASE_URL/jobs/$JOB_ID/status")
echo "Job status: $STATUS_RESPONSE"
echo ""

# Test 3: Query execution history (should be empty)
echo "Test 3: Querying execution history..."
HISTORY_RESPONSE=$(curl -s "$BASE_URL/jobs/$JOB_ID/executions")
echo "Execution history: $HISTORY_RESPONSE"
echo ""

# Test 4: List jobs by status (pending)
echo "Test 4: Listing pending jobs..."
LIST_RESPONSE=$(curl -s "$BASE_URL/jobs?status=pending")
echo "Pending jobs: $LIST_RESPONSE"
echo ""

# Test 5: Cancel the job
echo "Test 5: Cancelling job..."
CANCEL_RESPONSE=$(curl -s -X POST "$BASE_URL/jobs/$JOB_ID/cancel" \
  -H "Content-Type: application/json" \
  -d '{"reason": "Integration test"}')
echo "Cancel response: $CANCEL_RESPONSE"
echo ""

# Test 6: Verify job is cancelled
echo "Test 6: Verifying job is cancelled..."
STATUS_AFTER_CANCEL=$(curl -s "$BASE_URL/jobs/$JOB_ID/status")
echo "Job status after cancel: $STATUS_AFTER_CANCEL"
echo ""

# Test 7: List cancelled jobs
echo "Test 7: Listing cancelled jobs..."
CANCELLED_LIST=$(curl -s "$BASE_URL/jobs?status=cancelled")
echo "Cancelled jobs: $CANCELLED_LIST"
echo ""

# Test 8: Check health endpoint
echo "Test 8: Checking health endpoint..."
HEALTH_RESPONSE=$(curl -s "$BASE_URL/health")
echo "Health: $HEALTH_RESPONSE"
echo ""

# Test 9: Check metrics endpoint
echo "Test 9: Checking metrics endpoint..."
METRICS_RESPONSE=$(curl -s "$BASE_URL/metrics" | grep -E "scheduled_db_(jobs_by_status|execution_|status_query)")
echo "Status tracking metrics:"
echo "$METRICS_RESPONSE"
echo ""

echo "=== Integration Test Complete ==="
