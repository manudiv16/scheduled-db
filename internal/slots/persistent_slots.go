//go:build !wasm

package slots

import (
	"scheduled-db/internal/logger"
	"scheduled-db/internal/store"
	"sync"
	"time"
)

type PersistentSlotQueue struct {
	mu          sync.RWMutex
	store       *store.Store
	slotGap     time.Duration
	jobToSlot   map[string]int64
	wheel       *HierarchicalTimingWheel
	lastAdvance int64
}

func NewPersistentSlotQueue(slotGap time.Duration, store *store.Store) *PersistentSlotQueue {
	if slotGap == 0 {
		slotGap = DefaultSlotGap
	}

	configs := DefaultWheelConfigs(slotGap)
	wheel := NewHierarchicalTimingWheel(slotGap, configs)

	return &PersistentSlotQueue{
		store:     store,
		slotGap:   slotGap,
		jobToSlot: make(map[string]int64),
		wheel:     wheel,
	}
}

func NewPersistentSlotQueueWithConfig(slotGap time.Duration, store *store.Store, wheelConfigs []WheelLevelConfig) *PersistentSlotQueue {
	if slotGap == 0 {
		slotGap = DefaultSlotGap
	}

	wheel := NewHierarchicalTimingWheel(slotGap, wheelConfigs)

	return &PersistentSlotQueue{
		store:     store,
		slotGap:   slotGap,
		jobToSlot: make(map[string]int64),
		wheel:     wheel,
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
		logger.Debug("Cannot add job %s: invalid type or missing timestamp", job.ID)
		return
	}

	key := psq.getSlotKey(timestamp)

	if !psq.wheel.initialized {
		psq.wheel.Init(time.Now().Unix())
	}

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
		slotData.JobIDs = append(slotData.JobIDs, job.ID)
	}

	if psq.store.IsLeader() {
		if err := psq.store.CreateSlot(slotData); err != nil {
			logger.Debug("Failed to persist slot %d: %v", key, err)
		}
	}

	psq.jobToSlot[job.ID] = key
	psq.wheel.Add(key)
}

func (psq *PersistentSlotQueue) RemoveJob(jobID string) {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	key, exists := psq.jobToSlot[jobID]
	if !exists {
		psq.removeJobFallback(jobID)
		return
	}
	delete(psq.jobToSlot, jobID)

	slotData, slotExists := psq.store.GetSlot(key)
	if !slotExists {
		return
	}

	for i, id := range slotData.JobIDs {
		if id == jobID {
			slotData.JobIDs = append(slotData.JobIDs[:i], slotData.JobIDs[i+1:]...)
			break
		}
	}

	if len(slotData.JobIDs) == 0 {
		if psq.store.IsLeader() {
			if err := psq.store.DeleteSlot(key); err != nil {
				logger.Debug("Failed to delete slot %d: %v", key, err)
			}
		}
		psq.wheel.Remove(key)
	} else {
		if psq.store.IsLeader() {
			if err := psq.store.CreateSlot(slotData); err != nil {
				logger.Debug("Failed to update slot %d: %v", key, err)
			}
		}
	}
}

func (psq *PersistentSlotQueue) removeJobFallback(jobID string) {
	if psq.store.IsColdSpillingEnabled() {
		slots := psq.store.GetAllSlots()
		coldKeys := psq.store.GetColdSlotKeys()
		for _, key := range coldKeys {
			slotData, err := psq.store.GetColdSlotData(key)
			if err != nil || slotData == nil {
				continue
			}
			for i, id := range slotData.JobIDs {
				if id == jobID {
					slotData.JobIDs = append(slotData.JobIDs[:i], slotData.JobIDs[i+1:]...)
					if len(slotData.JobIDs) == 0 {
						if psq.store.IsLeader() {
							psq.store.DeleteSlot(key)
						}
						psq.wheel.Remove(key)
					} else {
						if psq.store.IsLeader() {
							psq.store.CreateSlot(slotData)
						}
					}
					psq.jobToSlot[jobID] = key
					delete(psq.jobToSlot, jobID)
					return
				}
			}
		}
		for key, slotData := range slots {
			for i, id := range slotData.JobIDs {
				if id == jobID {
					slotData.JobIDs = append(slotData.JobIDs[:i], slotData.JobIDs[i+1:]...)
					if len(slotData.JobIDs) == 0 {
						if psq.store.IsLeader() {
							psq.store.DeleteSlot(key)
						}
						psq.wheel.Remove(key)
					} else {
						if psq.store.IsLeader() {
							psq.store.CreateSlot(slotData)
						}
					}
					psq.jobToSlot[jobID] = key
					delete(psq.jobToSlot, jobID)
					return
				}
			}
		}
		return
	}

	slots := psq.store.GetAllSlots()
	for key, slotData := range slots {
		for i, id := range slotData.JobIDs {
			if id == jobID {
				slotData.JobIDs = append(slotData.JobIDs[:i], slotData.JobIDs[i+1:]...)
				if len(slotData.JobIDs) == 0 {
					if psq.store.IsLeader() {
						psq.store.DeleteSlot(key)
					}
					psq.wheel.Remove(key)
				} else {
					if psq.store.IsLeader() {
						psq.store.CreateSlot(slotData)
					}
				}
				psq.jobToSlot[jobID] = key
				delete(psq.jobToSlot, jobID)
				return
			}
		}
	}
}

