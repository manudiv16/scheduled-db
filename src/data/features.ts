export const features = [
  {
    icon: 'consensus',
    title: 'Distributed Consensus',
    description: 'Built on HashiCorp Raft for strong consistency. Automatic leader election and log replication across all nodes.',
    color: '#22d3ee',
  },
  {
    icon: 'jobs',
    title: 'Two Job Types',
    description: 'Unico: one-time execution at a specific timestamp. Recurrente: recurring execution with cron expressions.',
    color: '#a78bfa',
  },
  {
    icon: 'slots',
    title: 'Time-Slotted Scheduling',
    description: 'Efficient job organization with configurable slot intervals. Optimized for high-throughput scheduling.',
    color: '#3b82f6',
  },
  {
    icon: 'capacity',
    title: 'Queue Size Limits',
    description: 'Configurable memory and job count limits to prevent OOM. Smart rejection with 507 Insufficient Storage.',
    color: '#fbbf24',
  },
  {
    icon: 'ha',
    title: 'High Availability',
    description: 'Automatic failover and graceful leader resignation. Zero-downtime cluster reconfiguration.',
    color: '#34d399',
  },
  {
    icon: 'discovery',
    title: 'Service Discovery',
    description: 'Multiple strategies: Kubernetes, DNS, Gossip, Static. Automatic cluster formation and node joining.',
    color: '#f472b6',
  },
];

export const apiEndpoints = [
  { method: 'POST', path: '/jobs', description: 'Create job' },
  { method: 'GET', path: '/jobs/{id}', description: 'Get job details' },
  { method: 'DELETE', path: '/jobs/{id}', description: 'Delete job' },
  { method: 'GET', path: '/jobs/{id}/status', description: 'Get job execution status' },
  { method: 'GET', path: '/jobs/{id}/executions', description: 'Get execution history' },
  { method: 'GET', path: '/jobs?status={status}', description: 'List jobs by status' },
  { method: 'POST', path: '/jobs/{id}/cancel', description: 'Cancel job' },
  { method: 'GET', path: '/health', description: 'Health check' },
  { method: 'GET', path: '/debug/cluster', description: 'Cluster info' },
  { method: 'POST', path: '/join', description: 'Join cluster' },
];

export const codeExamples = {
  docker: `# Start 3-node cluster with load balancer
make dev-up

# Create a test job
curl -X POST http://localhost:80/jobs \\
  -H "Content-Type: application/json" \\
  -d '{
    "type": "unico",
    "timestamp": "2024-12-25T10:00:00Z",
    "webhook_url": "https://webhook.site/your-id"
  }'

# Check cluster status
curl http://localhost:80/health | jq

# View logs
make dev-logs

# Stop cluster
make dev-down`,

  k8s: `# Deploy to Kubernetes
make k8s-deploy

# Check status
kubectl get pods -l app=scheduled-db

# Port forward for local access
kubectl port-forward svc/scheduled-db-api 8080:8080

# Create a recurring job
curl -X POST http://localhost:8080/jobs \\
  -H "Content-Type: application/json" \\
  -d '{
    "type": "recurrente",
    "cron_expression": "0 0 * * *",
    "webhook_url": "https://example.com/daily-report"
  }'`,

  binary: `# Build
make build

# Run single node
./scheduled-db --node-id=node-1

# Run with custom config
./scheduled-db \\
  --node-id=node-1 \\
  --http-port=8080 \\
  --raft-port=7000 \\
  --data-dir=./data \\
  --slot-gap=10s`,
};
