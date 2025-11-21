package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/store"

	"github.com/hashicorp/raft"
)

// DiscoveryManager manages cluster discovery and integrates with Raft
type DiscoveryManager struct {
	strategy         Strategy
	store            *store.Store
	config           Config
	ctx              context.Context
	cancel           context.CancelFunc
	isRunning        bool
	shutdownCallback func() error
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
func NewDiscoveryManager(config DiscoveryConfig, store *store.Store, shutdownCallback func() error) (*DiscoveryManager, error) {
	// Create strategy based on configuration
	factory := &Factory{}
	strategy, err := factory.NewStrategy(config.Strategy, config.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create discovery strategy: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &DiscoveryManager{
		strategy:         strategy,
		store:            store,
		config:           config.Config,
		ctx:              ctx,
		cancel:           cancel,
		isRunning:        false,
		shutdownCallback: shutdownCallback,
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
				logger.Error("discovery watch error: %v", err)
			}
		}
	}()

	dm.isRunning = true
	logger.Info("discovery manager started using %s strategy", dm.strategy.Name())
	return nil
}

// Stop shuts down the discovery manager
func (dm *DiscoveryManager) Stop() error {
	if !dm.isRunning {
		return nil
	}

	logger.Info("stopping discovery manager...")

	// Cancel context to stop all goroutines
	dm.cancel()

	// Quick deregistration with short timeout
	logger.Debug("deregistering node from discovery...")
	deregDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		deregDone <- dm.strategy.Deregister(ctx)
	}()

	select {
	case err := <-deregDone:
		if err != nil {
			logger.Error("error during deregistration: %v", err)
		}
	case <-time.After(3 * time.Second):
		logger.Warn("deregistration timeout, forcing shutdown...")
	}

	dm.isRunning = false
	logger.Info("discovery manager stopped")
	return nil
}

// onNodesChanged handles cluster membership changes
func (dm *DiscoveryManager) onNodesChanged(nodes []Node) {
	logger.Debug("Cluster membership changed: %d nodes discovered", len(nodes))

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

		logger.Debug("Discovered peer: %s (id: %s)", peerAddr, node.ID)
	}

	// Integrate discovered peers with Raft cluster
	if len(peers) > 0 {
		logger.Debug("Discovered %d Raft peers via kubernetes: %v", len(peers), peers)

		if dm.store.IsLeader() {
			// If we're leader, try to add new peers to our cluster and clean dead ones
			dm.updateRaftPeersWithNodes(nodes)
		} else {
			// If we're not leader, check the cluster state
			leader := dm.store.GetLeader()
			if leader == "" {
				logger.Debug("No leader found, attempting to find and join existing cluster")
				// First try to join an existing cluster before trying to bootstrap
				if !dm.attemptJoinExistingCluster(nodes) {
					logger.Debug("No existing cluster found, checking if this node should bootstrap")
					dm.handleLeaderlessCluster(nodes)
				}
			} else {
				// Verify if leader is actually alive before assuming we're part of cluster
				if dm.isLeaderAlive(leader) {
					logger.Debug("Already part of cluster with leader: %s", leader)
				} else {
					logger.Debug("Leader %s appears to be dead, attempting to join other clusters", leader)
					if !dm.attemptJoinExistingCluster(nodes) {
						dm.handleDeadLeader(nodes, leader)
					}
				}
			}
		}
	} else {
		// No peers discovered yet, but check if we should wait for cluster formation
		nodeID := dm.config.NodeID
		if nodeID != "scheduled-db-0" && !strings.Contains(nodeID, "-0") {
			logger.Debug("Non-bootstrap node %s waiting for cluster to form", nodeID)
		}
	}
}

