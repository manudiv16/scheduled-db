package slots

import (
	"scheduled-db/internal/logger"
	"sync"
	"time"
	"scheduled-db/internal/store"
)

// PersistentSlotQueue usa el store para persistir slots
type PersistentSlotQueue struct {
	mu      sync.RWMutex
	store   *store.Store
	slotGap time.Duration
}

func NewPersistentSlotQueue(slotGap time.Duration, store *store.Store) *PersistentSlotQueue {
	if slotGap == 0 {
		slotGap = DefaultSlotGap
	}

	return &PersistentSlotQueue{
		store:   store,
		slotGap: slotGap,
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
}

func (psq *PersistentSlotQueue) RemoveJob(jobID string) {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	// Find slot containing this job
	slots := psq.store.GetAllSlots()
	for key, slotData := range slots {
		for i, id := range slotData.JobIDs {
			if id == jobID {
				// Remove job from slot
				slotData.JobIDs = append(slotData.JobIDs[:i], slotData.JobIDs[i+1:]...)
				
				if len(slotData.JobIDs) == 0 {
					// Delete empty slot
					if psq.store.IsLeader() {
						psq.store.DeleteSlot(key)
					}
				} else {
					// Update slot
					if psq.store.IsLeader() {
						psq.store.CreateSlot(slotData)
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

	slots := psq.store.GetAllSlots()
	if len(slots) == 0 {
		return nil
	}

	// Find earliest slot
	var earliestKey int64 = -1
	for key := range slots {
		if earliestKey == -1 || key < earliestKey {
			earliestKey = key
		}
	}

	slotData := slots[earliestKey]
	
	// Convert to Slot with actual Job objects
	jobs := make([]*store.Job, 0, len(slotData.JobIDs))
	allJobs := psq.store.GetAllJobs()
	
	for _, jobID := range slotData.JobIDs {
		if job, exists := allJobs[jobID]; exists {
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

func (psq *PersistentSlotQueue) RemoveSlot(key SlotKey) {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	if psq.store.IsLeader() {
		psq.store.DeleteSlot(int64(key))
	}
}

// LoadJobs ya no es necesario - los slots se cargan automáticamente del store
func (psq *PersistentSlotQueue) LoadJobs(jobs map[string]*store.Job) {
	// No-op - slots are already persisted
	logger.Debug("PersistentSlotQueue: slots loaded from persistent store")
}
