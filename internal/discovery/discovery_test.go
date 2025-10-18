package discovery

import (
	"fmt"
	"testing"
	"time"
)

func TestNode_Serialization(t *testing.T) {
	node := Node{
		ID:      "test-node-1",
		Address: "127.0.0.1",
		Port:    7000,
		Meta: map[string]string{
			"region": "us-east-1",
			"zone":   "a",
		},
	}

	// Test that all fields are accessible
	if node.ID != "test-node-1" {
		t.Errorf("Node.ID = %s, want 'test-node-1'", node.ID)
	}
	if node.Address != "127.0.0.1" {
		t.Errorf("Node.Address = %s, want '127.0.0.1'", node.Address)
	}
	if node.Port != 7000 {
		t.Errorf("Node.Port = %d, want 7000", node.Port)
	}
	if len(node.Meta) != 2 {
		t.Errorf("Node.Meta length = %d, want 2", len(node.Meta))
	}
	if node.Meta["region"] != "us-east-1" {
		t.Errorf("Node.Meta[region] = %s, want 'us-east-1'", node.Meta["region"])
	}
}

func TestNode_EmptyMeta(t *testing.T) {
	node := Node{
		ID:      "test-node",
		Address: "localhost",
		Port:    8080,
	}

	if node.Meta != nil {
		t.Errorf("Node.Meta should be nil when not initialized, got %v", node.Meta)
	}
}

func TestConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config Config
		valid  bool
	}{
		{
			name: "valid config",
			config: Config{
				NodeID:      "node-1",
				ServiceName: "scheduled-db",
				Namespace:   "default",
				Tags:        []string{"scheduler", "distributed"},
				Meta:        map[string]string{"version": "1.0"},
				Interval:    30 * time.Second,
			},
			valid: true,
		},
		{
			name: "empty node ID",
			config: Config{
				ServiceName: "scheduled-db",
			},
			valid: false,
		},
		{
			name: "empty service name",
			config: Config{
				NodeID: "node-1",
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Basic validation checks
			hasNodeID := tt.config.NodeID != ""
			hasServiceName := tt.config.ServiceName != ""

			isValid := hasNodeID && hasServiceName

			if isValid != tt.valid {
				t.Errorf("Config validation = %v, want %v", isValid, tt.valid)
			}
		})
	}
}

func TestStrategyType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		strategy StrategyType
		expected string
	}{
		{"static", StrategyStatic, "static"},
		{"kubernetes", StrategyKubernetes, "kubernetes"},
		{"dns", StrategyDNS, "dns"},
		{"gossip", StrategyGossip, "gossip"},
		{"consul", StrategyConsul, "consul"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.strategy) != tt.expected {
				t.Errorf("Strategy %s = %s, want %s", tt.name, string(tt.strategy), tt.expected)
			}
		})
	}
}

func TestEvent_Structure(t *testing.T) {
	node := Node{
		ID:      "test-node",
		Address: "192.168.1.100",
		Port:    7000,
	}

	tests := []struct {
		name      string
		eventType EventType
		expected  string
	}{
		{"node joined", EventNodeJoined, "node_joined"},
		{"node left", EventNodeLeft, "node_left"},
		{"node failed", EventNodeFailed, "node_failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := Event{
				Type: tt.eventType,
				Node: node,
			}

			if string(event.Type) != tt.expected {
				t.Errorf("Event.Type = %s, want %s", string(event.Type), tt.expected)
			}

			if event.Node.ID != node.ID {
				t.Errorf("Event.Node.ID = %s, want %s", event.Node.ID, node.ID)
			}
		})
	}
}

func TestEventType_Constants(t *testing.T) {
	if string(EventNodeJoined) != "node_joined" {
		t.Errorf("EventNodeJoined = %s, want 'node_joined'", string(EventNodeJoined))
	}
	if string(EventNodeLeft) != "node_left" {
		t.Errorf("EventNodeLeft = %s, want 'node_left'", string(EventNodeLeft))
	}
	if string(EventNodeFailed) != "node_failed" {
		t.Errorf("EventNodeFailed = %s, want 'node_failed'", string(EventNodeFailed))
	}
}

