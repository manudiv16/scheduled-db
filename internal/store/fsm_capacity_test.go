package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/raft"
	"pgregory.net/rapid"
)

func TestFSM_MemoryUsageTracking(t *testing.T) {
	fsm := NewFSM()

	// Initial memory usage should be 0
	if usage := fsm.GetMemoryUsage(); usage != 0 {
		t.Errorf("Initial memory usage = %d, want 0", usage)
	}

	// Apply memory update command
	command := Command{
		Type:        CommandUpdateMemoryUsage,
		MemoryDelta: 1000,
	}

	data, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	logEntry := &raft.Log{Data: data}
	result := fsm.Apply(logEntry)

	// Check result
	resultUsage, ok := result.(int64)
	if !ok {
		t.Fatalf("Apply() returned %T, expected int64", result)
	}

	if resultUsage != 1000 {
		t.Errorf("Apply() returned %d, want 1000", resultUsage)
	}

	// Verify memory usage was updated
	if usage := fsm.GetMemoryUsage(); usage != 1000 {
		t.Errorf("Memory usage = %d, want 1000", usage)
	}

	// Apply another update
	command.MemoryDelta = 500
	data, _ = json.Marshal(command)
	logEntry = &raft.Log{Data: data}
	fsm.Apply(logEntry)

	if usage := fsm.GetMemoryUsage(); usage != 1500 {
		t.Errorf("Memory usage = %d, want 1500", usage)
	}

	// Apply negative delta
	command.MemoryDelta = -500
	data, _ = json.Marshal(command)
	logEntry = &raft.Log{Data: data}
	fsm.Apply(logEntry)

	if usage := fsm.GetMemoryUsage(); usage != 1000 {
		t.Errorf("Memory usage = %d, want 1000", usage)
	}
}

func TestFSM_MemoryUsageNonNegative(t *testing.T) {
	fsm := NewFSM()

	// Apply negative delta that would make usage negative
	command := Command{
		Type:        CommandUpdateMemoryUsage,
		MemoryDelta: -1000,
	}

	data, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	logEntry := &raft.Log{Data: data}
	fsm.Apply(logEntry)

	// Memory usage should be clamped to 0
	if usage := fsm.GetMemoryUsage(); usage != 0 {
		t.Errorf("Memory usage = %d, want 0 (should be non-negative)", usage)
	}
}

func TestFSM_JobCountTracking(t *testing.T) {
	fsm := NewFSM()

	// Initial job count should be 0
	if count := fsm.GetJobCount(); count != 0 {
		t.Errorf("Initial job count = %d, want 0", count)
	}

	// Apply job count update command
	command := Command{
		Type:          CommandUpdateJobCount,
		JobCountDelta: 5,
	}

	data, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	logEntry := &raft.Log{Data: data}
	result := fsm.Apply(logEntry)

	// Check result
	resultCount, ok := result.(int64)
	if !ok {
		t.Fatalf("Apply() returned %T, expected int64", result)
	}

	if resultCount != 5 {
		t.Errorf("Apply() returned %d, want 5", resultCount)
	}

	// Verify job count was updated
	if count := fsm.GetJobCount(); count != 5 {
		t.Errorf("Job count = %d, want 5", count)
	}

	// Apply another update
	command.JobCountDelta = 3
	data, _ = json.Marshal(command)
	logEntry = &raft.Log{Data: data}
	fsm.Apply(logEntry)

	if count := fsm.GetJobCount(); count != 8 {
		t.Errorf("Job count = %d, want 8", count)
	}

	// Apply negative delta
	command.JobCountDelta = -3
	data, _ = json.Marshal(command)
	logEntry = &raft.Log{Data: data}
	fsm.Apply(logEntry)

	if count := fsm.GetJobCount(); count != 5 {
		t.Errorf("Job count = %d, want 5", count)
	}
}

