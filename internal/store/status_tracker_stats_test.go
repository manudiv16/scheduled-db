package store

import (
	"testing"
	"time"

	"pgregory.net/rapid"
)

// Property 38: Execution stats accuracy
func TestProperty38_ExecutionStatsAccuracy(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate counts for each status
		pending := rapid.Int64Range(0, 50).Draw(rt, "pending")
		inProgress := rapid.Int64Range(0, 50).Draw(rt, "inProgress")
		completed := rapid.Int64Range(0, 50).Draw(rt, "completed")
		failed := rapid.Int64Range(0, 50).Draw(rt, "failed")
		cancelled := rapid.Int64Range(0, 50).Draw(rt, "cancelled")
		timeout := rapid.Int64Range(0, 50).Draw(rt, "timeout")

		// Create FSM
		fsm := NewFSM()

		// Populate FSM with jobs in different states
		populateFSM(rt, fsm, StatusPending, pending)
		populateFSM(rt, fsm, StatusInProgress, inProgress)
		populateFSM(rt, fsm, StatusCompleted, completed)
		populateFSM(rt, fsm, StatusFailed, failed)
		populateFSM(rt, fsm, StatusCancelled, cancelled)
		populateFSM(rt, fsm, StatusTimeout, timeout)

		// Create StatusTracker with mock store (we only need the FSM access)
		// Since StatusTracker accesses store.fsm directly, we need a Store struct.
		// However, we can't easily inject a mock store into StatusTracker because it expects *Store.
		// But we can construct a Store with our FSM.
		store := &Store{
			fsm: fsm,
		}
		tracker := NewStatusTracker(store)

		// Calculate stats
		stats, err := tracker.GetExecutionStats()
		if err != nil {
			rt.Fatalf("GetExecutionStats failed: %v", err)
		}

		// Verify counts
		if stats.Pending != pending {
			rt.Fatalf("Pending mismatch: expected %d, got %d", pending, stats.Pending)
		}
		if stats.InProgress != inProgress {
			rt.Fatalf("InProgress mismatch: expected %d, got %d", inProgress, stats.InProgress)
		}
		if stats.Completed != completed {
			rt.Fatalf("Completed mismatch: expected %d, got %d", completed, stats.Completed)
		}
		if stats.Failed != failed {
			rt.Fatalf("Failed mismatch: expected %d, got %d", failed, stats.Failed)
		}
		if stats.Cancelled != cancelled {
			rt.Fatalf("Cancelled mismatch: expected %d, got %d", cancelled, stats.Cancelled)
		}
		if stats.Timeout != timeout {
			rt.Fatalf("Timeout mismatch: expected %d, got %d", timeout, stats.Timeout)
		}

		total := pending + inProgress + completed + failed + cancelled + timeout
		if stats.Total != total {
			rt.Fatalf("Total mismatch: expected %d, got %d", total, stats.Total)
		}

		// Verify failure rate
		finishedCount := completed + failed + timeout
		var expectedFailureRate float64
		if finishedCount > 0 {
			expectedFailureRate = float64(failed+timeout) / float64(finishedCount)
		} else {
			expectedFailureRate = 0
		}

		if stats.FailureRate != expectedFailureRate {
			rt.Fatalf("FailureRate mismatch: expected %f, got %f", expectedFailureRate, stats.FailureRate)
		}
	})
}

func populateFSM(rt *rapid.T, fsm *FSM, status JobStatus, count int64) {
	for i := int64(0); i < count; i++ {
		jobID := rapid.String().Draw(rt, "jobID")
		// Ensure uniqueness
		for {
			if _, exists := fsm.executionStates[jobID]; !exists {
				break
			}
			jobID = jobID + "x"
		}

		state := &JobExecutionState{
			JobID:     jobID,
			Status:    status,
			CreatedAt: time.Now().Unix(),
		}

		if status == StatusCompleted || status == StatusFailed {
			state.CompletedAt = time.Now().Unix()
		}

		fsm.executionStates[jobID] = state
	}
}
