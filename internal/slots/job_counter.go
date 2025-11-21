package slots

import (
	"scheduled-db/internal/store"
)

// JobCounter tracks total job count
type JobCounter struct {
	store *store.Store
	limit int64
}

// NewJobCounter creates a new JobCounter with the given store and limit
func NewJobCounter(store *store.Store, limit int64) *JobCounter {
	return &JobCounter{
		store: store,
		limit: limit,
	}
}

// CheckCapacity verifies if another job can be added
func (jc *JobCounter) CheckCapacity() error {
	current := jc.GetCount()

	if current >= jc.limit {
		return &CapacityError{
			Type:    "job_count",
			Current: current,
			Limit:   jc.limit,
		}
	}

	return nil
}

// Increment adds one to job count via Raft
func (jc *JobCounter) Increment() error {
	return jc.store.UpdateJobCount(1)
}

// Decrement subtracts one from job count via Raft
func (jc *JobCounter) Decrement() error {
	return jc.store.UpdateJobCount(-1)
}

// GetCount returns current job count
func (jc *JobCounter) GetCount() int64 {
	return jc.store.GetJobCount()
}

// GetLimit returns the job count limit
func (jc *JobCounter) GetLimit() int64 {
	return jc.limit
}
