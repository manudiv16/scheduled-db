package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"scheduled-db/internal/logger"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/memberlist"
)

// GossipStrategy implements discovery using Hashicorp Memberlist (gossip protocol)
type GossipStrategy struct {
	config     Config
	memberlist *memberlist.Memberlist
	delegate   *gossipDelegate
	mutex      sync.RWMutex
	nodes      map[string]Node
	callbacks  []func([]Node)
}

// gossipDelegate implements memberlist.Delegate for handling events and metadata
type gossipDelegate struct {
	strategy *GossipStrategy
	meta     []byte
}

// NewGossipStrategy creates a new gossip-based discovery strategy
func NewGossipStrategy(config Config) (Strategy, error) {
	// Create strategy first
	strategy := &GossipStrategy{
		config:    config,
		nodes:     make(map[string]Node),
		callbacks: make([]func([]Node), 0),
	}

	// Create delegate with strategy reference
	delegate := &gossipDelegate{
		strategy: strategy,
	}

	// Prepare node metadata
	metadata := map[string]string{
		"node_id":      config.NodeID,
		"service_name": config.ServiceName,
		"timestamp":    strconv.FormatInt(time.Now().Unix(), 10),
	}
	for k, v := range config.Meta {
		metadata[k] = v
	}

	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %v", err)
	}
	delegate.meta = metaBytes

	// Create memberlist config
	mlConfig := memberlist.DefaultWANConfig()
	mlConfig.Name = config.NodeID
	mlConfig.Delegate = delegate
	mlConfig.Events = delegate

	// Use configurable bind address
	bindAddr := os.Getenv("GOSSIP_BIND_ADDR")
	if bindAddr == "" {
		bindAddr = "0.0.0.0" // Bind to all interfaces by default
	}
	mlConfig.BindAddr = bindAddr
	mlConfig.AdvertiseAddr = bindAddr

	// Configure ports and timeouts from environment or config
	gossipPort := getGossipPort()
	mlConfig.BindPort = gossipPort
	mlConfig.AdvertisePort = gossipPort
	mlConfig.ProbeInterval = 1 * time.Second
	mlConfig.ProbeTimeout = 500 * time.Millisecond
	mlConfig.GossipInterval = 200 * time.Millisecond

	// Create memberlist
	ml, err := memberlist.Create(mlConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create memberlist: %v", err)
	}

	// Set memberlist and delegate in strategy
	strategy.memberlist = ml
	strategy.delegate = delegate

	logger.Debug("Gossip strategy initialized on %s", ml.LocalNode().Addr)
	return strategy, nil
}

// getGossipPort gets gossip port from environment or default
func getGossipPort() int {
	if portStr := os.Getenv("GOSSIP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			return port
		}
	}
	return 7946 // Default port
}

// Discover returns current nodes known through gossip
func (g *GossipStrategy) Discover(ctx context.Context) ([]Node, error) {
	g.mutex.RLock()
	defer g.mutex.RUnlock()

	nodes := make([]Node, 0, len(g.nodes))
	for _, node := range g.nodes {
		nodes = append(nodes, node)
	}

	// Add local node if memberlist is available and not already present
	if g.memberlist != nil {
		localNode := g.getLocalNode()
		found := false
		for _, node := range nodes {
			if node.ID == localNode.ID {
				found = true
				break
			}
		}
		if !found {
			nodes = append(nodes, localNode)
		}
	}

	logger.Debug("Gossip discovery found %d nodes", len(nodes))
	return nodes, nil
}

// Watch continuously monitors for gossip membership changes
func (g *GossipStrategy) Watch(ctx context.Context, callback func([]Node)) error {
	g.mutex.Lock()
	g.callbacks = append(g.callbacks, callback)
	g.mutex.Unlock()

	// Send initial node list
	nodes, err := g.Discover(ctx)
	if err != nil {
		return err
	}
	callback(nodes)

	// Keep watching until context is cancelled
	<-ctx.Done()
	return ctx.Err()
}

