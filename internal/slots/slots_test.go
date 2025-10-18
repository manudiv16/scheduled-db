package slots

import (
	"container/heap"
	"fmt"
	"testing"
	"time"

	"scheduled-db/internal/store"
)

func createTestJob(id string, jobType store.JobType, timestamp *int64) *store.Job {
	job := &store.Job{
		ID:        id,
		Type:      jobType,
		CreatedAt: time.Now().Unix(),
	}

	if jobType == store.JobUnico && timestamp != nil {
		job.Timestamp = timestamp
	} else if jobType == store.JobRecurrente {
		job.CronExpr = "0 */6 * * *"
		if timestamp != nil {
			job.CreatedAt = *timestamp
		}
	}

	return job
}

func TestNewSlotQueue(t *testing.T) {
	tests := []struct {
		name            string
		slotGap         time.Duration
		expectedInitial bool
	}{
		{
			name:            "default slot gap",
			slotGap:         0,
			expectedInitial: true,
		},
		{
			name:            "custom slot gap",
			slotGap:         30 * time.Second,
			expectedInitial: true,
		},
		{
			name:            "minute slot gap",
			slotGap:         60 * time.Second,
			expectedInitial: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sq := NewSlotQueue(tt.slotGap)

			if sq == nil {
				t.Error("NewSlotQueue() returned nil")
				return
			}

			if sq.Size() != 0 {
				t.Errorf("Size() = %d, want 0", sq.Size())
			}

			if sq.JobCount() != 0 {
				t.Errorf("JobCount() = %d, want 0", sq.JobCount())
			}
		})
	}
}

func TestSlotQueue_AddJob(t *testing.T) {
	sq := NewSlotQueue(10 * time.Second)

	tests := []struct {
		name      string
		job       *store.Job
		shouldAdd bool
	}{
		{
			name: "add unique job",
			job: createTestJob("job-1", store.JobUnico, func() *int64 {
				ts := int64(15)
				return &ts
			}()),
			shouldAdd: true,
		},
		{
			name: "add recurring job",
			job: createTestJob("job-2", store.JobRecurrente, func() *int64 {
				ts := int64(25)
				return &ts
			}()),
			shouldAdd: true,
		},
		{
			name: "add unique job without timestamp",
			job: &store.Job{
				ID:        "job-3",
				Type:      store.JobUnico,
				CreatedAt: time.Now().Unix(),
			},
			shouldAdd: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialSize := sq.Size()
			initialJobCount := sq.JobCount()

			sq.AddJob(tt.job)

			if tt.shouldAdd {
				if sq.Size() <= initialSize {
					t.Errorf("Size() = %d, expected increase from %d", sq.Size(), initialSize)
				}

				if sq.JobCount() <= initialJobCount {
					t.Errorf("JobCount() = %d, expected increase from %d", sq.JobCount(), initialJobCount)
				}
			} else {
				if sq.Size() != initialSize {
					t.Errorf("Size() = %d, expected no change from %d", sq.Size(), initialSize)
				}

				if sq.JobCount() != initialJobCount {
					t.Errorf("JobCount() = %d, expected no change from %d", sq.JobCount(), initialJobCount)
				}
			}
		})
	}
}

func TestSlotQueue_RemoveJob(t *testing.T) {
	sq := NewSlotQueue(10 * time.Second)

	// Add some test jobs
	job1 := createTestJob("job-1", store.JobUnico, func() *int64 {
		ts := int64(15)
		return &ts
	}())
	job2 := createTestJob("job-2", store.JobUnico, func() *int64 {
		ts := int64(15)
		return &ts
	}())
	job3 := createTestJob("job-3", store.JobUnico, func() *int64 {
		ts := int64(25)
		return &ts
	}())

	sq.AddJob(job1)
	sq.AddJob(job2)
	sq.AddJob(job3)

	tests := []struct {
		name             string
		jobID            string
		expectedSize     int
		expectedJobCount int
	}{
		{
			name:             "remove job from slot with multiple jobs",
			jobID:            "job-1",
			expectedSize:     2, // Still 2 slots
			expectedJobCount: 2, // 2 jobs remaining
		},
		{
			name:             "remove last job from slot",
			jobID:            "job-2",
			expectedSize:     1, // 1 slot remaining
			expectedJobCount: 1, // 1 job remaining
		},
		{
			name:             "remove non-existing job",
			jobID:            "non-existing",
			expectedSize:     1, // No change
			expectedJobCount: 1, // No change
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sq.RemoveJob(tt.jobID)

			if sq.Size() != tt.expectedSize {
				t.Errorf("Size() = %d, want %d", sq.Size(), tt.expectedSize)
			}

			if sq.JobCount() != tt.expectedJobCount {
				t.Errorf("JobCount() = %d, want %d", sq.JobCount(), tt.expectedJobCount)
			}
		})
	}
}

