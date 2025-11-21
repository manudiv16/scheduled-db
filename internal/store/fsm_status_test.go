package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

// Helper function to create a raft log entry from a command
func createLogEntry(t *testing.T, cmd *Command) *raft.Log {
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}
	return &raft.Log{Data: data}
}

func TestFSM_ExecutionStateInitialization(t *testing.T) {
	fsm := NewFSM()

	// Create a job
	job := &Job{
		ID:        "test-job-1",
		Type:      JobUnico,
		CreatedAt: time.Now().Unix(),
	}

	// Apply create job command
	cmd := &Command{
		Type: CommandCreateJob,
		Job:  job,
	}

	result := fsm.Apply(createLogEntry(t, cmd))
	if err, ok := result.(error); ok {
		t.Fatalf("Failed to create job: %v", err)
	}

	// Verify execution state was initialized
	state, exists := fsm.GetExecutionState(job.ID)
	if !exists {
		t.Fatal("Execution state not found for job")
	}

	if state.Status != StatusPending {
		t.Errorf("Expected status %s, got %s", StatusPending, state.Status)
	}

	if state.JobID != job.ID {
		t.Errorf("Expected job ID %s, got %s", job.ID, state.JobID)
	}

	if len(state.Attempts) != 0 {
		t.Errorf("Expected 0 attempts, got %d", len(state.Attempts))
	}
}