// Register joins the gossip cluster with seed nodes
func (g *GossipStrategy) Register(ctx context.Context, node Node) error {
	// Update local metadata
	metadata := map[string]string{
		"node_id":      node.ID,
		"service_name": g.config.ServiceName,
		"address":      node.Address,
		"port":         strconv.Itoa(node.Port),
		"timestamp":    strconv.FormatInt(time.Now().Unix(), 10),
	}
	for k, v := range node.Meta {
		metadata[k] = v
	}

	metaBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal node metadata: %v", err)
	}
	g.delegate.meta = metaBytes

	// Try to join existing cluster through seed nodes from environment or DNS
	seedNodes := g.findSeedNodes()
	if len(seedNodes) > 0 {
		logger.Debug("Attempting to join gossip cluster with seed nodes: %v", seedNodes)
		numJoined, err := g.memberlist.Join(seedNodes)
		if err != nil {
			logger.Debug("Failed to join gossip cluster: %v", err)
			logger.Debug("Will continue as isolated node, retrying connections in background")
		} else {
			logger.Debug("Successfully joined gossip cluster, connected to %d seed nodes", numJoined)
		}
	} else {
		logger.Debug("No seed nodes found, starting as cluster bootstrap node")
	}

	// Add local node to nodes map
	g.mutex.Lock()
	g.nodes[node.ID] = node
	g.mutex.Unlock()

	logger.Debug("Registered node %s in gossip cluster", node.ID)
	return nil
}

// Deregister leaves the gossip cluster
func (g *GossipStrategy) Deregister(ctx context.Context) error {
	timeout := 5 * time.Second
	if err := g.memberlist.Leave(timeout); err != nil {
		logger.Debug("Error leaving gossip cluster: %v", err)
	}

	if err := g.memberlist.Shutdown(); err != nil {
		return fmt.Errorf("failed to shutdown memberlist: %v", err)
	}

	logger.Debug("Deregistered from gossip cluster")
	return nil
}

// Name returns the strategy name
func (g *GossipStrategy) Name() string {
	return "gossip"
}

// findSeedNodes discovers seed nodes from environment or DNS
func (g *GossipStrategy) findSeedNodes() []string {
	var seeds []string

	// Try environment variable first
	if seedEnv := os.Getenv("GOSSIP_SEEDS"); seedEnv != "" {
		seeds = strings.Split(seedEnv, ",")
		for i, seed := range seeds {
			seeds[i] = strings.TrimSpace(seed)
		}
		logger.Debug("Found GOSSIP_SEEDS environment variable: %v", seeds)
	}

	// Try DNS SRV lookup as fallback
	if len(seeds) == 0 && g.config.ServiceName != "" {
		dnsName := fmt.Sprintf("_%s._tcp.%s", g.config.ServiceName, g.config.Namespace)
		if _, addrs, err := net.LookupSRV("", "", dnsName); err == nil {
			for _, addr := range addrs {
				seed := fmt.Sprintf("%s:%d",
					strings.TrimSuffix(addr.Target, "."),
					addr.Port)
				seeds = append(seeds, seed)
			}
			logger.Debug("Found seeds via DNS SRV lookup: %v", seeds)
		} else {
			logger.Debug("DNS SRV lookup failed for %s: %v", dnsName, err)
		}
	}

	return seeds
}

// getLocalNode creates a Node representation of the local memberlist node
func (g *GossipStrategy) getLocalNode() Node {
	if g.memberlist == nil {
		// Fallback if memberlist not available yet
		return Node{
			ID:      g.config.NodeID,
			Address: "127.0.0.1",
			Port:    getGossipPort(),
			Meta:    make(map[string]string),
		}
	}

	local := g.memberlist.LocalNode()
	if local == nil {
		// Another fallback
		return Node{
			ID:      g.config.NodeID,
			Address: "127.0.0.1",
			Port:    getGossipPort(),
			Meta:    make(map[string]string),
		}
	}

	// Parse metadata
	var metadata map[string]string
	if len(local.Meta) > 0 {
		if err := json.Unmarshal(local.Meta, &metadata); err != nil {
			logger.Debug("Failed to unmarshal local node metadata: %v", err)
		}
	}
	if metadata == nil {
		metadata = make(map[string]string)
	}

	return Node{
		ID:      local.Name,
		Address: local.Addr.String(),
		Port:    int(local.Port),
		Meta:    metadata,
	}
}

