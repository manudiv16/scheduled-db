package store

import (
	"testing"
	"time"
)

func TestStatusTracker_GetStatus(t *testing.T) {
	fsm := NewFSM()

	// Create a job with execution state
	jobID := "test-job-1"
	state := &JobExecutionState{
		JobID:     jobID,
		Status:    StatusPending,
		CreatedAt: time.Now().Unix(),
		Attempts:  []ExecutionAttempt{},
	}
	fsm.executionStates[jobID] = state

	store := &Store{fsm: fsm}
	tracker := NewStatusTracker(store)

	// Test GetStatus
	result, err := tracker.GetStatus(jobID)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if result.JobID != jobID {
		t.Errorf("expected job ID %s, got %s", jobID, result.JobID)
	}

	if result.Status != StatusPending {
		t.Errorf("expected status %s, got %s", StatusPending, result.Status)
	}
}

func TestStatusTracker_GetStatus_NotFound(t *testing.T) {
	fsm := NewFSM()
	store := &Store{fsm: fsm}
	tracker := NewStatusTracker(store)

	_, err := tracker.GetStatus("non-existent-job")
	if err == nil {
		t.Error("expected error for non-existent job, got nil")
	}
}

func TestStatusTracker_CanExecute(t *testing.T) {
	tests := []struct {
		name     string
		status   JobStatus
		expected bool
	}{
		{"pending job can execute", StatusPending, true},
		{"timeout job can execute", StatusTimeout, true},
		{"failed job can execute", StatusFailed, true},
		{"completed job cannot execute", StatusCompleted, false},
		{"cancelled job cannot execute", StatusCancelled, false},
		{"in-progress job cannot execute", StatusInProgress, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsm := NewFSM()
			jobID := "test-job"
			state := &JobExecutionState{
				JobID:     jobID,
				Status:    tt.status,
				CreatedAt: time.Now().Unix(),
				Attempts:  []ExecutionAttempt{},
			}
			fsm.executionStates[jobID] = state

			store := &Store{fsm: fsm}
			tracker := NewStatusTracker(store)

			canExec, err := tracker.CanExecute(jobID)
			if err != nil {
				t.Fatalf("CanExecute failed: %v", err)
			}

			if canExec != tt.expected {
				t.Errorf("expected CanExecute=%v, got %v", tt.expected, canExec)
			}
		})
	}
}

func TestStatusTracker_CanExecute_NoState(t *testing.T) {
	fsm := NewFSM()
	store := &Store{fsm: fsm}
	tracker := NewStatusTracker(store)

	// Job with no execution state should be executable
	canExec, err := tracker.CanExecute("new-job")
	if err != nil {
		t.Fatalf("CanExecute failed: %v", err)
	}

	if !canExec {
		t.Error("expected new job to be executable")
	}
}

func TestStatusTracker_ListByStatus(t *testing.T) {
	fsm := NewFSM()

	// Create multiple jobs with different statuses
	jobs := []struct {
		id     string
		status JobStatus
	}{
		{"job-1", StatusPending},
		{"job-2", StatusCompleted},
		{"job-3", StatusPending},
		{"job-4", StatusFailed},
		{"job-5", StatusPending},
	}

	for _, job := range jobs {
		fsm.executionStates[job.id] = &JobExecutionState{
			JobID:     job.id,
			Status:    job.status,
			CreatedAt: time.Now().Unix(),
			Attempts:  []ExecutionAttempt{},
		}
	}

	store := &Store{fsm: fsm}
	tracker := NewStatusTracker(store)

	// List pending jobs
	pending, err := tracker.ListByStatus(StatusPending)
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}

	if len(pending) != 3 {
		t.Errorf("expected 3 pending jobs, got %d", len(pending))
	}

	// List completed jobs
	completed, err := tracker.ListByStatus(StatusCompleted)
	if err != nil {
		t.Fatalf("ListByStatus failed: %v", err)
	}

	if len(completed) != 1 {
		t.Errorf("expected 1 completed job, got %d", len(completed))
	}
}

func TestStatusTracker_GetExecutionHistory(t *testing.T) {
	fsm := NewFSM()
	jobID := "test-job"

	attempts := []ExecutionAttempt{
		{
			AttemptNum: 1,
			StartTime:  time.Now().Unix(),
			EndTime:    time.Now().Unix() + 10,
			NodeID:     "node-1",
			Status:     StatusFailed,
		},
		{
			AttemptNum: 2,
			StartTime:  time.Now().Unix() + 20,
			EndTime:    time.Now().Unix() + 30,
			NodeID:     "node-1",
			Status:     StatusCompleted,
		},
	}

	fsm.executionStates[jobID] = &JobExecutionState{
		JobID:     jobID,
		Status:    StatusCompleted,
		CreatedAt: time.Now().Unix(),
		Attempts:  attempts,
	}

	store := &Store{fsm: fsm}
	tracker := NewStatusTracker(store)

	history, err := tracker.GetExecutionHistory(jobID)
	if err != nil {
		t.Fatalf("GetExecutionHistory failed: %v", err)
	}

	if len(history) != 2 {
		t.Errorf("expected 2 attempts, got %d", len(history))
	}

	if history[0].AttemptNum != 1 {
		t.Errorf("expected first attempt number 1, got %d", history[0].AttemptNum)
	}

	if history[1].AttemptNum != 2 {
		t.Errorf("expected second attempt number 2, got %d", history[1].AttemptNum)
	}
}

