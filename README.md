# Scheduled-DB

A distributed, fault-tolerant job scheduler built with Go and Raft consensus algorithm. Supports both unique and recurring job scheduling with automatic failover and load balancing.

## 🏗️ Project Structure

```
scheduled-db/
├── cmd/                    # Application entry points
│   └── scheduled-db/
│       └── main.go        # Main application
├── internal/              # Internal Go packages
│   ├── api/              # HTTP API handlers and routing
│   ├── discovery/        # Cluster discovery strategies
│   ├── slots/           # Job scheduling and execution
│   └── store/           # Raft store and data persistence
├── docker/              # Docker-related files
│   ├── docker-compose.yml  # Multi-node cluster setup
│   ├── Dockerfile          # Container image definition
│   ├── nginx.conf         # Load balancer configuration
│   └── prometheus.yml     # Monitoring configuration
├── k8s/                 # Kubernetes manifests
│   ├── statefulset.yaml   # Pod deployment
│   ├── service.yaml       # Service definitions
│   ├── configmap.yaml     # Configuration
│   └── kustomization.yaml # Kustomize configuration
├── scripts/             # Utility scripts
│   ├── create-test-jobs.sh           # Job creation utility
│   ├── start-traditional-cluster-fixed.sh  # Local cluster startup
│   ├── stop-traditional-cluster.sh  # Local cluster shutdown
│   └── test-*.sh                     # Testing scripts
└── Makefile            # Build and deployment automation
```

## 🚀 Quick Start

### Option 1: Docker Cluster (Recommended)

Start a 3-node cluster with nginx load balancer:

```bash
make dev-up        # Start Docker cluster
make create-jobs   # Create test jobs
make dev-down      # Stop cluster
```

### Option 2: Local Development Cluster

Start a traditional cluster on localhost:

```bash
make cluster-start  # Start local cluster
make create-jobs    # Create test jobs
make cluster-stop   # Stop cluster
```

### Option 3: Kubernetes Deployment

Deploy to Kubernetes cluster:

```bash
make k8s-deploy     # Deploy to Kubernetes
make create-jobs    # Create test jobs (auto-detects K8s)
make k8s-delete     # Remove from Kubernetes
```

## 📋 Available Commands

### Build & Test
- `make build` - Build the binary
- `make test` - Run tests with coverage
- `make lint` - Run golangci-lint
- `make clean` - Clean build artifacts

### Docker Development
- `make dev-up` - Start Docker cluster
- `make dev-down` - Stop Docker cluster
- `make dev-logs` - Show cluster logs

### Local Cluster
- `make cluster-start` - Start local cluster
- `make cluster-stop` - Stop local cluster
- `make cluster-test` - Test cluster functionality

### Kubernetes
- `make k8s-deploy` - Deploy to Kubernetes
- `make k8s-delete` - Delete from Kubernetes
- `make k8s-status` - Check deployment status
- `make k8s-logs` - Show pod logs

### Job Management
- `make create-jobs` - Create test jobs (auto-detects environment)
- `make create-jobs-local` - Create jobs on local cluster
- `make create-jobs-k8s` - Create jobs on Kubernetes

## 🔧 Configuration

### Environment Variables

```bash
# Raft Configuration
RAFT_HOST=0.0.0.0              # Bind address for Raft
RAFT_ADVERTISE_HOST=node-1      # Advertise address for Raft
RAFT_PORT=7000                  # Raft communication port
HTTP_HOST=0.0.0.0              # HTTP API bind address
HTTP_PORT=8080                  # HTTP API port

# Node Configuration
NODE_ID=node-1                  # Unique node identifier
DATA_DIR=/data                  # Data storage directory
SLOT_GAP=10s                   # Time slot interval

# Discovery Configuration
DISCOVERY_STRATEGY=static       # Discovery method (static/kubernetes/dns/gossip)
STATIC_PEERS=node1:7000,node2:7000,node3:7000  # Static peer list
```

### Docker Compose Configuration

The Docker setup includes:
- **3 scheduled-db nodes** with automatic discovery
- **Nginx load balancer** for high availability
- **Prometheus** for monitoring
- **Grafana** for visualization
- **Persistent volumes** for data storage

## 📡 API Endpoints

### Job Management
- `POST /jobs` - Create a new job
- `GET /jobs/{id}` - Get job details
- `DELETE /jobs/{id}` - Delete a job

### Cluster Management
- `GET /health` - Node health status
- `GET /debug/cluster` - Cluster information
- `POST /join` - Join cluster (leader only)

### Example: Create a Job

```bash
# Unique job (executes once)
curl -X POST http://localhost:80/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "unico",
    "timestamp": "2025-12-25T10:00:00Z"
  }'

# Recurring job (cron-based)
curl -X POST http://localhost:80/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "type": "recurrente",
    "timestamp": "2025-12-25T10:00:00Z",
    "cron_expression": "0 */6 * * *"
  }'
```

## 🏃‍♂️ Development

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- kubectl (for Kubernetes)
- make

### Building from Source

```bash
git clone <repository>
cd scheduled-db
go mod download
make build
```

### Running Tests

```bash
make test              # Run all tests
make test-short        # Run short tests only
make lint              # Run linter
```

### Local Development

```bash
# Start a development cluster
make cluster-start

# In another terminal, create some test jobs
make create-jobs

# Monitor logs
tail -f logs_node-*.log

# Test failover
scripts/test-failover.sh
```

## 🔍 Monitoring

### Access Points (Docker)
- **API**: http://localhost:80 (nginx load balancer)
- **Node 1**: http://localhost:8080
- **Node 2**: http://localhost:8081  
- **Node 3**: http://localhost:8082
- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3000 (admin/admin)

### Health Checks

```bash
# Check cluster health
curl http://localhost:80/health

# Get detailed cluster info
curl http://localhost:80/debug/cluster | jq

# Monitor job execution
docker logs scheduled-db-node-1 -f | grep "Executing job"
```

## 🚨 Troubleshooting

### Common Issues

**Port conflicts**: If you get "port already in use" errors:
```bash
# Find and kill processes using ports
lsof -i :8080
kill <PID>
```

**Docker network issues**: Clean up Docker resources:
```bash
docker network prune -f
docker volume prune -f
```

**Cluster not forming**: Check node connectivity:
```bash
# Verify all nodes are healthy
make cluster-info

# Check logs for errors
make dev-logs
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Run tests: `make test lint`
4. Submit a pull request

## 📄 License

[Add your license here]

## 🔗 Related Documentation

- [Kubernetes Deployment Guide](k8s/README.md)
- [API Documentation](docs/api.md)
- [Architecture Overview](docs/architecture.md)