func TestFSM_JobCountNonNegative(t *testing.T) {
	fsm := NewFSM()

	// Apply negative delta that would make count negative
	command := Command{
		Type:          CommandUpdateJobCount,
		JobCountDelta: -10,
	}

	data, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	logEntry := &raft.Log{Data: data}
	fsm.Apply(logEntry)

	// Job count should be clamped to 0
	if count := fsm.GetJobCount(); count != 0 {
		t.Errorf("Job count = %d, want 0 (should be non-negative)", count)
	}
}

func TestFSM_SnapshotWithCapacity(t *testing.T) {
	fsm := NewFSM()

	// Set some memory usage and job count
	memCmd := Command{
		Type:        CommandUpdateMemoryUsage,
		MemoryDelta: 5000,
	}
	data, _ := json.Marshal(memCmd)
	fsm.Apply(&raft.Log{Data: data})

	countCmd := Command{
		Type:          CommandUpdateJobCount,
		JobCountDelta: 10,
	}
	data, _ = json.Marshal(countCmd)
	fsm.Apply(&raft.Log{Data: data})

	// Create snapshot
	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	// Verify snapshot includes capacity data
	snap, ok := snapshot.(*Snapshot)
	if !ok {
		t.Fatalf("Snapshot is not *Snapshot type")
	}

	if snap.memoryUsage != 5000 {
		t.Errorf("Snapshot memory usage = %d, want 5000", snap.memoryUsage)
	}

	if snap.jobCount != 10 {
		t.Errorf("Snapshot job count = %d, want 10", snap.jobCount)
	}
}

func TestFSM_RestoreWithCapacity(t *testing.T) {
	// Create snapshot data with capacity fields
	snapshotData := struct {
		Jobs            map[string]*Job               `json:"jobs"`
		Slots           map[int64]*SlotData           `json:"slots"`
		ExecutionStates map[string]*JobExecutionState `json:"execution_states"`
		MemoryUsage     int64                         `json:"memory_usage"`
		JobCount        int64                         `json:"job_count"`
	}{
		Jobs:            make(map[string]*Job),
		Slots:           make(map[int64]*SlotData),
		ExecutionStates: make(map[string]*JobExecutionState),
		MemoryUsage:     7500,
		JobCount:        15,
	}

	data, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot data: %v", err)
	}

	// Create new FSM and restore
	fsm := NewFSM()
	reader := &mockCapacityReadCloser{data: data}

	err = fsm.Restore(reader)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Verify capacity was restored
	if usage := fsm.GetMemoryUsage(); usage != 7500 {
		t.Errorf("Restored memory usage = %d, want 7500", usage)
	}

	if count := fsm.GetJobCount(); count != 15 {
		t.Errorf("Restored job count = %d, want 15", count)
	}
}

// mockCapacityReadCloser for testing restore
type mockCapacityReadCloser struct {
	data []byte
	pos  int
}

func (m *mockCapacityReadCloser) Read(p []byte) (n int, err error) {
	if m.pos >= len(m.data) {
		return 0, nil
	}
	n = copy(p, m.data[m.pos:])
	m.pos += n
	return n, nil
}

func (m *mockCapacityReadCloser) Close() error {
	return nil
}

