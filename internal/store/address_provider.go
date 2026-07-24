//go:build !wasm

package store

import (
	"strings"

	"github.com/manudiv16/pkgcluster"

	"github.com/hashicorp/raft"
)

// NewRaftAddressProvider creates a raft.ServerAddressProvider that resolves
// server IDs to K8s StatefulSet DNS names using pkgcluster.StatefulSetDNSName.
// When a server ID is not a StatefulSet pod name (doesn't start with
// "scheduled-db-"), it returns the ID as-is as a fallback.
func NewRaftAddressProvider(serviceName, namespace, domain string, port int) raft.ServerAddressProvider {
	return &dnsAddressProvider{
		serviceName: serviceName,
		namespace:   namespace,
		domain:      domain,
		port:        port,
	}
}

type dnsAddressProvider struct {
	serviceName string
	namespace   string
	domain      string
	port        int
}

func (p *dnsAddressProvider) ServerAddr(id raft.ServerID) (raft.ServerAddress, error) {
	serverID := string(id)
	if strings.HasPrefix(serverID, "scheduled-db-") {
		return raft.ServerAddress(pkgcluster.StatefulSetDNSName(serverID, p.serviceName, p.namespace, p.domain, p.port)), nil
	}
	return raft.ServerAddress(serverID), nil
}
