package slots

import (
	"container/heap"
	"scheduled-db/internal/logger"
	"sync"
	"time"

	"scheduled-db/internal/store"
)

const DefaultSlotGap = 10 * time.Second

type SlotKey int64

type Slot struct {
	Key     SlotKey
	MinTime int64
	MaxTime int64
	Jobs    []*store.Job
}

type SlotQueue struct {
	mu         sync.RWMutex
	slots      map[SlotKey]*Slot
	sortedKeys *SlotHeap
	slotGap    time.Duration
}

type SlotHeap []SlotKey

func (h SlotHeap) Len() int           { return len(h) }
func (h SlotHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h SlotHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *SlotHeap) Push(x interface{}) {
	*h = append(*h, x.(SlotKey))
}

func (h *SlotHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

func NewSlotQueue(slotGap time.Duration) *SlotQueue {
	if slotGap == 0 {
		slotGap = DefaultSlotGap
	}

	sortedKeys := &SlotHeap{}
	heap.Init(sortedKeys)

	return &SlotQueue{
		slots:      make(map[SlotKey]*Slot),
		sortedKeys: sortedKeys,
		slotGap:    slotGap,
	}
}

func (sq *SlotQueue) getSlotKey(timestamp int64) SlotKey {
	slotGapSeconds := int64(sq.slotGap.Seconds())
	return SlotKey(timestamp / slotGapSeconds)
}

func (sq *SlotQueue) AddJob(job *store.Job) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	var timestamp int64
	if job.Type == store.JobUnico && job.Timestamp != nil {
		timestamp = *job.Timestamp
	} else if job.Type == store.JobRecurrente {
		// Para jobs recurrentes, usar CreatedAt como timestamp de ejecución
		timestamp = job.CreatedAt
	} else {
		logger.Debug("Cannot add job %s: invalid type or missing timestamp", job.ID)
		return
	}

	key := sq.getSlotKey(timestamp)

	slot, exists := sq.slots[key]
	if !exists {
		slotGapSeconds := int64(sq.slotGap.Seconds())
		slot = &Slot{
			Key:     key,
			MinTime: int64(key) * slotGapSeconds,
			MaxTime: (int64(key)+1)*slotGapSeconds - 1,
			Jobs:    make([]*store.Job, 0),
		}
		sq.slots[key] = slot
		heap.Push(sq.sortedKeys, key)
	}

	slot.Jobs = append(slot.Jobs, job)
	logger.Debug("Added job %s to slot %d (time range: %d-%d)", job.ID, key, slot.MinTime, slot.MaxTime)
}

func (sq *SlotQueue) RemoveJob(jobID string) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	for key, slot := range sq.slots {
		for i, job := range slot.Jobs {
			if job.ID == jobID {
				// Remove job from slice
				slot.Jobs = append(slot.Jobs[:i], slot.Jobs[i+1:]...)

				// If slot is empty, remove it
				if len(slot.Jobs) == 0 {
					delete(sq.slots, key)
					sq.removeKeyFromHeap(key)
				}

				logger.Debug("Removed job %s from slot %d", jobID, key)
				return
			}
		}
	}
}

func (sq *SlotQueue) removeKeyFromHeap(keyToRemove SlotKey) {
	// Rebuild heap without the key
	newHeap := &SlotHeap{}
	for sq.sortedKeys.Len() > 0 {
		key := heap.Pop(sq.sortedKeys).(SlotKey)
		if key != keyToRemove {
			heap.Push(newHeap, key)
		}
	}
	sq.sortedKeys = newHeap
}

func (sq *SlotQueue) GetNextSlot() *Slot {
	sq.mu.RLock()
	defer sq.mu.RUnlock()

	if sq.sortedKeys.Len() == 0 {
		return nil
	}

	key := (*sq.sortedKeys)[0]
	return sq.slots[key]
}

func (sq *SlotQueue) RemoveSlot(key SlotKey) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	delete(sq.slots, key)
	sq.removeKeyFromHeap(key)
}

func (sq *SlotQueue) LoadJobs(jobs map[string]*store.Job) {
	sq.mu.Lock()
	defer sq.mu.Unlock()

	// Clear existing slots
	sq.slots = make(map[SlotKey]*Slot)
	sq.sortedKeys = &SlotHeap{}
	heap.Init(sq.sortedKeys)

	// Add all jobs to slots
	for _, job := range jobs {
		sq.addJobUnsafe(job)
	}

	logger.Debug("Loaded %d jobs into slot queue", len(jobs))
}

func (sq *SlotQueue) addJobUnsafe(job *store.Job) {
	var timestamp int64
	if job.Type == store.JobUnico && job.Timestamp != nil {
		timestamp = *job.Timestamp
	} else if job.Type == store.JobRecurrente {
		timestamp = job.CreatedAt
	} else {
		return
	}

	key := sq.getSlotKey(timestamp)

	slot, exists := sq.slots[key]
	if !exists {
		slotGapSeconds := int64(sq.slotGap.Seconds())
		slot = &Slot{
			Key:     key,
			MinTime: int64(key) * slotGapSeconds,
			MaxTime: (int64(key)+1)*slotGapSeconds - 1,
			Jobs:    make([]*store.Job, 0),
		}
		sq.slots[key] = slot
		heap.Push(sq.sortedKeys, key)
	}

	slot.Jobs = append(slot.Jobs, job)
}

func (sq *SlotQueue) Size() int {
	sq.mu.RLock()
	defer sq.mu.RUnlock()
	return len(sq.slots)
}

func (sq *SlotQueue) JobCount() int {
	sq.mu.RLock()
	defer sq.mu.RUnlock()

	count := 0
	for _, slot := range sq.slots {
		count += len(slot.Jobs)
	}
	return count
}