// notifyCallbacks sends updated node list to all registered callbacks
func (g *GossipStrategy) notifyCallbacks() {
	// Only notify if we have callbacks and memberlist is ready
	if g.memberlist == nil {
		logger.Debug("Memberlist not ready, skipping callback notification")
		return
	}

	nodes, err := g.Discover(context.Background())
	if err != nil {
		logger.Debug("Failed to discover nodes for callback: %v", err)
		return
	}

	g.mutex.RLock()
	callbacks := make([]func([]Node), len(g.callbacks))
	copy(callbacks, g.callbacks)
	g.mutex.RUnlock()

	for _, callback := range callbacks {
		go callback(nodes)
	}
}

// Memberlist Delegate implementation

// NodeMeta returns metadata for the local node
func (d *gossipDelegate) NodeMeta(limit int) []byte {
	if len(d.meta) > limit {
		return d.meta[:limit]
	}
	return d.meta
}

// NotifyMsg handles incoming gossip messages (not used in this implementation)
func (d *gossipDelegate) NotifyMsg([]byte) {}

// GetBroadcasts returns pending broadcasts (not used in this implementation)
func (d *gossipDelegate) GetBroadcasts(overhead, limit int) [][]byte {
	return nil
}

// LocalState returns the local state for state sync (not used in this implementation)
func (d *gossipDelegate) LocalState(join bool) []byte {
	return nil
}

// MergeRemoteState merges remote state (not used in this implementation)
func (d *gossipDelegate) MergeRemoteState(buf []byte, join bool) {}

// Memberlist EventDelegate implementation

// NotifyJoin is called when a node joins the cluster
func (d *gossipDelegate) NotifyJoin(node *memberlist.Node) {
	if d.strategy == nil {
		logger.Debug("Strategy is nil in NotifyJoin, skipping")
		return
	}

	logger.Debug("Node joined gossip cluster: %s (%s:%d)", node.Name, node.Addr, node.Port)

	// Parse metadata
	var metadata map[string]string
	if len(node.Meta) > 0 {
		if err := json.Unmarshal(node.Meta, &metadata); err != nil {
			logger.Debug("Failed to unmarshal node metadata: %v", err)
		}
	}
	if metadata == nil {
		metadata = make(map[string]string)
	}

	// Add to nodes map
	newNode := Node{
		ID:      node.Name,
		Address: node.Addr.String(),
		Port:    int(node.Port),
		Meta:    metadata,
	}

	d.strategy.mutex.Lock()
	d.strategy.nodes[node.Name] = newNode
	d.strategy.mutex.Unlock()

	// Notify callbacks
	d.strategy.notifyCallbacks()
}

// NotifyLeave is called when a node leaves the cluster gracefully
func (d *gossipDelegate) NotifyLeave(node *memberlist.Node) {
	if d.strategy == nil {
		logger.Debug("Strategy is nil in NotifyLeave, skipping")
		return
	}

	logger.Debug("Node left gossip cluster: %s (%s:%d)", node.Name, node.Addr, node.Port)

	d.strategy.mutex.Lock()
	delete(d.strategy.nodes, node.Name)
	d.strategy.mutex.Unlock()

	// Notify callbacks
	d.strategy.notifyCallbacks()
}

// NotifyUpdate is called when a node's metadata is updated
func (d *gossipDelegate) NotifyUpdate(node *memberlist.Node) {
	if d.strategy == nil {
		logger.Debug("Strategy is nil in NotifyUpdate, skipping")
		return
	}

	logger.Debug("Node updated in gossip cluster: %s (%s:%d)", node.Name, node.Addr, node.Port)

	// Parse updated metadata
	var metadata map[string]string
	if len(node.Meta) > 0 {
		if err := json.Unmarshal(node.Meta, &metadata); err != nil {
			logger.Debug("Failed to unmarshal node metadata: %v", err)
		}
	}
	if metadata == nil {
		metadata = make(map[string]string)
	}

	// Update node in map
	updatedNode := Node{
		ID:      node.Name,
		Address: node.Addr.String(),
		Port:    int(node.Port),
		Meta:    metadata,
	}

	d.strategy.mutex.Lock()
	d.strategy.nodes[node.Name] = updatedNode
	d.strategy.mutex.Unlock()

	// Notify callbacks
	d.strategy.notifyCallbacks()
}
