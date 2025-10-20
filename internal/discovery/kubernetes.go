package discovery

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"scheduled-db/internal/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubernetesStrategy implements discovery using Kubernetes API
type KubernetesStrategy struct {
	config           Config
	clientset        *kubernetes.Clientset
	namespace        string
	selector         string
	lastQuorumStatus string
}

// NewKubernetesStrategy creates a new Kubernetes discovery strategy
func NewKubernetesStrategy(config Config) (Strategy, error) {
	// Create in-cluster config
	kubeConfig, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create in-cluster config: %v", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}

	namespace := config.Namespace
	if namespace == "" {
		namespace = os.Getenv("POD_NAMESPACE")
		if namespace == "" {
			namespace = "default"
		}
	}

	// Create label selector for service discovery
	selector := fmt.Sprintf("app=%s", config.ServiceName)

	return &KubernetesStrategy{
		config:    config,
		clientset: clientset,
		namespace: namespace,
		selector:  selector,
	}, nil
}

// Discover returns current nodes by querying Kubernetes pods directly
func (k *KubernetesStrategy) Discover(ctx context.Context) ([]Node, error) {
	// Use pod listing instead of endpoints to get better control over health filtering
	pods, err := k.clientset.CoreV1().Pods(k.namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: k.selector,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %v", err)
	}

	var nodes []Node
	healthyCount := 0
	totalCount := len(pods.Items)
	readyNodes := 0

	for _, pod := range pods.Items {
		// Check if pod is healthy (running and ready)
		isHealthy := k.isPodHealthy(&pod)
		if isHealthy {
			healthyCount++
			readyNodes++
		}

		// Include node if:
		// 1. It's healthy, OR
		// 2. It's running but not ready (potential cluster join issues)
		shouldInclude := isHealthy || (pod.Status.Phase == corev1.PodRunning && !isHealthy)

		if !shouldInclude {
			logger.Debug("skipping pod %s in phase %s (not running)", pod.Name, pod.Status.Phase)
			continue
		}

		// Extract pod information
		nodeID := pod.Name
		podIP := pod.Status.PodIP
		if podIP == "" {
			logger.Debug("pod %s has no IP, skipping", pod.Name)
			continue
		}

		// Use pod IP directly (simpler and more reliable than complex hostnames)
		nodeAddress := podIP

		// Get Raft port from pod spec
		raftPort := 7000 // default
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == "raft" {
					raftPort = int(port.ContainerPort)
					break
				}
			}
		}

		node := Node{
			ID:      nodeID,
			Address: nodeAddress,
			Port:    raftPort,
			Meta: map[string]string{
				"pod_ip":    podIP,
				"pod_phase": string(pod.Status.Phase),
				"ready":     fmt.Sprintf("%v", isHealthy),
			},
		}

		// Add metadata from pod annotations
		if pod.Annotations != nil {
			for key, value := range pod.Annotations {
				if key == "scheduled-db/node-id" {
					node.ID = value
				}
				node.Meta[key] = value
			}
		}

		nodes = append(nodes, node)
	}

	if readyNodes < totalCount {
		logger.Debug("kubernetes discovery found %d total nodes (%d ready, %d not-ready-but-running)",
			len(nodes), readyNodes, len(nodes)-readyNodes)
	} else {
		logger.Debug("kubernetes discovery found %d healthy nodes", len(nodes))
	}

	// Check for quorum and implement split-brain protection
	if err := k.checkQuorumAndHandleSplitBrain(healthyCount, totalCount); err != nil {
		logger.Warn("quorum check failed: %v", err)
		// Don't return error, but log the issue
	}

	return nodes, nil
}

