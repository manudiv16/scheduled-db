package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"scheduled-db/internal/store"
)

// DiscoveryManager manages cluster discovery and integrates with Raft
type DiscoveryManager struct {
	strategy  Strategy
	store     *store.Store
	config    Config
	ctx       context.Context
	cancel    context.CancelFunc
	isRunning bool
}

// DiscoveryConfig extends the base config with integration settings
type DiscoveryConfig struct {
	Config
	Strategy         StrategyType      `json:"strategy"`
	AutoJoin         bool              `json:"auto_join"`
	RaftPort         int               `json:"raft_port"`
	HTTPPort         int               `json:"http_port"`
	UpdateInterval   time.Duration     `json:"update_interval"`
	KubernetesConfig *KubernetesConfig `json:"kubernetes,omitempty"`
	GossipConfig     *GossipConfig     `json:"gossip,omitempty"`
	DNSConfig        *DNSConfig        `json:"dns,omitempty"`
}

type KubernetesConfig struct {
	InCluster    bool   `json:"in_cluster"`
	KubeConfig   string `json:"kubeconfig,omitempty"`
	PodNamespace string `json:"pod_namespace"`
	ServiceName  string `json:"service_name"`
}

type GossipConfig struct {
	BindPort      int      `json:"bind_port"`
	SeedNodes     []string `json:"seed_nodes,omitempty"`
	EncryptionKey string   `json:"encryption_key,omitempty"`
}

type DNSConfig struct {
	Domain       string        `json:"domain"`
	PollInterval time.Duration `json:"poll_interval"`
}

// NewDiscoveryManager creates a new discovery manager
func NewDiscoveryManager(config DiscoveryConfig, store *store.Store) (*DiscoveryManager, error) {
	// Create strategy based on configuration
	factory := &Factory{}
	strategy, err := factory.NewStrategy(config.Strategy, config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery strategy: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DiscoveryManager{
		strategy:  strategy,
		store:     store,
		config:    config.Config,
		ctx:       ctx,
		cancel:    cancel,
		isRunning: false,
	}, nil
}

// Start begins the discovery process
func (dm *DiscoveryManager) Start() error {
	if dm.isRunning {
		return fmt.Errorf("discovery manager already running")
	}

	// Register this node
	localNode := Node{
		ID:      dm.config.NodeID,
		Address: dm.getLocalAddress(),
		Port:    dm.getRaftPort(),
		Meta: map[string]string{
			"service":    dm.config.ServiceName,
			"started_at": time.Now().Format(time.RFC3339),
			"version":    "1.0.0",
		},
	}

	if err := dm.strategy.Register(dm.ctx, localNode); err != nil {
		return fmt.Errorf("failed to register node: %v", err)
	}

	// Start watching for cluster changes
	go func() {
		if err := dm.strategy.Watch(dm.ctx, dm.onNodesChanged); err != nil {
			if err != context.Canceled {
				log.Printf("Discovery watch error: %v", err)
			}
		}
	}()

	dm.isRunning = true
	log.Printf("Discovery manager started using %s strategy", dm.strategy.Name())
	return nil
}

// Stop shuts down the discovery manager
func (dm *DiscoveryManager) Stop() error {
	if !dm.isRunning {
		return nil
	}

	log.Printf("Stopping discovery manager...")

	// Cancel context to stop all goroutines
	dm.cancel()

	// Quick deregistration with short timeout
	log.Printf("Deregistering node from discovery...")
	deregDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		deregDone <- dm.strategy.Deregister(ctx)
	}()

	select {
	case err := <-deregDone:
		if err != nil {
			log.Printf("Error during deregistration: %v", err)
		}
	case <-time.After(3 * time.Second):
		log.Printf("Deregistration timeout, forcing shutdown...")
	}

	dm.isRunning = false
	log.Printf("Discovery manager stopped")
	return nil
}