// updateRaftPeersWithNodes adds discovered peers to the Raft cluster using node information
func (dm *DiscoveryManager) updateRaftPeersWithNodes(nodes []Node) {
	logger.Debug("Leader attempting to update cluster membership")

	// Get current cluster configuration
	servers, err := dm.store.GetClusterConfiguration()
	if err != nil {
		logger.Debug("Failed to get cluster configuration: %v", err)
		return
	}

	// Build map of discovered nodes
	discoveredNodes := make(map[string]Node)
	for _, node := range nodes {
		discoveredNodes[node.ID] = node
	}

	localNodeID := dm.config.NodeID

	// Remove dead peers from cluster
	for _, server := range servers {
		serverID := string(server.ID)

		// Skip local node
		if serverID == localNodeID {
			continue
		}

		// If peer is not in discovered nodes, it might be dead
		if _, exists := discoveredNodes[serverID]; !exists {
			logger.Debug("Peer %s (%s) not found in discovery, checking if it's alive", serverID, server.Address)

			if !dm.isPeerAlive(string(server.Address)) {
				logger.Debug("Removing dead peer %s (%s) from Raft cluster", serverID, server.Address)

				if err := dm.store.RemovePeer(serverID); err != nil {
					logger.Debug("Failed to remove dead peer %s: %v", serverID, err)
				} else {
					logger.Debug("Successfully removed dead peer %s from cluster", serverID)
				}
			}
		}
	}

	// Build map of existing nodes
	existingByID := make(map[string]bool)
	for _, server := range servers {
		existingByID[string(server.ID)] = true
	}

	// Add new peers that aren't already in the cluster
	for _, node := range nodes {
		if node.ID == localNodeID {
			continue // Skip local node
		}

		// Use node.Address directly - it already includes the port from discovery
		peerAddr := node.Address

		// Check if peer exists by ID
		if !existingByID[node.ID] {
			logger.Debug("Adding new peer %s (%s) to Raft cluster", node.ID, peerAddr)

			// Add small delay to prevent rapid peer additions during startup
			time.Sleep(2 * time.Second)

			// Verify peer is reachable before adding to cluster
			if !dm.isPeerReachable(peerAddr) {
				logger.Debug("Peer %s (%s) not yet reachable, skipping for now", node.ID, peerAddr)
				continue
			}

			if err := dm.store.AddPeer(node.ID, peerAddr); err != nil {
				logger.Debug("Failed to add peer %s: %v", peerAddr, err)
			} else {
				logger.Debug("Successfully added peer %s to cluster", node.ID)
				// Update cluster metrics dynamically
				dm.updateClusterMetrics()
			}
		} else {
			logger.Debug("Peer %s (%s) already in cluster", node.ID, peerAddr)
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
	return 7000 // Default
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

// isPeerReachable checks if a peer is reachable on its Raft port
func (dm *DiscoveryManager) isPeerReachable(address string) bool {
	var fullAddress string

	// If address is just a node ID (like "scheduled-db-1"), convert to DNS name
	if strings.HasPrefix(address, "scheduled-db-") && !strings.Contains(address, ":") {
		fullAddress = fmt.Sprintf("%s.scheduled-db.default.svc.cluster.local:7000", address)
	} else {
		fullAddress = address
	}

	conn, err := net.DialTimeout("tcp", fullAddress, 2*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// getRaftPortFromNode extracts Raft port from discovered node
func (dm *DiscoveryManager) getRaftPortFromNode(node Node) int {
	return node.Port // Use the port directly from the node
}

// joinRaftCluster attempts to join an existing Raft cluster

// attemptJoinExistingCluster tries to find and join an existing cluster
func (dm *DiscoveryManager) attemptJoinExistingCluster(nodes []Node) bool {
	localNodeID := dm.config.NodeID

	for _, node := range nodes {
		if node.ID == localNodeID {
			continue // Skip self
		}

		// Extract hostname from node.Address (which includes port like "host:7000")
		nodeHost := node.Address
		if idx := strings.LastIndex(node.Address, ":"); idx != -1 {
			nodeHost = node.Address[:idx]
		}

		// Use HTTP port directly from environment
		httpPort := 8080
		if portStr := os.Getenv("HTTP_PORT"); portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil {
				httpPort = port
			}
		}
		healthURL := fmt.Sprintf("http://%s:%d/health", nodeHost, httpPort)

		logger.Debug("Checking if node %s is leader via %s", node.ID, healthURL)

		// Check if this node is a leader
		if dm.isNodeLeader(healthURL) {
			logger.Debug("Found existing leader %s, attempting to join cluster", node.ID)

			// Get our local address for joining
			localRaftAddr := dm.getLocalRaftAddress()
			joinURL := fmt.Sprintf("http://%s:%d/join", nodeHost, httpPort)

			if dm.requestJoin(joinURL, localNodeID, localRaftAddr) {
				logger.Debug("Successfully joined existing cluster via leader %s", node.ID)
				return true
			}
		}
	}

	logger.Debug("No existing cluster leader found among %d discovered nodes", len(nodes))
	return false
}

// attemptJoinViaHTTP tries to join cluster via HTTP API
func (dm *DiscoveryManager) attemptJoinViaHTTP(nodes []Node) {
	localNodeID := dm.config.NodeID
	localRaftAddr := dm.getLocalRaftAddress()

	logger.Debug("Local node trying to join with address: %s", localRaftAddr)

	for _, node := range nodes {
		if node.ID == localNodeID {
			continue // Skip self
		}

		// Extract hostname from node.Address (which includes port like "host:7000")
		nodeHost := node.Address
		if idx := strings.LastIndex(node.Address, ":"); idx != -1 {
			nodeHost = node.Address[:idx]
		}

		// Use HTTP port directly from environment
		httpPort := 8080
		if portStr := os.Getenv("HTTP_PORT"); portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil {
				httpPort = port
			}
		}
		healthURL := fmt.Sprintf("http://%s:%d/health", nodeHost, httpPort)

		logger.Debug("Checking if node %s is leader via %s", node.ID, healthURL)

		// Check if this node is a leader
		if dm.isNodeLeader(healthURL) {
			logger.Debug("Found leader %s, attempting to join cluster", node.ID)

			joinURL := fmt.Sprintf("http://%s:%d/join", nodeHost, httpPort)
			if dm.requestJoin(joinURL, localNodeID, localRaftAddr) {
				logger.Debug("Successfully joined cluster via leader %s", node.ID)
				return
			}
		}
	}

	logger.Debug("No leader found or join failed, will retry on next discovery cycle")
}

// getLocalRaftAddress returns the local Raft address for joining
func (dm *DiscoveryManager) getLocalRaftAddress() string {
	nodeID := dm.config.NodeID

	// For StatefulSet pods, use just the node ID - ServerAddressProvider will resolve
	if strings.HasPrefix(nodeID, "scheduled-db-") {
		return nodeID
	}

	// For non-StatefulSet pods, use IP:port format
	localRaftPort := dm.getRaftPort()
	podIP := os.Getenv("POD_IP")
	if podIP != "" {
		return fmt.Sprintf("%s:%d", podIP, localRaftPort)
	} else {
		return fmt.Sprintf("127.0.0.1:%d", localRaftPort)
	}
}

// isLeaderAlive checks if the current leader is still alive and responding
func (dm *DiscoveryManager) isLeaderAlive(leader string) bool {
	if leader == "" {
		return false
	}

	// Extract IP from leader address (format: "10.244.0.219:7000")
	parts := strings.Split(leader, ":")
	if len(parts) != 2 {
		logger.Debug("Invalid leader address format: %s", leader)
		return false
	}

	leaderIP := parts[0]
	httpPort := 8080
	if portStr := os.Getenv("HTTP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			httpPort = port
		}
	}

	healthURL := fmt.Sprintf("http://%s:%d/health", leaderIP, httpPort)

	// Use a short timeout for leader health check
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		logger.Debug("Leader %s health check failed: %v", leader, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Debug("Leader %s health check returned status: %d", leader, resp.StatusCode)
		return false
	}

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		logger.Debug("Failed to decode leader health response: %v", err)
		return false
	}

	role, ok := health["role"].(string)
	isLeader := ok && role == "leader"

	if !isLeader {
		logger.Debug("Leader %s is no longer leader, role: %s", leader, role)
	}

	return isLeader
}

// isPeerAlive checks if a peer is alive by trying to connect to its Raft address
func (dm *DiscoveryManager) isPeerAlive(peerAddr string) bool {
	if peerAddr == "" {
		return false
	}

	// Extract IP from peer address (format: "10.244.0.219:7000" or "hostname:7000")
	parts := strings.Split(peerAddr, ":")
	if len(parts) != 2 {
		logger.Debug("Invalid peer address format: %s", peerAddr)
		return false
	}

	peerHost := parts[0]
	httpPort := 8080
	if portStr := os.Getenv("HTTP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			httpPort = port
		}
	}

	healthURL := fmt.Sprintf("http://%s:%d/health", peerHost, httpPort)

	// Use a short timeout for peer health check
	client := &http.Client{
		Timeout: 3 * time.Second,
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		logger.Debug("Peer %s health check failed: %v", peerAddr, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Debug("Peer %s health check returned status: %d", peerAddr, resp.StatusCode)
		return false
	}

	logger.Debug("Peer %s is alive", peerAddr)
	return true
}

// detectSplitBrain checks if we are in a split-brain scenario
func (dm *DiscoveryManager) detectSplitBrain(discoveredNodes []Node) bool {
	// Get current cluster configuration
	servers, err := dm.store.GetClusterConfiguration()
	if err != nil {
		logger.Debug("Cannot check split-brain: failed to get cluster configuration: %v", err)
		return false
	}

	localNodeID := dm.config.NodeID
	expectedClusterSize := dm.getExpectedClusterSize()

	// Count nodes in current cluster configuration (excluding self)
	nodesInMyCluster := 0
	for _, server := range servers {
		if string(server.ID) != localNodeID {
			// Check if this node is alive and reachable
			if dm.isPeerAlive(string(server.Address)) {
				nodesInMyCluster++
			}
		}
	}

	// Count total discovered healthy nodes (excluding self)
	totalHealthyNodes := 0
	for _, node := range discoveredNodes {
		if node.ID != localNodeID {
			totalHealthyNodes++
		}
	}

	logger.Debug("Split-brain check: myCluster=%d, totalHealthy=%d, expected=%d",
		nodesInMyCluster+1, totalHealthyNodes+1, expectedClusterSize)

	// If we have fewer nodes in our cluster than total healthy nodes,
	// and there are enough nodes outside our cluster, we might be in split-brain
	nodesOutsideMyCluster := totalHealthyNodes - nodesInMyCluster

	if nodesOutsideMyCluster > 0 && nodesInMyCluster+1 < expectedClusterSize {
		logger.Debug("Potential split-brain detected: %d nodes outside my cluster", nodesOutsideMyCluster)
		return true
	}

	return false
}

// handleSplitBrain implements RabbitMQ RA-style split-brain resolution
func (dm *DiscoveryManager) handleSplitBrain(discoveredNodes []Node) {
	expectedClusterSize := dm.getExpectedClusterSize()
	localNodeID := dm.config.NodeID

	// Get current cluster configuration
	servers, err := dm.store.GetClusterConfiguration()
	if err != nil {
		logger.Debug("Cannot handle split-brain: failed to get cluster configuration: %v", err)
		return
	}

	// Count alive nodes in our current cluster
	aliveInMyCluster := 1 // Count ourselves
	for _, server := range servers {
		if string(server.ID) != localNodeID && dm.isPeerAlive(string(server.Address)) {
			aliveInMyCluster++
		}
	}

	majorityThreshold := expectedClusterSize/2 + 1

	logger.Debug("Split-brain resolution: myCluster=%d, majorityNeeded=%d, expectedTotal=%d",
		aliveInMyCluster, majorityThreshold, expectedClusterSize)

	if aliveInMyCluster < majorityThreshold {
		logger.Debug("I am in MINORITY partition (%d < %d) - shutting down to prevent split-brain",
			aliveInMyCluster, majorityThreshold)

		// Give a grace period for the situation to resolve
		logger.Debug("Waiting 30 seconds for network partition to heal...")
		time.Sleep(30 * time.Second)

		// Re-check after grace period
		newAliveCount := 1
		for _, server := range servers {
			if string(server.ID) != localNodeID && dm.isPeerAlive(string(server.Address)) {
				newAliveCount++
			}
		}

		if newAliveCount < majorityThreshold {
			logger.Debug("Still in minority after grace period (%d < %d) - executing emergency shutdown",
				newAliveCount, majorityThreshold)

			// Graceful shutdown
			if dm.shutdownCallback != nil {
				if err := dm.shutdownCallback(); err != nil {
					logger.Debug("Graceful shutdown failed: %v", err)
				}
			}

			// Force exit to prevent split-brain
			logger.Debug("SPLIT-BRAIN PREVENTION: Terminating node in minority partition")
			os.Exit(42) // Special exit code for split-brain prevention
		} else {
			logger.Debug("Network partition healed, continuing operation")
		}
	} else {
		logger.Debug("I am in MAJORITY partition (%d >= %d) - continuing operation",
			aliveInMyCluster, majorityThreshold)
	}
}

// getExpectedClusterSize returns the expected cluster size dynamically
func (dm *DiscoveryManager) getExpectedClusterSize() int {
	// If CLUSTER_SIZE is set, use it (for backwards compatibility)
	if clusterSizeStr := os.Getenv("CLUSTER_SIZE"); clusterSizeStr != "" {
		if size, err := strconv.Atoi(clusterSizeStr); err == nil && size > 0 {
			return size
		}
	}

	// Dynamic sizing: use the maximum of discovered nodes and Raft configuration
	discoveredNodes, _ := dm.strategy.Discover(dm.ctx)
	discoveredCount := len(discoveredNodes)

	// Get current Raft cluster configuration
	servers, err := dm.store.GetClusterConfiguration()
	raftCount := 0
	if err == nil {
		raftCount = len(servers)
	}

	// Use the maximum, with a minimum of 1
	expectedSize := discoveredCount
	if raftCount > expectedSize {
		expectedSize = raftCount
	}
	if expectedSize < 1 {
		expectedSize = 1
	}

	return expectedSize
}

// handleLeaderlessCluster handles the case when there's no leader
func (dm *DiscoveryManager) handleLeaderlessCluster(nodes []Node) {
	logger.Debug("Handling leaderless cluster - attempting to clean dead peers")

	// Check if we can become leader by cleaning dead peers
	servers, err := dm.store.GetClusterConfiguration()
	if err != nil {
		logger.Debug("Failed to get cluster configuration: %v", err)
		return
	}

	localNodeID := dm.config.NodeID
	deadPeers := dm.findDeadPeers(servers, nodes, localNodeID)

	if len(deadPeers) > 0 {
		logger.Debug("Found %d dead peers, attempting to remove them", len(deadPeers))

		// Try to force leadership to clean the cluster
		if dm.attemptForcedLeadership(deadPeers) {
			logger.Debug("Successfully became leader, cleaning dead peers")
			dm.cleanDeadPeersAsLeader(deadPeers)
		} else {
			logger.Debug("Could not become leader, will retry on next cycle")
		}
	} else {
		// No dead peers, try normal join
		dm.attemptJoinViaHTTP(nodes)
	}
}

// handleDeadLeader handles the case when the leader is detected as dead
func (dm *DiscoveryManager) handleDeadLeader(nodes []Node, deadLeader string) {
	logger.Debug("Handling dead leader: %s", deadLeader)

	// Get current cluster config
	servers, err := dm.store.GetClusterConfiguration()
	if err != nil {
		logger.Debug("Failed to get cluster configuration: %v", err)
		return
	}

	// Find the dead leader in the configuration
	var deadLeaderID string
	for _, server := range servers {
		if string(server.Address) == deadLeader {
			deadLeaderID = string(server.ID)
			break
		}
	}

	if deadLeaderID != "" {
		deadPeers := []string{deadLeaderID}
		logger.Debug("Found dead leader ID: %s, attempting to remove it", deadLeaderID)

		// Try to force leadership to clean the dead leader
		if dm.attemptForcedLeadership(deadPeers) {
			logger.Debug("Successfully became leader, removing dead leader")
			dm.cleanDeadPeersAsLeader(deadPeers)
		} else {
			logger.Debug("Could not become leader to remove dead leader")
			dm.attemptJoinViaHTTP(nodes)
		}
	} else {
		logger.Debug("Could not find dead leader in cluster configuration")
		dm.attemptJoinViaHTTP(nodes)
	}
}

// findDeadPeers identifies which peers in the cluster are dead
func (dm *DiscoveryManager) findDeadPeers(servers []raft.Server, aliveNodes []Node, localNodeID string) []string {
	// Build map of alive nodes
	aliveNodeIDs := make(map[string]bool)
	for _, node := range aliveNodes {
		aliveNodeIDs[node.ID] = true
	}
	aliveNodeIDs[localNodeID] = true // Local node is alive by definition

	var deadPeers []string
	for _, server := range servers {
		serverID := string(server.ID)
		if serverID == localNodeID {
			continue // Skip local node
		}

		// If server is not in alive nodes, it's potentially dead
		if !aliveNodeIDs[serverID] {
			// Double-check by trying to contact it
			if !dm.isPeerAlive(string(server.Address)) {
				logger.Debug("Confirmed dead peer: %s (%s)", serverID, server.Address)
				deadPeers = append(deadPeers, serverID)
			}
		}
	}

	return deadPeers
}

// attemptForcedLeadership tries to become leader to clean the cluster
func (dm *DiscoveryManager) attemptForcedLeadership(deadPeers []string) bool {
	logger.Debug("Attempting forced leadership to clean %d dead peers", len(deadPeers))

	// Get current cluster config
	servers, err := dm.store.GetClusterConfiguration()
	if err != nil {
		logger.Debug("Failed to get cluster configuration: %v", err)
		return false
	}

	totalNodes := len(servers)
	aliveNodes := totalNodes - len(deadPeers)

	logger.Debug("Cluster has %d total nodes, %d alive, %d dead", totalNodes, aliveNodes, len(deadPeers))

	// Check if we have majority of alive nodes to safely remove dead ones
	if aliveNodes > totalNodes/2 {
		logger.Debug("We have majority (%d > %d), attempting to force bootstrap", aliveNodes, totalNodes/2)

		// Try to trigger an election by forcing Raft to attempt leadership
		// This is a bit of a hack, but necessary for cluster recovery
		return dm.triggerEmergencyElection()
	}

	logger.Debug("Insufficient alive nodes for safe recovery (%d <= %d)", aliveNodes, totalNodes/2)
	return false
}

// triggerEmergencyElection attempts to force a Raft election
func (dm *DiscoveryManager) triggerEmergencyElection() bool {
	logger.Debug("Triggering emergency election for cluster recovery")

	// This is a simplified approach - in a real implementation you might need
	// to use Raft's internal APIs or implement a more sophisticated recovery mechanism

	// For now, we'll wait a bit and check if we became leader naturally
	time.Sleep(2 * time.Second)

	if dm.store.IsLeader() {
		logger.Debug("Successfully became leader during emergency election")
		return true
	}

	logger.Debug("Emergency election did not result in leadership")
	return false
}

// cleanDeadPeersAsLeader removes dead peers when we are the leader
func (dm *DiscoveryManager) cleanDeadPeersAsLeader(deadPeerIDs []string) {
	if !dm.store.IsLeader() {
		logger.Debug("Cannot clean dead peers: not the leader")
		return
	}

	for _, deadPeerID := range deadPeerIDs {
		logger.Debug("Removing dead peer from cluster: %s", deadPeerID)

		if err := dm.store.RemovePeer(deadPeerID); err != nil {
			logger.Debug("Failed to remove dead peer %s: %v", deadPeerID, err)
		} else {
			logger.Debug("Successfully removed dead peer %s from cluster", deadPeerID)
		}
	}

	logger.Debug("Completed cleaning %d dead peers from cluster", len(deadPeerIDs))
}

// isNodeLeader checks if a node is the leader
func (dm *DiscoveryManager) isNodeLeader(healthURL string) bool {
	resp, err := http.Get(healthURL)
	if err != nil {
		logger.Debug("Failed to contact %s: %v", healthURL, err)
		return false
	}
	defer resp.Body.Close()

	var health map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		logger.Debug("Failed to decode health response: %v", err)
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
		logger.Debug("Failed to marshal join request: %v", err)
		return false
	}

	resp, err := http.Post(joinURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Debug("Failed to send join request to %s: %v", joinURL, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logger.Debug("Join request failed with status %d", resp.StatusCode)
		return false
	}

	var joinResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&joinResp); err != nil {
		logger.Debug("Failed to decode join response: %v", err)
		return false
	}

	success, _ := joinResp["success"].(bool)
	message, _ := joinResp["message"].(string)

	if success {
		logger.Debug("Join successful: %s", message)
		return true
	}

	logger.Debug("Join failed: %s", message)
	return false
}

// updateClusterMetrics updates cluster size metrics dynamically
func (dm *DiscoveryManager) updateClusterMetrics() {
	servers, err := dm.store.GetClusterConfiguration()
	if err == nil {
		logger.Debug("Cluster size updated to %d nodes", len(servers))
		// Note: Prometheus metrics update would be added here if needed
	}
}
