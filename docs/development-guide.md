# Development Guide

## Getting Started

### Prerequisites

- Go 1.23+ (toolchain 1.24.2)
- Docker and Docker Compose
- kubectl (for Kubernetes deployment)
- make

### Project Structure

```
scheduled-db/
├── cmd/
│   └── scheduled-db/          # Main application entry point
│       └── main.go
├── internal/                  # Private application code
│   ├── api/                   # HTTP API handlers and routing
│   ├── discovery/             # Service discovery strategies
│   ├── logger/                # Structured logging
│   ├── metrics/               # Observability and metrics
│   ├── slots/                 # Time-slotted job queue
│   └── store/                 # Raft state management
├── docker/                    # Docker configuration
├── k8s/                       # Kubernetes manifests
├── scripts/                   # Operational scripts
├── docs/                      # Documentation
└── .kiro/                     # Kiro IDE configuration
    ├── specs/                 # Feature specifications
    └── steering/              # Development guidelines
```

## Building and Running

### Local Development

```bash
# Build the binary
make build

# Run single node
./scheduled-db --node-id=node-1

# Run with custom configuration
./scheduled-db \
  --node-id=node-1 \
  --http-port=8080 \
  --raft-port=7000 \
  --data-dir=./data \
  --slot-gap=10s
```

### Docker Compose Cluster

```bash
# Start 3-node cluster with nginx load balancer
make dev-up

# View logs
make dev-logs

# Stop cluster
make dev-down

# Access points:
# - API: http://localhost:80
# - Node 1: http://localhost:8080
# - Node 2: http://localhost:8081
# - Node 3: http://localhost:8082
# - Prometheus: http://localhost:9090
# - Grafana: http://localhost:3000 (admin/admin)
```

### Kubernetes Deployment

```bash
# Build and push image
make docker-build docker-push

# Deploy to Kubernetes
make k8s-deploy

# Check status
make k8s-status

# View logs
make k8s-logs

# Port forward for local access
make k8s-port-forward
```

## Testing

### Running Tests

```bash
# Run all tests
make test

# Run short tests
make test-short

# Run specific package tests
go test -v ./internal/store

# Run with race detector
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Test Structure

Tests are co-located with source files:

```
internal/store/
├── store.go
├── store_test.go      # Store tests (excluded: requires Raft)
├── fsm.go
├── fsm_test.go        # FSM unit tests ✓
├── types.go
└── types_test.go      # Types unit tests ✓
```

### Writing Tests

Follow these patterns:

```go
// Table-driven tests
func TestJobValidation(t *testing.T) {
    tests := []struct {
        name      string
        job       *Job
        wantError bool
    }{
        {
            name: "valid job",
            job: &Job{
                ID: "test",
                Type: JobUnico,
                Timestamp: &futureTime,
            },
            wantError: false,
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.job.Validate()
            if (err != nil) != tt.wantError {
                t.Errorf("Validate() error = %v, wantError %v", err, tt.wantError)
            }
        })
    }
}
```

## API Usage

### Creating Jobs

#### One-Time Job (Unico)

```bash
# Using RFC3339 timestamp
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "unico",
    "timestamp": "2024-12-25T10:00:00Z",
    "webhook_url": "https://example.com/webhook",
    "payload": {
      "message": "Merry Christmas!"
    }
  }'

# Using epoch timestamp
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "unico",
    "timestamp": "1735120800"
  }'
```

#### Recurring Job (Recurrente)

```bash
# Daily at midnight
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "recurrente",
    "cron_expression": "0 0 * * *",
    "webhook_url": "https://example.com/daily-report"
  }'

# Every 15 minutes
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "recurrente",
    "cron_expression": "*/15 * * * *"
  }'

# With end date
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "recurrente",
    "cron_expression": "0 9 * * 1-5",
    "last_date": "2024-12-31T23:59:59Z"
  }'
```

### Retrieving Jobs

```bash
# Get specific job
curl http://localhost:8080/jobs/{job-id}

# Health check
curl http://localhost:8080/health

# Cluster debug info
curl http://localhost:8080/debug/cluster
```

### Deleting Jobs

```bash
curl -X DELETE http://localhost:8080/jobs/{job-id}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NODE_ID` | `node-1` | Unique node identifier |
| `HTTP_PORT` | `8080` | HTTP API port |
| `RAFT_PORT` | `7000` | Raft communication port |
| `DATA_DIR` | `./data` | Data directory for persistence |
| `SLOT_GAP` | `10s` | Time gap for slot intervals |
| `LOG_LEVEL` | `INFO` | Logging level (DEBUG, INFO, WARN, ERROR) |
| `DISCOVERY_STRATEGY` | `` | Discovery strategy (static, kubernetes, dns, gossip) |
| `PEERS` | `` | Comma-separated peer addresses |

### Logging Configuration

```bash
# Enable debug logging
LOG_LEVEL=DEBUG ./scheduled-db

# Only errors
LOG_LEVEL=ERROR ./scheduled-db
```

### Discovery Strategies

#### Static Peers

```bash
./scheduled-db \
  --node-id=node-1 \
  --peers=node-2:7000,node-3:7000
```

#### Kubernetes

```bash
DISCOVERY_STRATEGY=kubernetes \
KUBERNETES_IN_CLUSTER=true \
POD_NAMESPACE=default \
./scheduled-db
```