// onNodesChanged handles cluster membership changes
func (dm *DiscoveryManager) onNodesChanged(nodes []Node) {
	log.Printf("Cluster membership changed: %d nodes discovered", len(nodes))

	// Filter out this node and convert to Raft peer addresses
	var peers []string
	localNodeID := dm.config.NodeID

	for _, node := range nodes {
		if node.ID == localNodeID {
			continue // Skip local node
		}

		// Convert to Raft address format (use Raft port, not gossip port)
		raftPort := dm.getRaftPortFromNode(node)
		peerAddr := fmt.Sprintf("%s:%d", node.Address, raftPort)
		peers = append(peers, peerAddr)

		log.Printf("Discovered peer: %s (id: %s)", peerAddr, node.ID)
	}

	// Integrate discovered peers with Raft cluster
	if len(peers) > 0 {
		log.Printf("Discovered %d Raft peers via gossip: %v", len(peers), peers)

		if dm.store.IsLeader() {
			// If we're leader, try to add new peers to our cluster
			dm.updateRaftPeers(peers)
		} else {
			// If we're not leader, try to join cluster via HTTP API
			leader := dm.store.GetLeader()
			if leader == "" {
				log.Printf("No leader found, attempting to join via discovered nodes")
				dm.attemptJoinViaHTTP(nodes)
			} else {
				log.Printf("Already part of cluster with leader: %s", leader)
			}
		}
	}
}

// updateRaftPeers adds discovered peers to the Raft cluster
func (dm *DiscoveryManager) updateRaftPeers(peers []string) {
	log.Printf("Leader attempting to add peers to cluster: %v", peers)

	// Get current cluster configuration
	servers, err := dm.store.GetClusterConfiguration()
	if err != nil {
		log.Printf("Failed to get cluster configuration: %v", err)
		return
	}

	// Build map of existing peers
	existing := make(map[string]bool)
	for _, server := range servers {
		existing[string(server.Address)] = true
	}

	// Add new peers that aren't already in the cluster
	for i, peer := range peers {
		if !existing[peer] {
			peerID := fmt.Sprintf("discovered-peer-%d", i)
			log.Printf("Adding new peer %s (%s) to Raft cluster", peerID, peer)

			if err := dm.store.AddPeer(peerID, peer); err != nil {
				log.Printf("Failed to add peer %s: %v", peer, err)
			}
		} else {
			log.Printf("Peer %s already in cluster", peer)
		}
	}
}

// GetCurrentNodes returns the currently discovered nodes
func (dm *DiscoveryManager) GetCurrentNodes() ([]Node, error) {
	return dm.strategy.Discover(dm.ctx)
}

// GetStrategy returns the current discovery strategy name
func (dm *DiscoveryManager) GetStrategy() string {
	return dm.strategy.Name()
}

// Helper methods

func (dm *DiscoveryManager) getLocalAddress() string {
	// Try to get local IP address
	if addr := os.Getenv("POD_IP"); addr != "" {
		return addr // Kubernetes pod IP
	}

	if addr := os.Getenv("HOST_IP"); addr != "" {
		return addr // Host IP from environment
	}

	// Fallback to localhost
	return "127.0.0.1"
}

func (dm *DiscoveryManager) getRaftPort() int {
	if portStr := os.Getenv("RAFT_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			return port
		}
	}
	return 7000 // Default Raft port
}

// CreateDiscoveryConfig creates a discovery config based on environment
func CreateDiscoveryConfig(nodeID, serviceName string) DiscoveryConfig {
	strategyStr := os.Getenv("DISCOVERY_STRATEGY")
	if strategyStr == "" {
		strategyStr = "static" // Default to static
	}

	var strategy StrategyType
	switch strategyStr {
	case "kubernetes":
		strategy = StrategyKubernetes
	case "dns":
		strategy = StrategyDNS
	case "gossip":
		strategy = StrategyGossip
	case "consul":
		strategy = StrategyConsul
	default:
		strategy = StrategyStatic
	}

	config := DiscoveryConfig{
		Config: Config{
			NodeID:      nodeID,
			ServiceName: serviceName,
			Namespace:   getEnvOrDefault("NAMESPACE", "default"),
			Interval:    30 * time.Second,
			Meta:        make(map[string]string),
		},
		Strategy:       strategy,
		AutoJoin:       getEnvBoolOrDefault("AUTO_JOIN", true),
		UpdateInterval: 30 * time.Second,
	}

	// Add strategy-specific configuration
	switch strategy {
	case StrategyKubernetes:
		config.KubernetesConfig = &KubernetesConfig{
			InCluster:    getEnvBoolOrDefault("KUBERNETES_IN_CLUSTER", true),
			PodNamespace: getEnvOrDefault("POD_NAMESPACE", "default"),
			ServiceName:  serviceName,
		}
	case StrategyGossip:
		config.GossipConfig = &GossipConfig{
			BindPort: getEnvIntOrDefault("GOSSIP_PORT", 7946),
		}
		if seeds := os.Getenv("GOSSIP_SEEDS"); seeds != "" {
			config.Config.Meta["peers"] = seeds
		}
	case StrategyDNS:
		config.DNSConfig = &DNSConfig{
			Domain:       getEnvOrDefault("DNS_DOMAIN", "cluster.local"),
			PollInterval: 30 * time.Second,
		}
	case StrategyStatic:
		if peers := os.Getenv("STATIC_PEERS"); peers != "" {
			config.Config.Meta["peers"] = peers
		}
	}

	return config
}