func TestFSM_StatusUpdate(t *testing.T) {
	fsm := NewFSM()

	// Create a job first
	job := &Job{
		ID:        "test-job-2",
		Type:      JobUnico,
		CreatedAt: time.Now().Unix(),
	}

	createCmd := &Command{
		Type: CommandCreateJob,
		Job:  job,
	}
	fsm.Apply(createLogEntry(t, createCmd))

	// Update status to in_progress
	statusCmd := &Command{
		Type: CommandUpdateJobStatus,
		StatusCommand: &StatusCommand{
			JobID:     job.ID,
			Status:    StatusInProgress,
			NodeID:    "node-1",
			Timestamp: time.Now().Unix(),
		},
	}

	result := fsm.Apply(createLogEntry(t, statusCmd))
	if err, ok := result.(error); ok {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Verify status was updated
	state, exists := fsm.GetExecutionState(job.ID)
	if !exists {
		t.Fatal("Execution state not found")
	}

	if state.Status != StatusInProgress {
		t.Errorf("Expected status %s, got %s", StatusInProgress, state.Status)
	}

	if state.ExecutingNodeID != "node-1" {
		t.Errorf("Expected node ID 'node-1', got %s", state.ExecutingNodeID)
	}

	if state.AttemptCount != 1 {
		t.Errorf("Expected attempt count 1, got %d", state.AttemptCount)
	}
}

func TestFSM_InvalidStateTransition(t *testing.T) {
	fsm := NewFSM()

	// Create a job and mark it completed
	job := &Job{
		ID:        "test-job-3",
		Type:      JobUnico,
		CreatedAt: time.Now().Unix(),
	}

	createCmd := &Command{
		Type: CommandCreateJob,
		Job:  job,
	}
	fsm.Apply(createLogEntry(t, createCmd))

	// Mark as in_progress
	statusCmd1 := &Command{
		Type: CommandUpdateJobStatus,
		StatusCommand: &StatusCommand{
			JobID:     job.ID,
			Status:    StatusInProgress,
			NodeID:    "node-1",
			Timestamp: time.Now().Unix(),
		},
	}
	fsm.Apply(createLogEntry(t, statusCmd1))

	// Mark as completed
	statusCmd2 := &Command{
		Type: CommandUpdateJobStatus,
		StatusCommand: &StatusCommand{
			JobID:     job.ID,
			Status:    StatusCompleted,
			Timestamp: time.Now().Unix(),
		},
	}
	fsm.Apply(createLogEntry(t, statusCmd2))

	// Try to transition from completed to pending (invalid)
	statusCmd3 := &Command{
		Type: CommandUpdateJobStatus,
		StatusCommand: &StatusCommand{
			JobID:     job.ID,
			Status:    StatusPending,
			Timestamp: time.Now().Unix(),
		},
	}

	result := fsm.Apply(createLogEntry(t, statusCmd3))
	if _, ok := result.(error); !ok {
		t.Fatal("Expected error for invalid state transition")
	}
}

func TestFSM_RecordAttempt(t *testing.T) {
	fsm := NewFSM()

	// Create a job
	job := &Job{
		ID:        "test-job-4",
		Type:      JobUnico,
		CreatedAt: time.Now().Unix(),
	}

	createCmd := &Command{
		Type: CommandCreateJob,
		Job:  job,
	}
	fsm.Apply(createLogEntry(t, createCmd))

	// Record an attempt
	attempt := &ExecutionAttempt{
		AttemptNum:   1,
		StartTime:    time.Now().Unix(),
		EndTime:      time.Now().Unix() + 5,
		NodeID:       "node-1",
		Status:       StatusCompleted,
		HTTPStatus:   200,
		ResponseTime: 5000,
	}

	recordCmd := &Command{
		Type: CommandRecordAttempt,
		StatusCommand: &StatusCommand{
			JobID:   job.ID,
			Attempt: attempt,
		},
	}

	result := fsm.Apply(createLogEntry(t, recordCmd))
	if err, ok := result.(error); ok {
		t.Fatalf("Failed to record attempt: %v", err)
	}

	// Verify attempt was recorded
	state, exists := fsm.GetExecutionState(job.ID)
	if !exists {
		t.Fatal("Execution state not found")
	}

	if len(state.Attempts) != 1 {
		t.Fatalf("Expected 1 attempt, got %d", len(state.Attempts))
	}

	recorded := state.Attempts[0]
	if recorded.AttemptNum != attempt.AttemptNum {
		t.Errorf("Expected attempt num %d, got %d", attempt.AttemptNum, recorded.AttemptNum)
	}

	if recorded.HTTPStatus != attempt.HTTPStatus {
		t.Errorf("Expected HTTP status %d, got %d", attempt.HTTPStatus, recorded.HTTPStatus)
	}
}

func TestFSM_SnapshotWithExecutionStates(t *testing.T) {
	fsm := NewFSM()

	// Create jobs with execution states
	for i := 1; i <= 3; i++ {
		job := &Job{
			ID:        "test-job-" + string(rune('0'+i)),
			Type:      JobUnico,
			CreatedAt: time.Now().Unix(),
		}

		createCmd := &Command{
			Type: CommandCreateJob,
			Job:  job,
		}
		fsm.Apply(createLogEntry(t, createCmd))
	}

	// Create snapshot
	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// Verify snapshot includes execution states
	snap, ok := snapshot.(*Snapshot)
	if !ok {
		t.Fatal("Failed to cast snapshot to *Snapshot")
	}
	if len(snap.executionStates) != 3 {
		t.Errorf("Expected 3 execution states in snapshot, got %d", len(snap.executionStates))
	}
}

func TestFSM_RestoreWithExecutionStates(t *testing.T) {
	fsm := NewFSM()

	// Create snapshot data with execution states
	snapshotData := struct {
		Jobs            map[string]*Job               `json:"jobs"`
		Slots           map[int64]*SlotData           `json:"slots"`
		ExecutionStates map[string]*JobExecutionState `json:"execution_states"`
	}{
		Jobs: map[string]*Job{
			"job-1": {
				ID:        "job-1",
				Type:      JobUnico,
				CreatedAt: time.Now().Unix(),
			},
		},
		Slots: make(map[int64]*SlotData),
		ExecutionStates: map[string]*JobExecutionState{
			"job-1": {
				JobID:           "job-1",
				Status:          StatusInProgress,
				AttemptCount:    1,
				CreatedAt:       time.Now().Unix(),
				FirstAttemptAt:  time.Now().Unix(),
				LastAttemptAt:   time.Now().Unix(),
				ExecutingNodeID: "node-1",
				Attempts:        []ExecutionAttempt{},
			},
		},
	}

	data, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot data: %v", err)
	}

	reader := &mockReadCloser{data: data}
	if err := fsm.Restore(reader); err != nil {
		t.Fatalf("Failed to restore snapshot: %v", err)
	}

	// Verify execution state was restored
	state, exists := fsm.GetExecutionState("job-1")
	if !exists {
		t.Fatal("Execution state not found after restore")
	}

	if state.Status != StatusInProgress {
		t.Errorf("Expected status %s, got %s", StatusInProgress, state.Status)
	}

	if state.ExecutingNodeID != "node-1" {
		t.Errorf("Expected node ID 'node-1', got %s", state.ExecutingNodeID)
	}

	if state.AttemptCount != 1 {
		t.Errorf("Expected attempt count 1, got %d", state.AttemptCount)
	}
}

// mockReadCloser implements io.ReadCloser for testing
type mockReadCloser struct {
	data   []byte
	offset int
}

func (m *mockReadCloser) Read(p []byte) (n int, err error) {
	if m.offset >= len(m.data) {
		return 0, nil
	}
	n = copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	return nil
}

func TestFSM_DeleteJobRemovesExecutionState(t *testing.T) {
	fsm := NewFSM()

	// Create a job
	job := &Job{
		ID:        "test-job-delete",
		Type:      JobUnico,
		CreatedAt: time.Now().Unix(),
	}

	createCmd := &Command{
		Type: CommandCreateJob,
		Job:  job,
	}
	fsm.Apply(createLogEntry(t, createCmd))

	// Verify execution state exists
	_, exists := fsm.GetExecutionState(job.ID)
	if !exists {
		t.Fatal("Execution state should exist after job creation")
	}

	// Delete job
	deleteCmd := &Command{
		Type: CommandDeleteJob,
		ID:   job.ID,
	}
	fsm.Apply(createLogEntry(t, deleteCmd))

	// Verify execution state was removed
	_, exists = fsm.GetExecutionState(job.ID)
	if exists {
		t.Fatal("Execution state should be removed after job deletion")
	}
}