func TestSlotQueue_GetNextSlot(t *testing.T) {
	sq := NewSlotQueue(10 * time.Second)

	// Test empty queue
	nextSlot := sq.GetNextSlot()
	if nextSlot != nil {
		t.Error("GetNextSlot() should return nil for empty queue")
	}

	// Add jobs with different timestamps
	job1 := createTestJob("job-1", store.JobUnico, func() *int64 {
		ts := int64(25) // Slot 2
		return &ts
	}())
	job2 := createTestJob("job-2", store.JobUnico, func() *int64 {
		ts := int64(5) // Slot 0
		return &ts
	}())
	job3 := createTestJob("job-3", store.JobUnico, func() *int64 {
		ts := int64(15) // Slot 1
		return &ts
	}())

	sq.AddJob(job1)
	sq.AddJob(job2)
	sq.AddJob(job3)

	// Should return slot with smallest key (0)
	nextSlot = sq.GetNextSlot()
	if nextSlot == nil {
		t.Fatal("GetNextSlot() returned nil")
	}

	if nextSlot.Key != SlotKey(0) {
		t.Errorf("GetNextSlot() returned slot %d, want slot 0", nextSlot.Key)
	}

	if len(nextSlot.Jobs) != 1 {
		t.Errorf("Next slot has %d jobs, want 1", len(nextSlot.Jobs))
	}

	if nextSlot.Jobs[0].ID != "job-2" {
		t.Errorf("Next slot job ID = %s, want 'job-2'", nextSlot.Jobs[0].ID)
	}

	// Verify slot boundaries
	expectedMinTime := int64(0)
	expectedMaxTime := int64(9)
	if nextSlot.MinTime != expectedMinTime {
		t.Errorf("MinTime = %d, want %d", nextSlot.MinTime, expectedMinTime)
	}
	if nextSlot.MaxTime != expectedMaxTime {
		t.Errorf("MaxTime = %d, want %d", nextSlot.MaxTime, expectedMaxTime)
	}
}

func TestSlotQueue_RemoveSlot(t *testing.T) {
	sq := NewSlotQueue(10 * time.Second)

	// Add jobs to create slots
	job1 := createTestJob("job-1", store.JobUnico, func() *int64 {
		ts := int64(5)
		return &ts
	}())
	job2 := createTestJob("job-2", store.JobUnico, func() *int64 {
		ts := int64(15)
		return &ts
	}())

	sq.AddJob(job1)
	sq.AddJob(job2)

	initialSize := sq.Size()
	initialJobCount := sq.JobCount()

	// Remove slot 0
	sq.RemoveSlot(SlotKey(0))

	if sq.Size() != initialSize-1 {
		t.Errorf("Size() = %d, want %d", sq.Size(), initialSize-1)
	}

	if sq.JobCount() != initialJobCount-1 {
		t.Errorf("JobCount() = %d, want %d", sq.JobCount(), initialJobCount-1)
	}

	// Next slot should now be slot 1
	nextSlot := sq.GetNextSlot()
	if nextSlot == nil {
		t.Fatal("GetNextSlot() returned nil")
	}

	if nextSlot.Key != SlotKey(1) {
		t.Errorf("GetNextSlot() returned slot %d, want slot 1", nextSlot.Key)
	}
}

func TestSlotQueue_LoadJobs(t *testing.T) {
	sq := NewSlotQueue(10 * time.Second)

	// Add some initial jobs
	initialJob := createTestJob("initial-job", store.JobUnico, func() *int64 {
		ts := int64(5)
		return &ts
	}())
	sq.AddJob(initialJob)

	if sq.Size() == 0 {
		t.Error("Initial job was not added")
	}

	// Create jobs map to load
	jobs := make(map[string]*store.Job)
	jobs["job-1"] = createTestJob("job-1", store.JobUnico, func() *int64 {
		ts := int64(15)
		return &ts
	}())
	jobs["job-2"] = createTestJob("job-2", store.JobRecurrente, func() *int64 {
		ts := int64(25)
		return &ts
	}())
	jobs["job-3"] = createTestJob("job-3", store.JobUnico, func() *int64 {
		ts := int64(35)
		return &ts
	}())

	// Load jobs
	sq.LoadJobs(jobs)

	// Verify old jobs were cleared and new jobs loaded
	if sq.JobCount() != 3 {
		t.Errorf("JobCount() = %d, want 3", sq.JobCount())
	}

	// Verify the next slot contains one of our new jobs
	nextSlot := sq.GetNextSlot()
	if nextSlot == nil {
		t.Error("Expected at least one slot after loading jobs")
	}
}

