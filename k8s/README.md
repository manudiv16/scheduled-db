# Scheduled-DB Kubernetes Deployment

This directory contains Kubernetes manifests and configuration files for deploying the scheduled-db cluster in a Kubernetes environment.

## 📁 Directory Structure

```
k8s/
├── README.md                 # This file
├── configmap.yaml           # Configuration for the cluster
├── rbac.yaml               # ServiceAccount and RBAC permissions
├── service.yaml            # Kubernetes services
├── statefulset.yaml        # StatefulSet for the cluster nodes
├── pdb.yaml               # PodDisruptionBudget for HA
├── hpa.yaml               # HorizontalPodAutoscaler
└── kustomization.yaml     # Kustomize configuration
```

## 🚀 Quick Start

### Prerequisites

- Kubernetes cluster (v1.19+)
- `kubectl` configured to access your cluster
- Container registry access (or local Docker images)

### Deploy the Cluster

```bash
# Build and push the Docker image
make docker-build docker-push

# Deploy to Kubernetes
make k8s-deploy

# Check deployment status
make k8s-status
```

### Alternative: Using kubectl directly

```bash
# Apply all manifests
kubectl apply -k k8s/

# Check pods
kubectl get pods -l app=scheduled-db

# Check services
kubectl get svc -l app=scheduled-db
```

## 📋 Components

### ConfigMap (`configmap.yaml`)
Contains environment variables and configuration for the cluster:
- Discovery strategy (Kubernetes)
- Raft and HTTP ports
- Slot configuration
- Service discovery settings

### RBAC (`rbac.yaml`)
Defines permissions for the scheduled-db pods to:
- Read pods, services, and endpoints
- Perform Kubernetes service discovery
- Access ConfigMaps and Secrets

### Services (`service.yaml`)
- **scheduled-db** (Headless): For internal Raft communication
- **scheduled-db-api**: Load-balanced API access

### StatefulSet (`statefulset.yaml`)
Manages the cluster nodes with:
- 3 replicas by default
- Persistent storage (1Gi per node)
- Health checks and probes
- Environment variable injection

### PodDisruptionBudget (`pdb.yaml`)
Ensures at least 2 pods remain available during voluntary disruptions.

### HorizontalPodAutoscaler (`hpa.yaml`)
Automatically scales the cluster based on CPU and memory usage:
- Min replicas: 3
- Max replicas: 7
- Target CPU: 70%
- Target Memory: 80%

## ⚙️ Configuration

### Environment Variables

Key environment variables configurable through the ConfigMap:

| Variable | Default | Description |
|----------|---------|-------------|
| `DISCOVERY_STRATEGY` | `kubernetes` | Service discovery method |
| `RAFT_PORT` | `7000` | Port for Raft communication |
| `HTTP_PORT` | `8080` | Port for HTTP API |
| `DATA_DIR` | `/data` | Directory for persistent data |
| `SLOT_GAP` | `10s` | Time gap for slot intervals |

### Scaling

#### Manual Scaling
```bash
# Scale to 5 replicas
kubectl scale statefulset scheduled-db --replicas=5

# Or edit the StatefulSet
kubectl edit statefulset scheduled-db
```

#### Automatic Scaling
The HPA will automatically scale based on resource usage. To modify:
```bash
kubectl edit hpa scheduled-db-hpa
```

### Storage

Each node gets a 1Gi persistent volume. To change:
1. Edit `statefulset.yaml`
2. Modify the `volumeClaimTemplates` section
3. Reapply the manifest

**Note**: Existing PVCs won't be resized automatically.

## 🔍 Monitoring and Debugging

### Check Cluster Status
```bash
# View all resources
kubectl get all -l app=scheduled-db

# Check pod logs
kubectl logs -f statefulset/scheduled-db

# Check specific pod
kubectl logs scheduled-db-0 -f

# Get cluster info via API
kubectl port-forward svc/scheduled-db-api 8080:8080
curl http://localhost:8080/debug/cluster
```

### Health Checks
Each pod has three types of probes:
- **Startup Probe**: Ensures the application starts properly
- **Liveness Probe**: Restarts pod if unhealthy
- **Readiness Probe**: Controls traffic routing

### Troubleshooting

#### Pods Not Starting
```bash
# Check pod events
kubectl describe pod scheduled-db-0

# Check resource constraints
kubectl top pods -l app=scheduled-db

# Verify RBAC permissions
kubectl auth can-i get pods --as=system:serviceaccount:default:scheduled-db
```

