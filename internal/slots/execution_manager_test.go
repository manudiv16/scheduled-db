package slots

import (
	"context"
	"errors"
	"testing"
	"time"

	"scheduled-db/internal/store"
)

func TestExecutionManager_classifyError(t *testing.T) {
	em := &ExecutionManager{}

	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: "",
		},
		{
			name:     "timeout error",
			err:      context.DeadlineExceeded,
			expected: "timeout",
		},
		{
			name:     "cancelled error",
			err:      context.Canceled,
			expected: "cancelled",
		},
		{
			name:     "connection refused",
			err:      errors.New("connection refused"),
			expected: "connection_error",
		},
		{
			name:     "dns error",
			err:      errors.New("no such host"),
			expected: "dns_error",
		},
		{
			name:     "http error",
			err:      errors.New("webhook returned status 404"),
			expected: "http_error",
		},
		{
			name:     "unknown error",
			err:      errors.New("something went wrong"),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := em.classifyError(tt.err)
			if result != tt.expected {
				t.Errorf("classifyError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNewExecutionManager(t *testing.T) {
	// Create a minimal store for testing
	dataDir := t.TempDir()
	st, err := store.NewStore(dataDir, "127.0.0.1:7000", "127.0.0.1:7000", "test-node", []string{})
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	statusTracker := store.NewStatusTracker(st)
	nodeID := "test-node"
	timeout := 5 * time.Second
	maxAttempts := 3

	em := NewExecutionManager(statusTracker, st, nodeID, timeout, maxAttempts)

	if em == nil {
		t.Fatal("NewExecutionManager returned nil")
	}

	if em.nodeID != nodeID {
		t.Errorf("nodeID = %v, want %v", em.nodeID, nodeID)
	}

	if em.executionTimeout != timeout {
		t.Errorf("executionTimeout = %v, want %v", em.executionTimeout, timeout)
	}

	if em.maxAttempts != maxAttempts {
		t.Errorf("maxAttempts = %v, want %v", em.maxAttempts, maxAttempts)
	}
}
