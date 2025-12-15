# API Reference

## Base URL

```
http://<node-address>:8080
```

## Authentication

Currently, the API does not require authentication. For production deployments, consider adding:
- API keys
- JWT tokens
- mTLS
- Network policies

## Endpoints

### Job Management

#### Create Job

Create a new scheduled job.

**Endpoint:** `POST /jobs`

**Request Body:**

```json
{
  "id": "optional-custom-id",
  "type": "unico|recurrente",
  "timestamp": "RFC3339 or epoch seconds",
  "cron_expression": "cron format (for recurrente)",
  "last_date": "RFC3339 or epoch seconds (optional)",
  "webhook_url": "https://example.com/webhook (optional)",
  "payload": {
    "custom": "data"
  }
}
```

**Response:** `200 OK`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "unico",
  "timestamp": 1704067200,
  "created_at": 1704060000,
  "webhook_url": "https://example.com/webhook",
  "payload": {
    "custom": "data"
  }
}
```

**Examples:**

```bash
# One-time job
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "unico",
    "timestamp": "2024-12-25T10:00:00Z",
    "webhook_url": "https://example.com/webhook"
  }'

# Recurring job (daily at midnight)
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "recurrente",
    "cron_expression": "0 0 * * *"
  }'

# Recurring job with end date
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "recurrente",
    "cron_expression": "0 9 * * 1-5",
    "last_date": "2024-12-31T23:59:59Z"
  }'
```

**Error Responses:**

```json
// 400 Bad Request - Invalid input
{
  "error": "timestamp is required for unico jobs"
}

// 500 Internal Server Error - Server error
{
  "error": "failed to create job: raft error"
}

// 503 Service Unavailable - No leader
{
  "error": "No leader available"
}

// 507 Insufficient Storage - Queue full
{
  "error": "insufficient memory: current=1000, limit=1000",
  "type": "memory",
  "current": 1000,
  "limit": 1000,
  "requested": 100
}
```

---

#### Get Job

Retrieve job details by ID.

**Endpoint:** `GET /jobs/{id}`

**Response:** `200 OK`

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "unico",
  "timestamp": 1704067200,
  "created_at": 1704060000,
  "webhook_url": "https://example.com/webhook",
  "payload": {
    "custom": "data"
  }
}
```

**Example:**

```bash
curl http://localhost:8080/jobs/550e8400-e29b-41d4-a716-446655440000
```

**Error Responses:**

```json
// 404 Not Found
{
  "error": "Job not found"
}

// 400 Bad Request
{
  "error": "Job ID is required"
}
```

---

#### Delete Job

Delete a scheduled job.

**Endpoint:** `DELETE /jobs/{id}`

**Response:** `200 OK` (empty body)

**Example:**

```bash
curl -X DELETE http://localhost:8080/jobs/550e8400-e29b-41d4-a716-446655440000
```

**Error Responses:**

```json
// 404 Not Found
{
  "error": "Job not found"
}

// 500 Internal Server Error
{
  "error": "Failed to delete job: raft error"
}
```

---

### Job Status Tracking

#### Get Job Status

Get the current execution status of a job.

**Endpoint:** `GET /jobs/{id}/status`

**Response:** `200 OK`

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "attempt_count": 1,
  "created_at": 1704060000,
  "first_attempt_at": 1704067200,
  "last_attempt_at": 1704067200,
  "completed_at": 1704067201,
  "executing_node_id": "node-1"
}
```

**Status Values:**
- `pending` - Job created, not yet executed
- `in_progress` - Currently executing
- `completed` - Successfully executed
- `failed` - Execution failed
- `cancelled` - Cancelled by user
- `timeout` - Execution timed out

**Example:**

```bash
curl http://localhost:8080/jobs/550e8400-e29b-41d4-a716-446655440000/status | jq
```

**Error Responses:**

```json
// 404 Not Found
{
  "error": "Job not found"
}
```

---

#### Get Job Execution History

Get detailed execution history with all attempts.

**Endpoint:** `GET /jobs/{id}/executions`

**Response:** `200 OK`

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "attempts": [
    {
      "attempt_num": 1,
      "start_time": 1704067200,
      "end_time": 1704067201,
      "node_id": "node-1",
      "status": "failed",
      "http_status": 0,
      "response_time_ms": 5000,
      "error_message": "context deadline exceeded",
      "error_type": "timeout"
    },
    {
      "attempt_num": 2,
      "start_time": 1704067500,
      "end_time": 1704067501,
      "node_id": "node-1",
      "status": "completed",
      "http_status": 200,
      "response_time_ms": 234,
      "error_message": "",
      "error_type": ""
    }
  ]
}
```