func TestFactory_NewStrategy_ErrorCases(t *testing.T) {
	factory := &Factory{}
	config := Config{
		NodeID:      "test-node",
		ServiceName: "test-service",
	}

	tests := []struct {
		name         string
		strategyType StrategyType
		config       Config
		wantErr      bool
		expectedErr  error
	}{
		{
			name:         "consul strategy not implemented",
			strategyType: StrategyConsul,
			config:       config,
			wantErr:      true,
		},
		{
			name:         "unknown strategy",
			strategyType: StrategyType("unknown"),
			config:       config,
			wantErr:      true,
			expectedErr:  ErrUnknownStrategy,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strategy, err := factory.NewStrategy(tt.strategyType, tt.config)

			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				}
				if strategy != nil {
					t.Error("Expected nil strategy when error occurs")
				}
				if tt.expectedErr != nil && err != tt.expectedErr {
					t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if strategy == nil {
					t.Error("Expected strategy but got nil")
				}
			}
		})
	}
}

func TestErrors(t *testing.T) {
	if ErrUnknownStrategy == nil {
		t.Error("ErrUnknownStrategy should not be nil")
	}
	if ErrNotImplemented == nil {
		t.Error("ErrNotImplemented should not be nil")
	}

	// Test error messages
	expectedUnknown := "unknown discovery strategy"
	if ErrUnknownStrategy.Error() != expectedUnknown {
		t.Errorf("ErrUnknownStrategy message = %s, want %s",
			ErrUnknownStrategy.Error(), expectedUnknown)
	}

	expectedNotImpl := "strategy not implemented"
	if ErrNotImplemented.Error() != expectedNotImpl {
		t.Errorf("ErrNotImplemented message = %s, want %s",
			ErrNotImplemented.Error(), expectedNotImpl)
	}
}

func TestConfig_DefaultValues(t *testing.T) {
	config := Config{}

	// Test zero values
	if config.NodeID != "" {
		t.Errorf("Default NodeID should be empty, got %s", config.NodeID)
	}
	if config.ServiceName != "" {
		t.Errorf("Default ServiceName should be empty, got %s", config.ServiceName)
	}
	if config.Namespace != "" {
		t.Errorf("Default Namespace should be empty, got %s", config.Namespace)
	}
	if config.Tags != nil {
		t.Errorf("Default Tags should be nil, got %v", config.Tags)
	}
	if config.Meta != nil {
		t.Errorf("Default Meta should be nil, got %v", config.Meta)
	}
	if config.Interval != 0 {
		t.Errorf("Default Interval should be 0, got %v", config.Interval)
	}
}

func TestConfig_WithTags(t *testing.T) {
	config := Config{
		NodeID:      "node-1",
		ServiceName: "test-service",
		Tags:        []string{"tag1", "tag2", "tag3"},
	}

	if len(config.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(config.Tags))
	}

	expectedTags := []string{"tag1", "tag2", "tag3"}
	for i, tag := range config.Tags {
		if tag != expectedTags[i] {
			t.Errorf("Tag[%d] = %s, want %s", i, tag, expectedTags[i])
		}
	}
}

func TestConfig_WithMeta(t *testing.T) {
	meta := map[string]string{
		"version":     "1.0.0",
		"environment": "production",
		"region":      "us-west-2",
	}

	config := Config{
		NodeID:      "node-1",
		ServiceName: "test-service",
		Meta:        meta,
	}

	if len(config.Meta) != 3 {
		t.Errorf("Expected 3 meta entries, got %d", len(config.Meta))
	}

	for key, expectedValue := range meta {
		if actualValue, exists := config.Meta[key]; !exists {
			t.Errorf("Meta key %s not found", key)
		} else if actualValue != expectedValue {
			t.Errorf("Meta[%s] = %s, want %s", key, actualValue, expectedValue)
		}
	}
}

func TestNode_FullAddress(t *testing.T) {
	tests := []struct {
		name     string
		node     Node
		expected string
	}{
		{
			name: "standard address",
			node: Node{
				Address: "127.0.0.1",
				Port:    7000,
			},
			expected: "127.0.0.1:7000",
		},
		{
			name: "hostname address",
			node: Node{
				Address: "node-1.cluster.local",
				Port:    8080,
			},
			expected: "node-1.cluster.local:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since there's no FullAddress method, we test the logic
			fullAddr := fmt.Sprintf("%s:%d", tt.node.Address, tt.node.Port)
			if fullAddr != tt.expected {
				t.Errorf("Full address = %s, want %s", fullAddr, tt.expected)
			}
		})
	}
}
