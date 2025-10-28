package store

import (
	"fmt"
	"strings"

	"github.com/hashicorp/raft"
)

// DNSAddressProvider implements ServerAddressProvider to map server IDs to DNS names
type DNSAddressProvider struct {
	serviceName string
	namespace   string
	port        int
}

// NewDNSAddressProvider creates a new DNS-based address provider
func NewDNSAddressProvider(serviceName, namespace string, port int) *DNSAddressProvider {
	return &DNSAddressProvider{
		serviceName: serviceName,
		namespace:   namespace,
		port:        port,
	}
}

// ServerAddr implements the ServerAddressProvider interface
// It maps server IDs to DNS names for StatefulSet pods
func (p *DNSAddressProvider) ServerAddr(id raft.ServerID) (raft.ServerAddress, error) {
	serverID := string(id)
	
	// For StatefulSet pods, convert to DNS name
	if strings.HasPrefix(serverID, "scheduled-db-") {
		dnsName := fmt.Sprintf("%s.%s.%s.svc.cluster.local:%d", 
			serverID, p.serviceName, p.namespace, p.port)
		return raft.ServerAddress(dnsName), nil
	}
	
	// For other server IDs, return as-is (fallback)
	return raft.ServerAddress(serverID), nil
}