**Example:**

```bash
curl http://localhost:8080/jobs/550e8400-e29b-41d4-a716-446655440000/executions | jq
```

**Error Responses:**

```json
// 404 Not Found
{
  "error": "Job not found"
}
```

---

#### List Jobs by Status

Filter jobs by execution status.

**Endpoint:** `GET /jobs?status={status}`

**Query Parameters:**
- `status` - Filter by status (pending, in_progress, completed, failed, cancelled, timeout)

**Response:** `200 OK`

```json
{
  "jobs": [
    {
      "job_id": "550e8400-e29b-41d4-a716-446655440000",
      "status": "completed",
      "attempt_count": 1,
      "created_at": 1704060000,
      "completed_at": 1704067201
    },
    {
      "job_id": "660e8400-e29b-41d4-a716-446655440001",
      "status": "completed",
      "attempt_count": 2,
      "created_at": 1704060100,
      "completed_at": 1704067301
    }
  ],
  "total": 2
}
```

**Examples:**

```bash
# List completed jobs
curl http://localhost:8080/jobs?status=completed | jq

# List failed jobs
curl http://localhost:8080/jobs?status=failed | jq

# List jobs in progress
curl http://localhost:8080/jobs?status=in_progress | jq
```

**Error Responses:**

```json
// 400 Bad Request - Invalid status
{
  "error": "Invalid status value"
}
```

---

#### Cancel Job

Cancel a pending or in-progress job.

**Endpoint:** `POST /jobs/{id}/cancel`

**Request Body:**

```json
{
  "reason": "No longer needed"
}
```

**Response:** `200 OK`

```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "cancelled",
  "cancelled_at": 1704067300
}
```

**Examples:**

```bash
# Cancel with reason
curl -X POST http://localhost:8080/jobs/550e8400-e29b-41d4-a716-446655440000/cancel \
  -H "Content-Type: application/json" \
  -d '{"reason": "Requirements changed"}'

# Cancel without reason
curl -X POST http://localhost:8080/jobs/550e8400-e29b-41d4-a716-446655440000/cancel \
  -H "Content-Type: application/json" \
  -d '{}'
```

**Error Responses:**

```json
// 404 Not Found
{
  "error": "Job not found"
}

// 400 Bad Request - Already completed
{
  "error": "Cannot cancel completed job"
}
```

---

### Health and Cluster

#### Health Check

Get node health and cluster role.

**Endpoint:** `GET /health`

**Response:** `200 OK`

```json
{
  "status": "healthy",
  "role": "leader",
  "node_id": "node-1",
  "leader": "",
  "jobs_by_status": {
    "pending": 10,
    "in_progress": 2,
    "completed": 100,
    "failed": 5,
    "cancelled": 1,
    "timeout": 0
  },
  "jobs_executing": 2,
  "last_execution_at": 1704067200,
  "timed_out_jobs": 0
}
```

For follower nodes:

```json
{
  "status": "healthy",
  "role": "follower",
  "node_id": "node-2",
  "leader": "node-1:7000",
  "jobs_by_status": {
    "pending": 10,
    "in_progress": 2,
    "completed": 100,
    "failed": 5,
    "cancelled": 1,
    "timeout": 0
  },
  "jobs_executing": 2,
  "last_execution_at": 1704067200,
  "timed_out_jobs": 0
}
```

**Health Status Values:**
- `healthy` - System operating normally
- `degraded` - Some failures but still operational (or memory utilization > 90%)
- `unhealthy` - High failure rate or critical issues

**Response with Capacity Information:**

