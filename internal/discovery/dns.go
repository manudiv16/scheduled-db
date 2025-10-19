package discovery

import (
	"context"
	"fmt"
	"scheduled-db/internal/logger"
	"net"
	"strconv"
	"strings"
	"time"
)

// DNSStrategy implements discovery using DNS SRV records
type DNSStrategy struct {
	config      Config
	serviceName string
	domain      string
}

// NewDNSStrategy creates a new DNS-based discovery strategy
func NewDNSStrategy(config Config) (Strategy, error) {
	serviceName := config.ServiceName
	if serviceName == "" {
		serviceName = "scheduled-db"
	}

	domain := config.Namespace
	if domain == "" {
		domain = "cluster.local" // Default Kubernetes cluster domain
	}

	return &DNSStrategy{
		config:      config,
		serviceName: serviceName,
		domain:      domain,
	}, nil
}

// Discover returns nodes by performing DNS SRV lookup
func (d *DNSStrategy) Discover(ctx context.Context) ([]Node, error) {
	// Construct DNS name for SRV lookup
	// Format: _service._protocol.domain
	srvName := fmt.Sprintf("_%s._tcp.%s", d.serviceName, d.domain)

	logger.Debug("DNS discovery querying SRV record: %s", srvName)

	// Perform SRV lookup
	_, addrs, err := net.LookupSRV("", "", srvName)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup SRV record %s: %v", srvName, err)
	}

	var nodes []Node
	for i, addr := range addrs {
		// Remove trailing dot from target
		target := strings.TrimSuffix(addr.Target, ".")

		// Try to resolve target to IP
		ips, err := net.LookupIP(target)
		if err != nil {
			logger.Debug("Failed to resolve %s: %v", target, err)
			continue
		}

		// Use first resolved IP
		var ipStr string
		for _, ip := range ips {
			if ip.To4() != nil { // Prefer IPv4
				ipStr = ip.String()
				break
			}
		}
		if ipStr == "" && len(ips) > 0 {
			ipStr = ips[0].String() // Fallback to first IP (could be IPv6)
		}

		if ipStr == "" {
			logger.Debug("No IP found for target %s", target)
			continue
		}

		// Create node
		node := Node{
			ID:      fmt.Sprintf("dns-node-%d", i),
			Address: ipStr,
			Port:    int(addr.Port),
			Meta: map[string]string{
				"discovery_method": "dns",
				"srv_target":       target,
				"srv_priority":     strconv.Itoa(int(addr.Priority)),
				"srv_weight":       strconv.Itoa(int(addr.Weight)),
			},
		}

		// Try to get node ID from TXT records
		nodeID := d.getNodeIDFromTXT(target)
		if nodeID != "" {
			node.ID = nodeID
		}

		nodes = append(nodes, node)
	}

	logger.Debug("DNS discovery found %d nodes", len(nodes))
	return nodes, nil
}

// Watch continuously monitors DNS for changes
func (d *DNSStrategy) Watch(ctx context.Context, callback func([]Node)) error {
	ticker := time.NewTicker(d.getInterval())
	defer ticker.Stop()

	// Initial discovery
	nodes, err := d.Discover(ctx)
	if err != nil {
		logger.Debug("Initial DNS discovery failed: %v", err)
	} else {
		callback(nodes)
	}

	// Track previous nodes for change detection
	previousNodes := make(map[string]Node)
	for _, node := range nodes {
		previousNodes[node.ID] = node
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			currentNodes, err := d.Discover(ctx)
			if err != nil {
				logger.Debug("DNS discovery error: %v", err)
				continue
			}

			// Check if nodes changed
			if d.nodesChanged(previousNodes, currentNodes) {
				logger.Debug("DNS discovery detected changes, notifying callback")
				callback(currentNodes)

				// Update previous nodes
				previousNodes = make(map[string]Node)
				for _, node := range currentNodes {
					previousNodes[node.ID] = node
				}
			}
		}
	}
}

// Register publishes this node's information via DNS (requires DNS server support)
func (d *DNSStrategy) Register(ctx context.Context, node Node) error {
	// DNS registration typically requires external DNS server management
	// This could be implemented with:
	// 1. Dynamic DNS updates (RFC 2136)
	// 2. External service registration (consul-template, external-dns, etc.)
	// 3. Kubernetes service/endpoint creation

	logger.Debug("DNS registration for node %s (address: %s:%d)", node.ID, node.Address, node.Port)
	logger.Debug("Note: DNS registration requires external DNS management")

	// For Kubernetes environments, we could create/update a headless service
	// For now, we'll just log the registration intent
	return nil
}

// Deregister removes this node from DNS (requires DNS server support)
func (d *DNSStrategy) Deregister(ctx context.Context) error {
	logger.Debug("DNS deregistration - requires external DNS management")
	return nil
}

// Name returns the strategy name
func (d *DNSStrategy) Name() string {
	return "dns"
}

// getNodeIDFromTXT attempts to get node ID from TXT records
func (d *DNSStrategy) getNodeIDFromTXT(target string) string {
	// Look for TXT record with node ID
	// Format: node-id=<id>
	txtName := fmt.Sprintf("_node-id.%s", target)

	txtRecords, err := net.LookupTXT(txtName)
	if err != nil {
		return ""
	}

	for _, record := range txtRecords {
		if strings.HasPrefix(record, "node-id=") {
			return strings.TrimPrefix(record, "node-id=")
		}
	}

	return ""
}

// getInterval returns the polling interval for DNS changes
func (d *DNSStrategy) getInterval() time.Duration {
	if d.config.Interval > 0 {
		return d.config.Interval
	}
	return 30 * time.Second // Default to 30 seconds
}

// nodesChanged compares two node maps to detect changes
func (d *DNSStrategy) nodesChanged(previous map[string]Node, current []Node) bool {
	// Convert current to map for easier comparison
	currentMap := make(map[string]Node)
	for _, node := range current {
		currentMap[node.ID] = node
	}

	// Check if counts differ
	if len(previous) != len(currentMap) {
		return true
	}

	// Check each node
	for id, prevNode := range previous {
		currNode, exists := currentMap[id]
		if !exists {
			return true // Node removed
		}

		// Check if node properties changed
		if prevNode.Address != currNode.Address || prevNode.Port != currNode.Port {
			return true
		}
	}

	// Check for new nodes
	for id := range currentMap {
		if _, exists := previous[id]; !exists {
			return true // New node added
		}
	}

	return false
}