#### Network Issues
```bash
# Test internal DNS resolution
kubectl exec scheduled-db-0 -- nslookup scheduled-db

# Check service endpoints
kubectl get endpoints scheduled-db

# Test connectivity between pods
kubectl exec scheduled-db-0 -- wget -qO- http://scheduled-db-1:8080/health
```

#### Storage Issues
```bash
# Check PVC status
kubectl get pvc -l app=scheduled-db

# Check storage class
kubectl get storageclass

# View volume details
kubectl describe pv
```

## 🛠️ Development

### Local Testing with Kind

```bash
# Create a Kind cluster
kind create cluster --name scheduled-db

# Load Docker image
kind load docker-image scheduled-db:latest --name scheduled-db

# Deploy
kubectl apply -k k8s/

# Port forward for testing
kubectl port-forward svc/scheduled-db-api 8080:8080
```

### Using Minikube

```bash
# Start Minikube
minikube start

# Use Minikube's Docker daemon
eval $(minikube docker-env)

# Build image in Minikube
docker build -t scheduled-db:latest .

# Deploy
kubectl apply -k k8s/

# Access via Minikube service
minikube service scheduled-db-api
```

## 🔧 Customization

### Using Kustomize

The `kustomization.yaml` file allows easy customization:

```bash
# Custom namespace
cd k8s/
kustomize edit set namespace production

# Custom image
kustomize edit set image scheduled-db=myregistry/scheduled-db:v1.2.3

# Apply with custom settings
kubectl apply -k .
```

### Environment-Specific Overlays

Create environment-specific configurations:

```bash
mkdir -p k8s/overlays/production
cd k8s/overlays/production

# Create kustomization.yaml
cat > kustomization.yaml << EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- ../../base

patchesStrategicMerge:
- replica-count.yaml
- resource-limits.yaml
EOF

# Create patches for production
cat > replica-count.yaml << EOF
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: scheduled-db
spec:
  replicas: 5
EOF
```

## 🔒 Security

### Security Context
Pods run with:
- Non-root user (65534)
- Read-only root filesystem
- Dropped capabilities
- No privilege escalation

### Network Policies

To restrict network access:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: scheduled-db-netpol
spec:
  podSelector:
    matchLabels:
      app: scheduled-db
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - podSelector:
        matchLabels:
          app: scheduled-db
    ports:
    - protocol: TCP
      port: 7000
    - protocol: TCP
      port: 8080
```

### Secrets Management

For sensitive configuration:

```bash
# Create secret
kubectl create secret generic scheduled-db-secrets \
  --from-literal=api-key=your-secret-key

# Reference in StatefulSet
# envFrom:
# - secretRef:
#     name: scheduled-db-secrets
```

## 📊 Production Considerations

### Resource Requirements
- **CPU**: 100m request, 500m limit per pod
- **Memory**: 128Mi request, 512Mi limit per pod
- **Storage**: 1Gi per pod (adjustable)

### High Availability
- Minimum 3 replicas for Raft consensus
- PodDisruptionBudget ensures availability during updates
- Anti-affinity rules spread pods across nodes

### Backup Strategy
```bash
# Create backup of persistent data
kubectl exec scheduled-db-0 -- tar czf /tmp/backup.tar.gz -C /data .
kubectl cp scheduled-db-0:/tmp/backup.tar.gz ./backup-$(date +%Y%m%d).tar.gz
```

### Monitoring Integration

The deployment is ready for Prometheus monitoring:
- Annotations for automatic discovery
- Health endpoints exposed
- Metrics available at `/health`

## 🚨 Alerts and Notifications

Recommended alerts:
- Pod restart rate
- Raft cluster health
- Job execution failures
- Storage utilization
- Network connectivity issues

## 📚 Additional Resources

- [Kubernetes Documentation](https://kubernetes.io/docs/)
- [StatefulSet Best Practices](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
- [Raft Consensus Algorithm](https://raft.github.io/)
- [Project Repository](https://github.com/your-org/scheduled-db)

## 🤝 Contributing

1. Test changes locally with Kind/Minikube
2. Validate manifests: `kubectl apply --dry-run=client -k k8s/`
3. Run security scans: `kubesec scan k8s/*.yaml`
4. Update documentation for any configuration changes

---

For questions or issues, please check the main project documentation or open an issue in the repository.