package slots

import (
	"context"
	"scheduled-db/internal/metrics"
	"scheduled-db/internal/store"
)

// LimitManager enforces queue size limits
type LimitManager struct {
	memoryTracker  *MemoryTracker
	jobCounter     *JobCounter
	sizeCalculator *SizeCalculator
}

// NewLimitManager creates a new LimitManager with all components
func NewLimitManager(memoryTracker *MemoryTracker, jobCounter *JobCounter, sizeCalculator *SizeCalculator) *LimitManager {
	return &LimitManager{
		memoryTracker:  memoryTracker,
		jobCounter:     jobCounter,
		sizeCalculator: sizeCalculator,
	}
}

// CheckCapacity verifies if a job can be added
// Validates: Requirements 4.1, 5.1
func (lm *LimitManager) CheckCapacity(job *store.Job) error {
	// Calculate job size
	size := lm.sizeCalculator.CalculateSize(job)

	// Check memory limit
	if err := lm.memoryTracker.CheckCapacity(size); err != nil {
		return err
	}

	// Check job count limit
	if err := lm.jobCounter.CheckCapacity(); err != nil {
		return err
	}

	return nil
}

// RecordJobAdded updates tracking after job is added
// Validates: Requirements 4.1, 5.1
func (lm *LimitManager) RecordJobAdded(job *store.Job) error {
	size := lm.sizeCalculator.CalculateSize(job)

	if err := lm.memoryTracker.IncrementUsage(size); err != nil {
		return err
	}

	if err := lm.jobCounter.Increment(); err != nil {
		// Rollback memory usage if job count increment fails
		lm.memoryTracker.DecrementUsage(size)
		return err
	}

	// Update metrics
	if m := metrics.GetGlobalMetrics(); m != nil {
		ctx := context.Background()
		m.UpdateQueueMemoryUsage(ctx, size)
		m.UpdateQueueJobCount(ctx, 1)
	}

	return nil
}

// RecordJobRemoved updates tracking after job is removed
// Validates: Requirements 4.1, 5.1
func (lm *LimitManager) RecordJobRemoved(job *store.Job) error {
	size := lm.sizeCalculator.CalculateSize(job)

	if err := lm.memoryTracker.DecrementUsage(size); err != nil {
		return err
	}

	if err := lm.jobCounter.Decrement(); err != nil {
		return err
	}

	// Update metrics
	if m := metrics.GetGlobalMetrics(); m != nil {
		ctx := context.Background()
		m.UpdateQueueMemoryUsage(ctx, -size)
		m.UpdateQueueJobCount(ctx, -1)
	}

	return nil
}

// GetMemoryUsage returns current memory usage details
func (lm *LimitManager) GetMemoryUsage() *MemoryUsage {
	return lm.memoryTracker.GetUsage()
}

// GetJobCount returns current job count
func (lm *LimitManager) GetJobCount() int64 {
	return lm.jobCounter.GetCount()
}

// GetJobLimit returns the job count limit
func (lm *LimitManager) GetJobLimit() int64 {
	return lm.jobCounter.GetLimit()
}