func TestStatusTracker_GetExecutionHistory_NotFound(t *testing.T) {
	fsm := NewFSM()
	store := &Store{fsm: fsm}
	tracker := NewStatusTracker(store)

	_, err := tracker.GetExecutionHistory("non-existent-job")
	if err == nil {
		t.Error("expected error for non-existent job, got nil")
	}
}

func TestFSM_ApplyPruneAttempts(t *testing.T) {
	fsm := NewFSM()
	jobID := "test-job"

	now := time.Now().Unix()
	oldTime := now - (40 * 24 * 60 * 60)    // 40 days ago
	recentTime := now - (10 * 24 * 60 * 60) // 10 days ago

	// Create attempts with different ages
	attempts := []ExecutionAttempt{
		{
			AttemptNum: 1,
			StartTime:  oldTime,
			EndTime:    oldTime + 10,
			NodeID:     "node-1",
			Status:     StatusFailed,
		},
		{
			AttemptNum: 2,
			StartTime:  oldTime + 100,
			EndTime:    oldTime + 110,
			NodeID:     "node-1",
			Status:     StatusFailed,
		},
		{
			AttemptNum: 3,
			StartTime:  recentTime,
			EndTime:    recentTime + 10,
			NodeID:     "node-1",
			Status:     StatusCompleted,
		},
	}

	fsm.executionStates[jobID] = &JobExecutionState{
		JobID:     jobID,
		Status:    StatusCompleted,
		CreatedAt: oldTime,
		Attempts:  attempts,
	}

	// Keep only the recent attempt
	keptAttempts := []ExecutionAttempt{
		{
			AttemptNum: 3,
			StartTime:  recentTime,
			EndTime:    recentTime + 10,
			NodeID:     "node-1",
			Status:     StatusCompleted,
		},
	}

	// Apply pruning
	cmd := &StatusCommand{
		JobID:     jobID,
		Timestamp: now,
	}

	err := fsm.applyPruneAttempts(cmd, keptAttempts)
	if err != nil {
		t.Fatalf("applyPruneAttempts failed: %v", err)
	}

	// Verify that only the recent attempt remains
	state := fsm.executionStates[jobID]
	if len(state.Attempts) != 1 {
		t.Errorf("expected 1 attempt after pruning, got %d", len(state.Attempts))
	}

	if state.Attempts[0].AttemptNum != 3 {
		t.Errorf("expected attempt 3 to remain, got attempt %d", state.Attempts[0].AttemptNum)
	}
}

func TestFSM_ApplyPruneAttempts_NotFound(t *testing.T) {
	fsm := NewFSM()

	cmd := &StatusCommand{
		JobID:     "non-existent-job",
		Timestamp: time.Now().Unix(),
	}

	err := fsm.applyPruneAttempts(cmd, []ExecutionAttempt{})
	if err == nil {
		t.Error("expected error for non-existent job, got nil")
	}
}

func TestPruningLogic(t *testing.T) {
	// Test the pruning logic without requiring a full Raft setup
	now := time.Now().Unix()
	oldTime := now - (40 * 24 * 60 * 60)    // 40 days ago
	recentTime := now - (10 * 24 * 60 * 60) // 10 days ago

	attempts := []ExecutionAttempt{
		{AttemptNum: 1, StartTime: oldTime},
		{AttemptNum: 2, StartTime: oldTime + 100},
		{AttemptNum: 3, StartTime: recentTime},
	}

	retention := 30 * 24 * time.Hour
	cutoffTime := time.Now().Add(-retention).Unix()

	var keptAttempts []ExecutionAttempt
	var prunedCount int

	for _, attempt := range attempts {
		if attempt.StartTime >= cutoffTime {
			keptAttempts = append(keptAttempts, attempt)
		} else {
			prunedCount++
		}
	}

	// Verify that old attempts would be pruned
	if prunedCount != 2 {
		t.Errorf("expected 2 attempts to be pruned, got %d", prunedCount)
	}

	if len(keptAttempts) != 1 {
		t.Errorf("expected 1 attempt to be kept, got %d", len(keptAttempts))
	}

	if len(keptAttempts) > 0 && keptAttempts[0].AttemptNum != 3 {
		t.Errorf("expected kept attempt to be attempt 3, got %d", keptAttempts[0].AttemptNum)
	}
}