```json
{
  "status": "degraded",
  "role": "leader",
  "node_id": "node-1",
  "leader": "",
  "memory": {
    "current_bytes": 920000000,
    "limit_bytes": 1000000000,
    "available_bytes": 80000000,
    "utilization_percent": 92.0
  },
  "jobs": {
    "current_count": 95000,
    "limit": 100000,
    "available": 5000
  },
  "jobs_by_status": {
    "pending": 10,
    "in_progress": 2,
    "completed": 100,
    "failed": 5,
    "cancelled": 1,
    "timeout": 0
  },
  "jobs_executing": 2,
  "last_execution_at": 1704067200,
  "timed_out_jobs": 0
}
```

**Example:**

```bash
curl http://localhost:8080/health | jq
```

---

#### Cluster Debug

Get detailed cluster information (for debugging).

**Endpoint:** `GET /debug/cluster`

**Response:** `200 OK`

```json
{
  "node_id": "node-1",
  "is_leader": true,
  "leader": "node-1:7000",
  "raft_state": "Leader",
  "servers": [
    {
      "id": "node-1",
      "address": "node-1:7000"
    },
    {
      "id": "node-2",
      "address": "node-2:7000"
    },
    {
      "id": "node-3",
      "address": "node-3:7000"
    }
  ],
  "job_count": 42
}
```

**Example:**

```bash
curl http://localhost:8080/debug/cluster | jq
```

---

#### Join Cluster

Manually add a node to the cluster (typically handled by service discovery).

**Endpoint:** `POST /join`

**Request Body:**

```json
{
  "node_id": "node-4",
  "address": "node-4:7000"
}
```

**Response:** `200 OK`

```json
{
  "success": true,
  "message": "Node node-4 successfully joined cluster"
}
```

**Example:**

```bash
curl -X POST http://localhost:8080/join \
  -H "Content-Type: application/json" \
  -d '{
    "node_id": "node-4",
    "address": "node-4:7000"
  }'
```

**Error Responses:**

```json
// 403 Forbidden - Not leader
{
  "error": "not leader, cannot accept join requests"
}

// 400 Bad Request
{
  "error": "node_id and address are required"
}
```

---

## Data Types

### Job Types

#### Unico (One-Time)

Executes once at the specified timestamp.

**Required Fields:**
- `type`: `"unico"`
- `timestamp`: Future timestamp (RFC3339 or epoch)

**Optional Fields:**
- `id`: Custom job ID (auto-generated if not provided)
- `webhook_url`: URL to call when job executes
- `payload`: Custom data to include in webhook

**Example:**

```json
{
  "type": "unico",
  "timestamp": "2024-12-25T10:00:00Z",
  "webhook_url": "https://example.com/webhook",
  "payload": {
    "event": "christmas",
    "year": 2024
  }
}
```

#### Recurrente (Recurring)

Executes repeatedly based on cron expression.

**Required Fields:**
- `type`: `"recurrente"`
- `cron_expression`: Valid cron expression

**Optional Fields:**
- `id`: Custom job ID
- `timestamp`: First execution time (defaults to now)
- `last_date`: Stop executing after this date
- `webhook_url`: URL to call on each execution
- `payload`: Custom data

**Example:**

```json
{
  "type": "recurrente",
  "cron_expression": "0 9 * * 1-5",
  "last_date": "2024-12-31T23:59:59Z",
  "webhook_url": "https://example.com/daily-report"
}
```

### Timestamp Formats

The API accepts multiple timestamp formats:

#### RFC3339 (Recommended)

```
2024-12-25T10:00:00Z
2024-12-25T10:00:00+02:00
2024-12-25T10:00:00-05:00
```

#### Epoch Seconds

```
1704067200
```

#### Simple DateTime

```
2024-12-25 10:00:00
```

### Cron Expression Format

Standard cron format with 5 fields:

```
* * * * *
│ │ │ │ │
│ │ │ │ └─── Day of week (0-6, Sunday=0)
│ │ │ └───── Month (1-12)
│ │ └─────── Day of month (1-31)
│ └───────── Hour (0-23)
└─────────── Minute (0-59)
```

**Examples:**

```bash
"0 0 * * *"      # Daily at midnight
"*/15 * * * *"   # Every 15 minutes
"0 9 * * 1-5"    # Weekdays at 9 AM
"0 0 1 * *"      # First day of month at midnight
"0 */6 * * *"    # Every 6 hours
```