func (psq *PersistentSlotQueue) GetNextSlot() *Slot {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	if !psq.wheel.initialized {
		psq.wheel.Init(time.Now().Unix())
	}

	now := time.Now().Unix()
	slotGapSeconds := int64(psq.slotGap.Seconds())

	readyKeys := psq.wheel.Advance(now)
	for _, key := range readyKeys {
		slotData, exists := psq.store.GetSlot(key)
		if !exists {
			psq.wheel.Remove(key)
			continue
		}
		if len(slotData.JobIDs) == 0 {
			psq.wheel.Remove(key)
			if psq.store.IsLeader() {
				psq.store.DeleteSlot(key)
			}
			continue
		}
	}

	minTime := psq.wheel.PeekMin()
	if minTime == -1 {
		return nil
	}

	minKey := minTime / slotGapSeconds

	slotData, exists := psq.store.GetSlot(minKey)
	if !exists {
		return nil
	}

	jobs := make([]*store.Job, 0, len(slotData.JobIDs))
	for _, jobID := range slotData.JobIDs {
		if job, jobExists := psq.store.GetJob(jobID); jobExists {
			jobs = append(jobs, job)
		}
	}

	if len(jobs) == 0 {
		return nil
	}

	return &Slot{
		Key:     SlotKey(minKey),
		MinTime: slotData.MinTime,
		MaxTime: slotData.MaxTime,
		Jobs:    jobs,
	}
}

func (psq *PersistentSlotQueue) RemoveSlot(key SlotKey) {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	slotData, exists := psq.store.GetSlot(int64(key))
	if exists {
		for _, jobID := range slotData.JobIDs {
			delete(psq.jobToSlot, jobID)
		}
	}

	psq.wheel.Remove(int64(key))

	if psq.store.IsLeader() {
		if err := psq.store.DeleteSlot(int64(key)); err != nil {
			logger.Debug("Failed to delete slot %d: %v", key, err)
		}
	}
}

func (psq *PersistentSlotQueue) LoadJobs(jobs map[string]*store.Job) {
	psq.mu.Lock()
	defer psq.mu.Unlock()

	psq.jobToSlot = make(map[string]int64)
	psq.wheel.Reset()
	psq.wheel.Init(time.Now().Unix())

	slots := psq.store.GetAllSlots()
	for key, slotData := range slots {
		for _, jobID := range slotData.JobIDs {
			psq.jobToSlot[jobID] = key
		}
		psq.wheel.Add(key)
	}

	if psq.store.IsColdSpillingEnabled() {
		coldKeys := psq.store.GetColdSlotKeys()
		for _, key := range coldKeys {
			slotData, err := psq.store.GetColdSlotData(key)
			if err != nil || slotData == nil {
				continue
			}
			for _, jobID := range slotData.JobIDs {
				psq.jobToSlot[jobID] = key
			}
			psq.wheel.Add(key)
		}
	}

	psq.lastAdvance = time.Now().Unix()
	logger.Debug("PersistentSlotQueue: loaded %d jobs into HTW, wheel size=%d", len(psq.jobToSlot), psq.wheel.Size())
}

func (psq *PersistentSlotQueue) Size() int {
	psq.mu.RLock()
	defer psq.mu.RUnlock()
	return psq.wheel.Size()
}

func (psq *PersistentSlotQueue) JobCount() int {
	psq.mu.RLock()
	defer psq.mu.RUnlock()
	return len(psq.jobToSlot)
}

func (psq *PersistentSlotQueue) GetWheel() *HierarchicalTimingWheel {
	return psq.wheel
}
