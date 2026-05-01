package slots

import (
	"math"
	"scheduled-db/internal/logger"
	"scheduled-db/internal/store"
	"sync"
	"time"
)

// PersistentSlotQueue usa el store para persistir slots.
// It maintains a local reverse index (jobID -> slotKey) to avoid O(n*m)
// scans during RemoveJob, and caches the minimum slot key to avoid
// O(n) full scans during GetNextSlot.
type PersistentSlotQueue struct {
	mu           sync.RWMutex
	store        *store.Store
	slotGap      time.Duration
	jobToSlot    map[string]int64 // jobID -> slotKey reverse index
	cachedMinKey int64            // cached minimum slot key, math.MaxInt64 when empty
}

func NewPersistentSlotQueue(slotGap time.Duration, store *store.Store) *PersistentSlotQueue {
	if slotGap == 0 {
		slotGap = DefaultSlotGap
	}

	return &PersistentSlotQueue{
		store:        store,
		slotGap:      slotGap,
		jobToSlot:    make(map[string]int64),
		cachedMinKey: math.MaxInt64,
	}
}

func (psq *PersistentSlotQueue) getSlotKey(timestamp int64) int64 {
	slotGapSeconds := int64(psq.slotGap.Seconds())
	return timestamp / slotGapSeconds
}

func (psq *PersistentSlotQueue) AddJob(job *store.Job) {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	var timestamp int64
	if job.Type == store.JobUnico && job.Timestamp != nil {
		timestamp = *job.Timestamp
	} else if job.Type == store.JobRecurrente {
		timestamp = job.CreatedAt
	} else {
		return
	}

	key := psq.getSlotKey(timestamp)

	// Get existing slot or create new one
	slotData, exists := psq.store.GetSlot(key)
	if !exists {
		slotGapSeconds := int64(psq.slotGap.Seconds())
		slotData = &store.SlotData{
			Key:     key,
			MinTime: key * slotGapSeconds,
			MaxTime: (key+1)*slotGapSeconds - 1,
			JobIDs:  []string{job.ID},
		}
	} else {
		// Add job to existing slot
		slotData.JobIDs = append(slotData.JobIDs, job.ID)
	}

	// Persist slot
	if psq.store.IsLeader() {
		if err := psq.store.CreateSlot(slotData); err != nil {
			logger.Debug("Failed to persist slot %d: %v", key, err)
		}
	}

	// Update reverse index and cached min key
	psq.jobToSlot[job.ID] = key
	if key < psq.cachedMinKey {
		psq.cachedMinKey = key
	}
}

func (psq *PersistentSlotQueue) RemoveJob(jobID string) {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	// Use reverse index to find the slot directly - O(1) instead of O(n*m)
	key, exists := psq.jobToSlot[jobID]
	if !exists {
		// Fallback: scan the store (handles jobs added before index was populated)
		psq.removeJobFallback(jobID)
		return
	}
	delete(psq.jobToSlot, jobID)

	slotData, slotExists := psq.store.GetSlot(key)
	if !slotExists {
		return
	}

	// Remove job from slot
	for i, id := range slotData.JobIDs {
		if id == jobID {
			slotData.JobIDs = append(slotData.JobIDs[:i], slotData.JobIDs[i+1:]...)
			break
		}
	}

	if len(slotData.JobIDs) == 0 {
		// Delete empty slot
		if psq.store.IsLeader() {
			if err := psq.store.DeleteSlot(key); err != nil {
				logger.Debug("Failed to delete slot %d: %v", key, err)
			}
		}
		// Invalidate min key cache if we removed the minimum
		if key == psq.cachedMinKey {
			psq.recomputeMinKey()
		}
	} else {
		// Update slot
		if psq.store.IsLeader() {
			if err := psq.store.CreateSlot(slotData); err != nil {
				logger.Debug("Failed to update slot %d: %v", key, err)
			}
		}
	}
}