**Tools:**
- [Crontab Guru](https://crontab.guru/) - Cron expression editor
- [Cron Expression Generator](https://www.freeformatter.com/cron-expression-generator-quartz.html)

## Webhook Execution

When a job executes, if `webhook_url` is configured, the system will make an HTTP POST request:

**Request:**

```http
POST {webhook_url}
Content-Type: application/json

{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "unico",
  "executed_at": 1704067200,
  "payload": {
    "custom": "data"
  }
}
```

**Expected Response:**

Any 2xx status code indicates success. Non-2xx responses are logged as failures.

## Request/Response Flow

### Write Operations (Create/Delete)

```mermaid
sequenceDiagram
    participant Client
    participant Follower
    participant Leader
    
    Client->>Follower: POST /jobs
    Follower->>Follower: Check IsLeader()
    Follower->>Leader: Proxy Request
    Leader->>Leader: Validate
    Leader->>Leader: Apply to Raft
    Leader->>Follower: Replicate
    Leader->>Client: 200 OK
```

### Read Operations (Get)

```mermaid
sequenceDiagram
    participant Client
    participant Node
    
    Client->>Node: GET /jobs/{id}
    Node->>Node: Read from FSM
    Node->>Client: 200 OK
```

## Rate Limiting

Currently, no rate limiting is implemented. For production:

- Implement rate limiting middleware
- Use API gateway (Kong, Traefik)
- Configure Kubernetes Ingress rate limits

## Error Codes

| Code | Meaning | Common Causes |
|------|---------|---------------|
| 200 | OK | Request successful |
| 400 | Bad Request | Invalid input, validation error |
| 403 | Forbidden | Not leader, cannot perform operation |
| 404 | Not Found | Job ID doesn't exist |
| 500 | Internal Server Error | Raft error, database error |
| 503 | Service Unavailable | No leader elected |
| 507 | Insufficient Storage | Queue memory or job count limit reached |

## Best Practices

### Job IDs

- Let the system generate UUIDs for you
- Only provide custom IDs if you need idempotency
- Use meaningful IDs for debugging: `daily-report-2024-12-25`

### Timestamps

- Always use RFC3339 format for clarity
- Include timezone information
- Ensure timestamps are in the future for unico jobs

### Cron Expressions

- Test expressions before deploying
- Use `last_date` to prevent infinite execution
- Consider timezone implications

### Webhooks

- Implement idempotency in webhook handlers
- Return 2xx quickly, process asynchronously
- Handle retries gracefully
- Log webhook responses for debugging

### Error Handling

- Always check response status codes
- Implement retry logic with exponential backoff
- Log errors for debugging
- Handle 503 (no leader) by retrying
- Handle 507 (capacity exceeded) by:
  - Deleting old completed jobs
  - Increasing memory/job limits
  - Scaling horizontally
  - Implementing job prioritization

## Examples

### Complete Job Lifecycle

```bash
# 1. Create job
JOB_ID=$(curl -s -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "unico",
    "timestamp": "'$(date -u -v+1H +%Y-%m-%dT%H:%M:%SZ)'",
    "webhook_url": "https://webhook.site/unique-id"
  }' | jq -r '.id')

echo "Created job: $JOB_ID"

# 2. Get job details
curl http://localhost:8080/jobs/$JOB_ID | jq

# 3. Check initial status
curl http://localhost:8080/jobs/$JOB_ID/status | jq
# Expected: {"status": "pending", ...}

# 4. Wait for execution...
# (Job will execute at specified timestamp)

# 5. Check status after execution
curl http://localhost:8080/jobs/$JOB_ID/status | jq
# Expected: {"status": "completed", ...}

# 6. View execution history
curl http://localhost:8080/jobs/$JOB_ID/executions | jq

# 7. Delete job (if needed)
curl -X DELETE http://localhost:8080/jobs/$JOB_ID
```

### Job Status Monitoring

```bash
# Monitor job status in real-time
JOB_ID="your-job-id"

# Watch status changes
watch -n 1 "curl -s http://localhost:8080/jobs/$JOB_ID/status | jq"

# Check if job completed successfully
STATUS=$(curl -s http://localhost:8080/jobs/$JOB_ID/status | jq -r '.status')
if [ "$STATUS" = "completed" ]; then
  echo "Job completed successfully"
elif [ "$STATUS" = "failed" ]; then
  echo "Job failed"
  curl -s http://localhost:8080/jobs/$JOB_ID/executions | jq '.attempts[-1]'
fi
```

### Handling Failed Jobs

```bash
# List all failed jobs
curl http://localhost:8080/jobs?status=failed | jq

# Get details of a failed job
JOB_ID="failed-job-id"
curl http://localhost:8080/jobs/$JOB_ID/executions | jq

# Check error details
curl http://localhost:8080/jobs/$JOB_ID/executions | jq '.attempts[] | {
  attempt: .attempt_num,
  error_type: .error_type,
  error_message: .error_message,
  response_time: .response_time_ms
}'
```

### Cancelling Jobs

```bash
# Cancel a pending job
curl -X POST http://localhost:8080/jobs/pending-job-id/cancel \
  -H "Content-Type: application/json" \
  -d '{"reason": "Requirements changed"}'

# Cancel an in-progress job (will attempt to stop)
curl -X POST http://localhost:8080/jobs/running-job-id/cancel \
  -H "Content-Type: application/json" \
  -d '{"reason": "Taking too long"}'

# List all cancelled jobs
curl http://localhost:8080/jobs?status=cancelled | jq
```

### Batch Job Creation

```bash
# Create multiple jobs
for i in {1..10}; do
  TIMESTAMP=$(date -u -v+${i}M +%Y-%m-%dT%H:%M:%SZ)
  curl -X POST http://localhost:8080/jobs \
    -H "Content-Type: application/json" \
    -d "{
      \"type\": \"unico\",
      \"timestamp\": \"$TIMESTAMP\",
      \"payload\": {\"batch\": $i}
    }"
  echo ""
done
```

### Monitoring Job Execution

```bash
# Check cluster health
watch -n 1 'curl -s http://localhost:8080/health | jq'

# Monitor job count
watch -n 1 'curl -s http://localhost:8080/debug/cluster | jq .job_count'
```

## Client Libraries

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "net/http"
    "time"
)

type Job struct {
    Type       string                 `json:"type"`
    Timestamp  string                 `json:"timestamp,omitempty"`
    CronExpr   string                 `json:"cron_expression,omitempty"`
    WebhookURL string                 `json:"webhook_url,omitempty"`
    Payload    map[string]interface{} `json:"payload,omitempty"`
}

func CreateJob(baseURL string, job *Job) error {
    data, _ := json.Marshal(job)
    resp, err := http.Post(
        baseURL+"/jobs",
        "application/json",
        bytes.NewBuffer(data),
    )
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    return nil
}

func main() {
    job := &Job{
        Type:      "unico",
        Timestamp: time.Now().Add(1 * time.Hour).Format(time.RFC3339),
    }
    CreateJob("http://localhost:8080", job)
}
```

### Python

```python
import requests
from datetime import datetime, timedelta

def create_job(base_url, job_data):
    response = requests.post(
        f"{base_url}/jobs",
        json=job_data
    )
    return response.json()

# One-time job
job = {
    "type": "unico",
    "timestamp": (datetime.now() + timedelta(hours=1)).isoformat() + "Z",
    "webhook_url": "https://example.com/webhook"
}

result = create_job("http://localhost:8080", job)
print(f"Created job: {result['id']}")
```

### JavaScript/Node.js

```javascript
const axios = require('axios');

async function createJob(baseURL, jobData) {
  const response = await axios.post(`${baseURL}/jobs`, jobData);
  return response.data;
}

// Recurring job
const job = {
  type: 'recurrente',
  cron_expression: '0 0 * * *',
  webhook_url: 'https://example.com/webhook'
};

createJob('http://localhost:8080', job)
  .then(result => console.log('Created job:', result.id))
  .catch(error => console.error('Error:', error));
```

## Capacity Management

### Overview

Scheduled-DB enforces memory-based and count-based limits to prevent system overload. When limits are reached, job creation requests are rejected with HTTP 507 status code.

### Configuration

Configure limits via environment variables:

```bash
# Explicit memory limit (takes precedence)
export QUEUE_MEMORY_LIMIT=2GB

# Memory percentage (used if QUEUE_MEMORY_LIMIT not set)
export QUEUE_MEMORY_PERCENT=50

# Job count limit
export QUEUE_JOB_LIMIT=100000
```

**Memory Limit Formats:**
- `2GB` - 2 gigabytes
- `500MB` - 500 megabytes
- `1073741824` - Bytes (no suffix)

**Auto-Detection:**
If `QUEUE_MEMORY_LIMIT` is not set, the system detects available memory and uses `QUEUE_MEMORY_PERCENT` (default 50%).

### Capacity Errors

When capacity is exceeded, the API returns 507 status:

**Memory Limit Exceeded:**

```json
{
  "error": "insufficient memory: current=950000000 bytes, limit=1000000000 bytes, requested=100000000 bytes",
  "type": "memory",
  "current": 950000000,
  "limit": 1000000000,
  "requested": 100000000
}
```

**Job Count Limit Exceeded:**

```json
{
  "error": "maximum jobs reached: current=100000, limit=100000",
  "type": "job_count",
  "current": 100000,
  "limit": 100000
}
```

### Monitoring Capacity

**Health Endpoint:**

```bash
curl http://localhost:8080/health | jq '.memory'
curl http://localhost:8080/health | jq '.jobs'
```

**Prometheus Metrics:**

```bash
# Memory metrics
curl http://localhost:9090/metrics | grep scheduled_db_queue_memory_usage_bytes
curl http://localhost:9090/metrics | grep scheduled_db_queue_memory_limit_bytes
curl http://localhost:9090/metrics | grep scheduled_db_queue_memory_utilization_percent

# Job count metrics
curl http://localhost:9090/metrics | grep scheduled_db_queue_job_count
curl http://localhost:9090/metrics | grep scheduled_db_queue_job_limit

# Rejection metrics
curl http://localhost:9090/metrics | grep scheduled_db_job_rejections_total
```

### Handling Capacity Issues

**When receiving 507 errors:**

1. **Check current capacity:**
   ```bash
   curl http://localhost:8080/health | jq
   ```

2. **Delete old completed jobs:**
   ```bash
   # List completed jobs
   curl http://localhost:8080/jobs?status=completed | jq
   
   # Delete specific job
   curl -X DELETE http://localhost:8080/jobs/{job-id}
   ```

3. **Increase limits:**
   ```bash
   # Increase memory limit
   export QUEUE_MEMORY_LIMIT=4GB
   
   # Increase job count limit
   export QUEUE_JOB_LIMIT=200000
   
   # Restart service
   ```

4. **Scale horizontally:**
   - Add more nodes to the cluster
   - Distribute load across multiple clusters

### Job Size Calculation

Job memory size includes:
- Job ID string
- Webhook URL string
- Cron expression string
- Serialized payload (JSON)
- Fixed metadata overhead (~64 bytes)

**Example sizes:**
- Minimal job (no payload): ~200 bytes
- Job with 1KB payload: ~1.2KB
- Job with 10KB payload: ~10.2KB

### Best Practices

1. **Set appropriate limits** based on available system memory
2. **Monitor utilization** using Prometheus metrics
3. **Set up alerts** when utilization exceeds 80%
4. **Clean up completed jobs** regularly
5. **Use job count limits** to prevent unbounded growth
6. **Test capacity** under expected load before production

## Metrics Endpoint

Prometheus metrics are exposed on port 9090:

```bash
curl http://localhost:9090/metrics
```

**Capacity Metrics:**

```
# Memory usage in bytes
scheduled_db_queue_memory_usage_bytes

# Configured memory limit in bytes
scheduled_db_queue_memory_limit_bytes

# Memory utilization percentage (0-100)
scheduled_db_queue_memory_utilization_percent

# Current job count
scheduled_db_queue_job_count

# Configured job count limit
scheduled_db_queue_job_limit

# Job rejections counter with reason label
scheduled_db_job_rejections_total{reason="memory"}
scheduled_db_job_rejections_total{reason="job_count"}
```

See [Monitoring Guide](./monitoring-guide.md) for details.
