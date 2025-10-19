package discovery

import (
	"context"
	"fmt"
	"scheduled-db/internal/logger"
	"net"
	"os"
	"strconv"
	"strings"
)

// StaticStrategy implements discovery using a static list of peers
type StaticStrategy struct {
	config Config
	peers  []string
}

// NewStaticStrategy creates a new static discovery strategy
func NewStaticStrategy(config Config) (Strategy, error) {
	// Peers can be provided in config.Meta["peers"] or through environment
	var peers []string

	if peerList, exists := config.Meta["peers"]; exists {
		peers = strings.Split(peerList, ",")
		for i, peer := range peers {
			peers[i] = strings.TrimSpace(peer)
		}
	}

	// Also check environment variable as fallback
	if len(peers) == 0 {
		if envPeers := os.Getenv("STATIC_PEERS"); envPeers != "" {
			peers = strings.Split(envPeers, ",")
			for i, peer := range peers {
				peers[i] = strings.TrimSpace(peer)
			}
		}
	}

	return &StaticStrategy{
		config: config,
		peers:  peers,
	}, nil
}

// Discover returns nodes from static peer list
func (s *StaticStrategy) Discover(ctx context.Context) ([]Node, error) {
	if len(s.peers) == 0 {
		logger.Debug("Static discovery: no peers configured")
		return []Node{}, nil
	}

	var nodes []Node
	for i, peer := range s.peers {
		node, err := s.parseNodeFromAddress(peer, i)
		if err != nil {
			logger.Debug("Failed to parse peer %s: %v", peer, err)
			continue
		}
		nodes = append(nodes, node)
	}

	logger.Debug("Static discovery found %d nodes", len(nodes))
	return nodes, nil
}

// Watch monitors for changes (static list doesn't change, but we maintain interface)
func (s *StaticStrategy) Watch(ctx context.Context, callback func([]Node)) error {
	// For static strategy, send initial discovery and then wait
	nodes, err := s.Discover(ctx)
	if err != nil {
		return err
	}

	callback(nodes)

	// Keep watching until context is cancelled (no changes expected for static)
	<-ctx.Done()
	return ctx.Err()
}

// Register adds local node info (no-op for static strategy)
func (s *StaticStrategy) Register(ctx context.Context, node Node) error {
	logger.Debug("Static strategy: registered local node %s (address: %s:%d)",
		node.ID, node.Address, node.Port)
	// Static strategy doesn't need to register anywhere
	return nil
}

// Deregister removes local node info (no-op for static strategy)
func (s *StaticStrategy) Deregister(ctx context.Context) error {
	logger.Debug("Static strategy: deregistered local node")
	return nil
}

// Name returns the strategy name
func (s *StaticStrategy) Name() string {
	return "static"
}

// parseNodeFromAddress converts address string to Node
func (s *StaticStrategy) parseNodeFromAddress(address string, index int) (Node, error) {
	// Parse address in format "host:port" or "ip:port"
	host, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return Node{}, fmt.Errorf("invalid address format %s: %v", address, err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Node{}, fmt.Errorf("invalid port in address %s: %v", address, err)
	}

	// Try to resolve hostname to IP
	var ipAddress string
	if ip := net.ParseIP(host); ip != nil {
		// Already an IP address
		ipAddress = host
	} else {
		// Resolve hostname
		ips, err := net.LookupIP(host)
		if err != nil {
			logger.Debug("Warning: failed to resolve %s, using as-is: %v", host, err)
			ipAddress = host
		} else {
			// Prefer IPv4
			for _, ip := range ips {
				if ip.To4() != nil {
					ipAddress = ip.String()
					break
				}
			}
			if ipAddress == "" && len(ips) > 0 {
				ipAddress = ips[0].String()
			}
		}
	}

	// Generate node ID based on address if not specified
	nodeID := fmt.Sprintf("static-node-%d", index)
	if s.config.NodeID != "" && index == 0 {
		// First node could be ourselves
		nodeID = s.config.NodeID
	}

	node := Node{
		ID:      nodeID,
		Address: ipAddress,
		Port:    port,
		Meta: map[string]string{
			"discovery_method": "static",
			"original_address": address,
			"resolved_from":    host,
		},
	}

	return node, nil
}

// UpdatePeers allows dynamic updating of peer list (useful for configuration changes)
func (s *StaticStrategy) UpdatePeers(peers []string) {
	s.peers = make([]string, len(peers))
	copy(s.peers, peers)
	logger.Debug("Static strategy: updated peer list with %d peers", len(peers))
}

// GetPeers returns current peer list
func (s *StaticStrategy) GetPeers() []string {
	peers := make([]string, len(s.peers))
	copy(peers, s.peers)
	return peers
}
