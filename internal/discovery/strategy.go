package discovery

import (
	"context"
	"fmt"
	"time"
)

// Node represents a discovered node in the cluster
type Node struct {
	ID      string            `json:"id"`
	Address string            `json:"address"`
	Port    int               `json:"port"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// Strategy defines the interface for cluster discovery strategies
type Strategy interface {
	// Discover returns the list of nodes in the cluster
	Discover(ctx context.Context) ([]Node, error)

	// Watch continuously monitors for cluster changes
	Watch(ctx context.Context, callback func([]Node)) error

	// Register announces this node to the cluster
	Register(ctx context.Context, node Node) error

	// Deregister removes this node from the cluster
	Deregister(ctx context.Context) error

	// Name returns the strategy name
	Name() string
}

// Config holds common configuration for discovery strategies
type Config struct {
	NodeID      string
	ServiceName string
	Namespace   string
	Tags        []string
	Meta        map[string]string
	Interval    time.Duration
}

// StrategyType represents the type of discovery strategy
type StrategyType string

const (
	StrategyStatic     StrategyType = "static"
	StrategyKubernetes StrategyType = "kubernetes"
	StrategyDNS        StrategyType = "dns"
	StrategyGossip     StrategyType = "gossip"
	StrategyConsul     StrategyType = "consul"
)

// Factory creates discovery strategies
type Factory struct{}

// NewStrategy creates a new discovery strategy based on the type
func (f *Factory) NewStrategy(strategyType StrategyType, config Config) (Strategy, error) {
	switch strategyType {
	case StrategyStatic:
		return NewStaticStrategy(config)
	case StrategyKubernetes:
		return NewKubernetesStrategy(config)
	case StrategyDNS:
		return NewDNSStrategy(config)
	case StrategyGossip:
		return NewGossipStrategy(config)
	case StrategyConsul:
		return nil, fmt.Errorf("consul strategy not implemented yet")
	default:
		return nil, ErrUnknownStrategy
	}
}

// Event represents a cluster membership event
type Event struct {
	Type EventType `json:"type"`
	Node Node      `json:"node"`
}

type EventType string

const (
	EventNodeJoined EventType = "node_joined"
	EventNodeLeft   EventType = "node_left"
	EventNodeFailed EventType = "node_failed"
)

// Errors
var (
	ErrUnknownStrategy = fmt.Errorf("unknown discovery strategy")
	ErrNotImplemented  = fmt.Errorf("strategy not implemented")
)
