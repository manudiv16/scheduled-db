package slots

import (
	"fmt"
	"scheduled-db/internal/store"
)

// MemoryTracker tracks slot memory usage
type MemoryTracker struct {
	store       *store.Store
	memoryLimit int64
}

// MemoryUsage represents current memory state
type MemoryUsage struct {
	CurrentBytes   int64   `json:"current_bytes"`
	LimitBytes     int64   `json:"limit_bytes"`
	AvailableBytes int64   `json:"available_bytes"`
	Utilization    float64 `json:"utilization_percent"`
}

// NewMemoryTracker creates a new MemoryTracker with the given store and limit
func NewMemoryTracker(store *store.Store, memoryLimit int64) *MemoryTracker {
	return &MemoryTracker{
		store:       store,
		memoryLimit: memoryLimit,
	}
}

// CheckCapacity verifies if size bytes can be added
func (mt *MemoryTracker) CheckCapacity(size int64) error {
	usage := mt.GetUsage()

	if usage.CurrentBytes+size > usage.LimitBytes {
		return &CapacityError{
			Type:      "memory",
			Current:   usage.CurrentBytes,
			Limit:     usage.LimitBytes,
			Requested: size,
		}
	}

	return nil
}

// IncrementUsage adds size to current usage via Raft
func (mt *MemoryTracker) IncrementUsage(size int64) error {
	return mt.store.UpdateMemoryUsage(size)
}

// DecrementUsage subtracts size from current usage via Raft
func (mt *MemoryTracker) DecrementUsage(size int64) error {
	return mt.store.UpdateMemoryUsage(-size)
}

// GetUsage returns current memory usage
func (mt *MemoryTracker) GetUsage() *MemoryUsage {
	current := mt.store.GetMemoryUsage()
	available := mt.memoryLimit - current

	// Ensure available is not negative
	if available < 0 {
		available = 0
	}

	// Calculate utilization percentage
	utilization := 0.0
	if mt.memoryLimit > 0 {
		utilization = float64(current) / float64(mt.memoryLimit) * 100.0
	}

	return &MemoryUsage{
		CurrentBytes:   current,
		LimitBytes:     mt.memoryLimit,
		AvailableBytes: available,
		Utilization:    utilization,
	}
}

// CapacityError represents a capacity limit error
type CapacityError struct {
	Type      string `json:"type"` // "memory" or "job_count"
	Current   int64  `json:"current"`
	Limit     int64  `json:"limit"`
	Requested int64  `json:"requested,omitempty"`
}

// Error implements the error interface
// Validates: Requirements 4.3, 4.4, 5.3, 5.4, 10.1, 10.2, 10.3, 10.4
func (e *CapacityError) Error() string {
	switch e.Type {
	case "memory":
		if e.Requested > 0 {
			return fmt.Sprintf("insufficient memory: current=%d bytes, limit=%d bytes, requested=%d bytes",
				e.Current, e.Limit, e.Requested)
		}
		return fmt.Sprintf("insufficient memory: current=%d bytes, limit=%d bytes",
			e.Current, e.Limit)
	case "job_count":
		return fmt.Sprintf("maximum jobs reached: current=%d, limit=%d",
			e.Current, e.Limit)
	default:
		return "capacity limit exceeded"
	}
}

// HTTPStatus returns the HTTP status code for this error
func (e *CapacityError) HTTPStatus() int {
	return 507 // Insufficient Storage
}