func TestSlotHeap(t *testing.T) {
	h := &SlotHeap{}
	heap.Init(h)

	// Test heap operations
	keys := []SlotKey{SlotKey(3), SlotKey(1), SlotKey(4), SlotKey(1), SlotKey(5)}

	// Push all keys
	for _, key := range keys {
		heap.Push(h, key)
	}

	if h.Len() != len(keys) {
		t.Errorf("Heap length = %d, want %d", h.Len(), len(keys))
	}

	// Pop keys and verify they come out in sorted order
	var popped []SlotKey
	for h.Len() > 0 {
		key := heap.Pop(h).(SlotKey)
		popped = append(popped, key)
	}

	// Verify sorted order (allowing duplicates)
	expected := []SlotKey{SlotKey(1), SlotKey(1), SlotKey(3), SlotKey(4), SlotKey(5)}
	if len(popped) != len(expected) {
		t.Errorf("Popped %d keys, want %d", len(popped), len(expected))
	}

	for i, key := range popped {
		if key != expected[i] {
			t.Errorf("Popped key[%d] = %d, want %d", i, key, expected[i])
		}
	}
}

func TestSlotQueue_ConcurrentAccess(t *testing.T) {
	sq := NewSlotQueue(10 * time.Second)

	// Test concurrent adds and reads
	done := make(chan bool)

	// Goroutine 1: Add jobs
	go func() {
		for i := 0; i < 100; i++ {
			job := createTestJob(fmt.Sprintf("job-%d", i), store.JobUnico, func(i int) *int64 {
				ts := int64(i * 10)
				return &ts
			}(i))
			sq.AddJob(job)
		}
		done <- true
	}()

	// Goroutine 2: Read operations
	go func() {
		for i := 0; i < 100; i++ {
			sq.Size()
			sq.JobCount()
			sq.GetNextSlot()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify final state
	if sq.JobCount() != 100 {
		t.Errorf("JobCount() = %d, want 100", sq.JobCount())
	}
}

func TestSlotQueue_SlotBoundaries(t *testing.T) {
	sq := NewSlotQueue(60 * time.Second) // 1-minute slots

	job := createTestJob("boundary-job", store.JobUnico, func() *int64 {
		ts := int64(119) // 1 second before 2-minute mark
		return &ts
	}())

	sq.AddJob(job)

	nextSlot := sq.GetNextSlot()
	if nextSlot == nil {
		t.Fatal("GetNextSlot() returned nil")
	}

	expectedKey := SlotKey(1) // Should be in slot 1 (60-119 seconds)
	if nextSlot.Key != expectedKey {
		t.Errorf("Slot key = %d, want %d", nextSlot.Key, expectedKey)
	}

	expectedMinTime := int64(60)
	expectedMaxTime := int64(119)
	if nextSlot.MinTime != expectedMinTime {
		t.Errorf("MinTime = %d, want %d", nextSlot.MinTime, expectedMinTime)
	}
	if nextSlot.MaxTime != expectedMaxTime {
		t.Errorf("MaxTime = %d, want %d", nextSlot.MaxTime, expectedMaxTime)
	}
}

func TestSlotQueue_EmptyOperations(t *testing.T) {
	sq := NewSlotQueue(10 * time.Second)

	// Test operations on empty queue
	if sq.Size() != 0 {
		t.Errorf("Size() = %d, want 0", sq.Size())
	}

	if sq.JobCount() != 0 {
		t.Errorf("JobCount() = %d, want 0", sq.JobCount())
	}

	if sq.GetNextSlot() != nil {
		t.Error("GetNextSlot() should return nil for empty queue")
	}

	// Remove job from empty queue (should not panic)
	sq.RemoveJob("non-existing-job")

	// Remove slot from empty queue (should not panic)
	sq.RemoveSlot(SlotKey(0))

	// Load empty jobs map
	emptyJobs := make(map[string]*store.Job)
	sq.LoadJobs(emptyJobs)

	if sq.Size() != 0 {
		t.Errorf("Size() after loading empty jobs = %d, want 0", sq.Size())
	}
}