func TestProperty29_MemoryUpdatesThroughRaft(t *testing.T) {
	// **Feature: queue-size-limits, Property 29: Memory updates through Raft**
	// **Validates: Requirements 8.1**

	// This property verifies that memory updates are applied through Raft consensus
	// by checking that the FSM Apply method correctly processes memory update commands
	// and that the cumulative effect of multiple updates is correctly tracked

	rapid.Check(t, func(t *rapid.T) {
		fsm := NewFSM()

		// Generate a sequence of random memory deltas
		numUpdates := rapid.IntRange(1, 20).Draw(t, "num_updates")
		deltas := make([]int64, numUpdates)
		expectedTotal := int64(0)

		for i := 0; i < numUpdates; i++ {
			// Generate random delta (can be positive or negative)
			delta := rapid.Int64Range(-10000, 10000).Draw(t, fmt.Sprintf("delta_%d", i))
			deltas[i] = delta

			// Calculate expected total (clamped to 0 for negative values)
			expectedTotal += delta
			if expectedTotal < 0 {
				expectedTotal = 0
			}

			// Apply memory update through Raft
			command := Command{
				Type:        CommandUpdateMemoryUsage,
				MemoryDelta: delta,
			}

			data, err := json.Marshal(command)
			if err != nil {
				t.Fatalf("Failed to marshal command: %v", err)
			}

			logEntry := &raft.Log{Data: data}
			result := fsm.Apply(logEntry)

			// Verify the result is the updated memory usage
			resultUsage, ok := result.(int64)
			if !ok {
				t.Fatalf("Apply() returned %T, expected int64", result)
			}

			// Verify FSM state matches the expected total
			actualUsage := fsm.GetMemoryUsage()
			if actualUsage != expectedTotal {
				t.Fatalf("After %d updates: FSM memory usage = %d, want %d (last delta: %d)",
					i+1, actualUsage, expectedTotal, delta)
			}

			// Verify the Apply result matches the FSM state
			if resultUsage != actualUsage {
				t.Fatalf("Apply result (%d) doesn't match FSM state (%d)", resultUsage, actualUsage)
			}
		}

		// Final verification: memory usage should equal the sum of all deltas (clamped to 0)
		finalUsage := fsm.GetMemoryUsage()
		if finalUsage != expectedTotal {
			t.Fatalf("Final memory usage = %d, want %d", finalUsage, expectedTotal)
		}

		// Verify memory usage is non-negative
		if finalUsage < 0 {
			t.Fatalf("Memory usage should be non-negative, got %d", finalUsage)
		}
	})
}

func TestProperty32_MemoryUpdatesReplicated(t *testing.T) {
	// **Feature: queue-size-limits, Property 32: Memory updates replicated**
	// **Validates: Requirements 8.4**

	// This property verifies that memory updates are replicated across nodes
	// by testing that snapshot/restore preserves memory usage state

	// Create FSM and apply memory updates
	fsm1 := NewFSM()

	// Apply several memory updates
	updates := []int64{1000, 2000, 1500, -500}
	expectedTotal := int64(0)

	for _, delta := range updates {
		command := Command{
			Type:        CommandUpdateMemoryUsage,
			MemoryDelta: delta,
		}

		data, err := json.Marshal(command)
		if err != nil {
			t.Fatalf("Failed to marshal command: %v", err)
		}

		logEntry := &raft.Log{Data: data}
		fsm1.Apply(logEntry)
		expectedTotal += delta
	}

	// Verify FSM1 has the expected memory usage
	if usage := fsm1.GetMemoryUsage(); usage != expectedTotal {
		t.Errorf("FSM1 memory usage = %d, want %d", usage, expectedTotal)
	}

	// Create snapshot (simulating replication)
	snapshot, err := fsm1.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	// Persist snapshot to buffer
	var buf bytes.Buffer
	sink := &mockSnapshotSink{buf: &buf}

	err = snapshot.Persist(sink)
	if err != nil {
		t.Fatalf("Persist() error = %v", err)
	}

	// Create new FSM (simulating another node)
	fsm2 := NewFSM()

	// Restore from snapshot
	reader := &mockCapacityReadCloser{data: buf.Bytes()}
	err = fsm2.Restore(reader)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Verify FSM2 has the same memory usage as FSM1
	if usage := fsm2.GetMemoryUsage(); usage != expectedTotal {
		t.Errorf("FSM2 memory usage after restore = %d, want %d", usage, expectedTotal)
	}

	// Verify both FSMs have identical memory usage
	if fsm1.GetMemoryUsage() != fsm2.GetMemoryUsage() {
		t.Errorf("Memory usage mismatch: FSM1=%d, FSM2=%d",
			fsm1.GetMemoryUsage(), fsm2.GetMemoryUsage())
	}
}