#### DNS

```bash
DISCOVERY_STRATEGY=dns \
DNS_DOMAIN=cluster.local \
./scheduled-db
```

## Code Style and Conventions

### Logging Standards

Always use the structured logger:

```go
import "scheduled-db/internal/logger"

// ✓ Correct
logger.Info("job created: %s", jobID)
logger.Error("failed to execute job: %v", err)
logger.Debug("processing slot %d", slotKey)

// ✗ Incorrect
log.Printf("job created: %s", jobID)
fmt.Printf("DEBUG: %v", data)
```

See [logging.md](../.kiro/steering/logging.md) for complete guidelines.

### Error Handling

```go
// Return errors, don't panic
func ProcessJob(job *Job) error {
    if err := job.Validate(); err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    // ...
    return nil
}

// Log and exit for fatal errors
if err := app.Start(); err != nil {
    logger.Error("failed to start application: %v", err)
    os.Exit(1)
}
```

### Naming Conventions

- **Packages**: lowercase, single word (`slots`, `store`, `api`)
- **Files**: lowercase with underscores (`persistent_slots.go`)
- **Types**: PascalCase (`SlotQueue`, `Store`)
- **Functions**: PascalCase for exported, camelCase for private
- **Constants**: PascalCase or SCREAMING_SNAKE_CASE

### Import Organization

```go
import (
    // Standard library
    "context"
    "fmt"
    "time"
    
    // External packages
    "github.com/hashicorp/raft"
    "github.com/robfig/cron/v3"
    
    // Internal packages
    "scheduled-db/internal/logger"
    "scheduled-db/internal/store"
)
```

## Debugging

### Local Debugging

```bash
# Run with debug logging
LOG_LEVEL=DEBUG ./scheduled-db

# Check cluster status
curl http://localhost:8080/debug/cluster | jq

# View Raft state
curl http://localhost:8080/health | jq
```

### Docker Debugging

```bash
# View logs for specific node
docker logs scheduled-db-node1 -f

# Execute commands in container
docker exec -it scheduled-db-node1 sh

# Check Raft data
docker exec scheduled-db-node1 ls -la /data
```

### Kubernetes Debugging

```bash
# Get pod logs
kubectl logs scheduled-db-0 -f

# Execute shell in pod
kubectl exec -it scheduled-db-0 -- sh

# Check persistent volume
kubectl describe pvc data-scheduled-db-0

# Port forward for debugging
kubectl port-forward scheduled-db-0 8080:8080
```

### Common Issues

#### Split Brain Prevention

If you see "split-brain detected" errors:

```bash
# Check cluster configuration
curl http://localhost:8080/debug/cluster

# Verify all nodes can communicate
# Check network connectivity between nodes
```

#### Leader Election Issues

```bash
# Check Raft state on all nodes
for port in 8080 8081 8082; do
  echo "Node on port $port:"
  curl -s http://localhost:$port/health | jq '.role, .leader'
done

# Trigger manual election (if needed)
# Restart the current leader node
```

#### Job Not Executing

```bash
# Check if node is leader
curl http://localhost:8080/health | jq '.role'

# Check slot queue
# Enable DEBUG logging to see slot processing

# Verify job timestamp is in the future
curl http://localhost:8080/jobs/{job-id} | jq '.timestamp'
```

## Performance Tuning

### Raft Configuration

Adjust timeouts in `internal/store/store.go`:

```go
config.HeartbeatTimeout = 5000 * time.Millisecond
config.ElectionTimeout = 5000 * time.Millisecond
config.CommitTimeout = 200 * time.Millisecond
```

### Slot Gap Optimization

```bash
# Smaller gap = more precise scheduling, more memory
--slot-gap=1s

# Larger gap = less memory, less precise
--slot-gap=60s
```

### Worker Polling

Adjust in `internal/slots/worker.go`:

```go
ticker := time.NewTicker(1 * time.Second)  // Polling interval
```

## Contributing

### Before Submitting

1. Run tests: `make test`
2. Run linter: `make lint`
3. Format code: `make fmt`
4. Update documentation if needed
5. Add tests for new features

### Creating Specs

For new features, create a spec in `.kiro/specs/`:

```bash
.kiro/specs/my-feature/
├── requirements.md  # User stories and acceptance criteria
├── design.md        # Technical design
└── tasks.md         # Implementation tasks
```

### Pull Request Guidelines

- Clear description of changes
- Reference related issues
- Include tests
- Update documentation
- Follow code style guidelines

## Useful Commands

```bash
# Development
make build              # Build binary
make test               # Run tests
make lint               # Run linter
make fmt                # Format code

# Docker
make docker-build       # Build image
make dev-up             # Start cluster
make dev-down           # Stop cluster

# Kubernetes
make k8s-deploy         # Deploy
make k8s-delete         # Remove
make k8s-status         # Check status

# Testing
make create-jobs        # Create test jobs
make test-proxy         # Test proxy
make test-failover      # Test failover
make cluster-info       # Show cluster info

# Utilities
make clean              # Clean artifacts
make version            # Show version
make help               # Show all commands
```

## Resources

- [Architecture Documentation](./architecture.md)
- [API Reference](./api-reference.md)
- [Deployment Guide](./deployment-guide.md)
- [Raft Consensus](https://raft.github.io/)
- [Cron Expression Format](https://crontab.guru/)