// isPodHealthy checks if a pod is healthy and ready
func (k *KubernetesStrategy) isPodHealthy(pod *corev1.Pod) bool {
	// Pod must be running
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	// Pod must be ready (all readiness probes passing)
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

// checkQuorumAndHandleSplitBrain implements quorum logic similar to RabbitMQ RA
func (k *KubernetesStrategy) checkQuorumAndHandleSplitBrain(healthyNodes, totalNodes int) error {
	expectedClusterSize := k.getExpectedClusterSize()

	// If we have majority of expected nodes, we're good
	if healthyNodes > expectedClusterSize/2 {
		// Log cluster stabilization on first quorum achievement
		if k.lastQuorumStatus != "healthy" {
			logger.Info("✅ CLUSTER STABILIZED: %d/%d nodes healthy, quorum established", 
				healthyNodes, expectedClusterSize)
			k.lastQuorumStatus = "healthy"
		} else {
			logger.Debug("quorum check passed: %d healthy nodes out of %d expected (majority achieved)",
				healthyNodes, expectedClusterSize)
		}
		return nil
	}

	// If we have exactly half or less, we might be in a split-brain situation
	if healthyNodes <= expectedClusterSize/2 {
		k.lastQuorumStatus = "unhealthy"
		logger.Warn("WARNING: potential split-brain detected - only %d healthy nodes out of %d expected",
			healthyNodes, expectedClusterSize)

		// In a production environment, you might want to:
		// 1. Stop accepting writes
		// 2. Shut down nodes in minority partition
		// 3. Wait for network partition to heal

		// For now, just log the warning
		return fmt.Errorf("insufficient quorum: %d nodes, need majority of %d", healthyNodes, expectedClusterSize)
	}

	return nil
}

// getExpectedClusterSize returns the expected cluster size from configuration
func (k *KubernetesStrategy) getExpectedClusterSize() int {
	if clusterSizeStr := os.Getenv("CLUSTER_SIZE"); clusterSizeStr != "" {
		if size, err := strconv.Atoi(clusterSizeStr); err == nil && size > 0 {
			return size
		}
	}
	return 3 // default cluster size
}

// Watch monitors for changes in Kubernetes endpoints
func (k *KubernetesStrategy) Watch(ctx context.Context, callback func([]Node)) error {
	watcher, err := k.clientset.CoreV1().Endpoints(k.namespace).Watch(
		ctx,
		metav1.ListOptions{
			FieldSelector: fmt.Sprintf("metadata.name=%s", k.config.ServiceName),
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Initial discovery
	nodes, err := k.Discover(ctx)
	if err != nil {
		logger.Error("initial discovery failed: %v", err)
	} else {
		callback(nodes)
	}

	// Watch for changes
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				// Watcher closed, try to restart
				logger.Warn("kubernetes watcher closed, restarting...")
				return k.Watch(ctx, callback)
			}

			switch event.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				// Re-discover nodes on any endpoint change
				nodes, err := k.Discover(ctx)
				if err != nil {
					logger.Error("failed to re-discover nodes: %v", err)
					continue
				}
				callback(nodes)
			}
		}
	}
}

// Register creates/updates a headless service for this node
func (k *KubernetesStrategy) Register(ctx context.Context, node Node) error {
	// In Kubernetes, registration is typically handled by the deployment
	// But we can update pod annotations to provide node metadata

	podName := k.getPodName()
	if podName == "" {
		logger.Debug("could not determine pod name, skipping registration")
		return nil
	}

	pod, err := k.clientset.CoreV1().Pods(k.namespace).Get(
		ctx,
		podName,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to get pod: %v", err)
	}

	// Update pod annotations with node metadata
	if pod.Annotations == nil {
		pod.Annotations = make(map[string]string)
	}

	pod.Annotations["scheduled-db/node-id"] = node.ID
	pod.Annotations["scheduled-db/address"] = node.Address
	pod.Annotations["scheduled-db/port"] = strconv.Itoa(node.Port)
	pod.Annotations["scheduled-db/registered-at"] = time.Now().Format(time.RFC3339)

	// Add custom metadata
	for key, value := range node.Meta {
		pod.Annotations[fmt.Sprintf("scheduled-db/meta-%s", key)] = value
	}

	_, err = k.clientset.CoreV1().Pods(k.namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update pod annotations: %v", err)
	}

	logger.Info("registered node %s in kubernetes", node.ID)
	return nil
}

// Deregister removes node-specific annotations
func (k *KubernetesStrategy) Deregister(ctx context.Context) error {
	podName := k.getPodName()
	if podName == "" {
		return nil
	}

	pod, err := k.clientset.CoreV1().Pods(k.namespace).Get(
		ctx,
		podName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil // Pod might already be deleted
	}

	// Remove scheduled-db annotations
	if pod.Annotations != nil {
		for key := range pod.Annotations {
			if len(key) > 12 && key[:12] == "scheduled-db" {
				delete(pod.Annotations, key)
			}
		}

		_, err = k.clientset.CoreV1().Pods(k.namespace).Update(ctx, pod, metav1.UpdateOptions{})
		if err != nil {
			logger.Error("failed to update pod annotations during deregister: %v", err)
		}
	}

	logger.Info("deregistered node from kubernetes")
	return nil
}

// Name returns the strategy name
func (k *KubernetesStrategy) Name() string {
	return "kubernetes"
}

// getPodName tries to determine the current pod name from environment
func (k *KubernetesStrategy) getPodName() string {
	// Try common environment variables set by Kubernetes
	podName := os.Getenv("HOSTNAME")
	if podName == "" {
		podName = os.Getenv("POD_NAME")
	}
	return podName
}