// removeJobFallback scans all slots when the reverse index misses.
func (psq *PersistentSlotQueue) removeJobFallback(jobID string) {
	slots := psq.store.GetAllSlots()
	for key, slotData := range slots {
		for i, id := range slotData.JobIDs {
			if id == jobID {
				slotData.JobIDs = append(slotData.JobIDs[:i], slotData.JobIDs[i+1:]...)

				if len(slotData.JobIDs) == 0 {
					if psq.store.IsLeader() {
						if err := psq.store.DeleteSlot(key); err != nil {
							logger.Debug("Failed to delete slot %d: %v", key, err)
						}
					}
					if key == psq.cachedMinKey {
						psq.recomputeMinKey()
					}
				} else {
					if psq.store.IsLeader() {
						if err := psq.store.CreateSlot(slotData); err != nil {
							logger.Debug("Failed to update slot %d: %v", key, err)
						}
					}
				}
				return
			}
		}
	}
}

func (psq *PersistentSlotQueue) GetNextSlot() *Slot {
	psq.mu.RLock()
	defer psq.mu.RUnlock()

	// Fast path: use cached min key to avoid scanning all slots
	if psq.cachedMinKey == math.MaxInt64 {
		// Try refreshing from store in case slots were added externally
		psq.mu.RUnlock()
		psq.mu.Lock()
		psq.recomputeMinKey()
		psq.mu.Unlock()
		psq.mu.RLock()
		if psq.cachedMinKey == math.MaxInt64 {
			return nil
		}
	}

	slotData, exists := psq.store.GetSlot(psq.cachedMinKey)
	if !exists {
		// Cache is stale, refresh under write lock
		psq.mu.RUnlock()
		psq.mu.Lock()
		psq.recomputeMinKey()
		psq.mu.Unlock()
		psq.mu.RLock()
		if psq.cachedMinKey == math.MaxInt64 {
			return nil
		}
		slotData, exists = psq.store.GetSlot(psq.cachedMinKey)
		if !exists {
			return nil
		}
	}

	// Convert to Slot with actual Job objects
	// Only fetch the jobs we need instead of GetAllJobs()
	jobs := make([]*store.Job, 0, len(slotData.JobIDs))
	for _, jobID := range slotData.JobIDs {
		if job, jobExists := psq.store.GetJob(jobID); jobExists {
			jobs = append(jobs, job)
		}
	}

	return &Slot{
		Key:     SlotKey(slotData.Key),
		MinTime: slotData.MinTime,
		MaxTime: slotData.MaxTime,
		Jobs:    jobs,
	}
}

// recomputeMinKey scans current slots to find the new minimum. Must be called under write lock.
func (psq *PersistentSlotQueue) recomputeMinKey() {
	slots := psq.store.GetAllSlots()
	psq.cachedMinKey = math.MaxInt64
	for key := range slots {
		if key < psq.cachedMinKey {
			psq.cachedMinKey = key
		}
	}
}

func (psq *PersistentSlotQueue) RemoveSlot(key SlotKey) {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	// Clean reverse index entries for this slot
	slotData, exists := psq.store.GetSlot(int64(key))
	if exists {
		for _, jobID := range slotData.JobIDs {
			delete(psq.jobToSlot, jobID)
		}
	}

	if psq.store.IsLeader() {
		if err := psq.store.DeleteSlot(int64(key)); err != nil {
			logger.Debug("Failed to delete slot %d: %v", key, err)
		}
	}

	if int64(key) == psq.cachedMinKey {
		psq.recomputeMinKey()
	}
}

// LoadJobs ya no es necesario - los slots se cargan automáticamente del store
func (psq *PersistentSlotQueue) LoadJobs(jobs map[string]*store.Job) {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	// Rebuild reverse index from store
	psq.jobToSlot = make(map[string]int64)
	slots := psq.store.GetAllSlots()
	for key, slotData := range slots {
		for _, jobID := range slotData.JobIDs {
			psq.jobToSlot[jobID] = key
		}
	}
	psq.recomputeMinKey()

	logger.Debug("PersistentSlotQueue: slots loaded from persistent store, indexed %d jobs", len(psq.jobToSlot))
}