// Helper functions for environment variables
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvBoolOrDefault(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// getRaftPortFromNode extracts Raft port from discovered node
func (dm *DiscoveryManager) getRaftPortFromNode(node Node) int {
	// Try to get Raft port from node metadata
	if raftPortStr, exists := node.Meta["raft_port"]; exists {
		if port, err := strconv.Atoi(raftPortStr); err == nil {
			return port
		}
	}

	// Fallback: assume Raft port is gossip port - 946 + 7000
	// gossip: 7946 -> raft: 7000
	// gossip: 7947 -> raft: 7001
	// gossip: 7948 -> raft: 7002
	if node.Port >= 7946 {
		return node.Port - 946
	}

	// Default fallback
	return 7000
}

// joinRaftCluster attempts to join an existing Raft cluster

// attemptJoinViaHTTP tries to join cluster via HTTP API
func (dm *DiscoveryManager) attemptJoinViaHTTP(nodes []Node) {
	localNodeID := dm.config.NodeID
	localRaftPort := dm.getRaftPort()
	localRaftAddr := fmt.Sprintf("127.0.0.1:%d", localRaftPort)

	for _, node := range nodes {
		if node.ID == localNodeID {
			continue // Skip self
		}

		// Calculate HTTP port from Raft port (same logic as in handlers.go)
		raftPort := dm.getRaftPortFromNode(node)
		var httpPort int
		if raftPort >= 7000 && raftPort <= 7010 {
			httpPort = raftPort + 1080 // 7000->8080, 7001->8081, etc.
		} else {
			httpPort = raftPort + 80 // Default offset
		}
		healthURL := fmt.Sprintf("http://%s:%d/health", node.Address, httpPort)

		log.Printf("Checking if node %s is leader via %s", node.ID, healthURL)

		// Check if this node is a leader
		if dm.isNodeLeader(healthURL) {
			log.Printf("Found leader %s, attempting to join cluster", node.ID)

			joinURL := fmt.Sprintf("http://%s:%d/join", node.Address, httpPort)
			if dm.requestJoin(joinURL, localNodeID, localRaftAddr) {
				log.Printf("Successfully joined cluster via leader %s", node.ID)
				return
			}
		}
	}

	log.Printf("No leader found or join failed, will retry on next discovery cycle")
}

// isNodeLeader checks if a node is the leader
func (dm *DiscoveryManager) isNodeLeader(healthURL string) bool {
	resp, err := http.Get(healthURL)
	if err != nil {
		log.Printf("Failed to contact %s: %v", healthURL, err)
		return false
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		log.Printf("Failed to decode health response: %v", err)
		return false
	}

	role, ok := health["role"].(string)
	return ok && role == "leader"
}

// requestJoin sends a join request to the leader
func (dm *DiscoveryManager) requestJoin(joinURL, nodeID, address string) bool {
	joinReq := map[string]string{
		"node_id": nodeID,
		"address": address,
	}

	jsonData, err := json.Marshal(joinReq)
	if err != nil {
		log.Printf("Failed to marshal join request: %v", err)
		return false
	}

	resp, err := http.Post(joinURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Failed to send join request to %s: %v", joinURL, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Join request failed with status %d", resp.StatusCode)
		return false
	}

	var joinResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&joinResp); err != nil {
		log.Printf("Failed to decode join response: %v", err)
		return false
	}

	success, _ := joinResp["success"].(bool)
	message, _ := joinResp["message"].(string)

	if success {
		log.Printf("Join successful: %s", message)
		return true
	}

	log.Printf("Join failed: %s", message)
	return false
}
