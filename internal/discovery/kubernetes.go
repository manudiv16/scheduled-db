package discovery

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// KubernetesStrategy implements discovery using Kubernetes API
type KubernetesStrategy struct {
	config    Config
	clientset *kubernetes.Clientset
	namespace string
	selector  string
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
		namespace = "default"
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

// Discover returns current nodes by querying Kubernetes endpoints
func (k *KubernetesStrategy) Discover(ctx context.Context) ([]Node, error) {
	endpoints, err := k.clientset.CoreV1().Endpoints(k.namespace).Get(
		ctx,
		k.config.ServiceName,
		metav1.GetOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints: %v", err)
	}

	var nodes []Node
	for _, subset := range endpoints.Subsets {
		for _, address := range subset.Addresses {
			for _, port := range subset.Ports {
				// Extract node ID from pod name or use address
				nodeID := address.IP
				if address.TargetRef != nil && address.TargetRef.Name != "" {
					nodeID = address.TargetRef.Name
				}

				node := Node{
					ID:      nodeID,
					Address: address.IP,
					Port:    int(port.Port),
					Meta:    make(map[string]string),
				}

				// Add metadata from pod annotations if available
				if address.TargetRef != nil {
					pod, err := k.clientset.CoreV1().Pods(k.namespace).Get(
						ctx,
						address.TargetRef.Name,
						metav1.GetOptions{},
					)
					if err == nil && pod.Annotations != nil {
						for key, value := range pod.Annotations {
							if key == "scheduled-db/node-id" {
								node.ID = value
							}
							node.Meta[key] = value
						}
					}
				}

				nodes = append(nodes, node)
			}
		}
	}

	log.Printf("Kubernetes discovery found %d nodes", len(nodes))
	return nodes, nil
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
		log.Printf("Initial discovery failed: %v", err)
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
				log.Println("Kubernetes watcher closed, restarting...")
				return k.Watch(ctx, callback)
			}

			switch event.Type {
			case watch.Added, watch.Modified, watch.Deleted:
				// Re-discover nodes on any endpoint change
				nodes, err := k.Discover(ctx)
				if err != nil {
					log.Printf("Failed to re-discover nodes: %v", err)
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
		log.Println("Could not determine pod name, skipping registration")
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

	log.Printf("Registered node %s in Kubernetes", node.ID)
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
			log.Printf("Failed to update pod annotations during deregister: %v", err)
		}
	}

	log.Printf("Deregistered node from Kubernetes")
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
