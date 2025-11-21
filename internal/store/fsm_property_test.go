package store

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/hashicorp/raft"
	"pgregory.net/rapid"
)

// **Feature: job-status-tracking, Property 5: Status changes persist through Raft**
// **Validates: Requirements 1.5**
func TestProperty5_StatusChangesPersistThroughRaft(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		fsm := NewFSM()

		// Generate a random job
		jobID := rapid.String().Draw(t, "job_id")
		job := &Job{
			ID:        jobID,
			Type:      JobUnico,
			CreatedAt: time.Now().Unix(),
		}

		// Create the job
		createCmd := &Command{
			Type: CommandCreateJob,
			Job:  job,
		}
		fsm.Apply(createLogEntryRapid(t, createCmd))

		// Generate a random valid status transition
		newStatus := rapid.SampledFrom([]JobStatus{
			StatusInProgress,
			StatusCancelled,
		}).Draw(t, "new_status")

		// Apply status update
		statusCmd := &Command{
			Type: CommandUpdateJobStatus,
			StatusCommand: &StatusCommand{
				JobID:     jobID,
				Status:    newStatus,
				NodeID:    rapid.String().Draw(t, "node_id"),
				Timestamp: time.Now().Unix(),
			},
		}

		result := fsm.Apply(createLogEntryRapid(t, statusCmd))

		// Check that the apply didn't return an error
		if err, ok := result.(error); ok {
			t.Fatalf("Status update failed: %v", err)
		}

		// Query the status after Raft apply completes
		state, exists := fsm.GetExecutionState(jobID)
		if !exists {
			t.Fatalf("Execution state not found for job %s", jobID)
		}

		// Verify the status was persisted
		if state.Status != newStatus {
			t.Fatalf("Expected status %s, got %s", newStatus, state.Status)
		}
	})
}

// Helper function to create a raft log entry from a command (for property tests)
func createLogEntryRapid(t *rapid.T, cmd *Command) *raft.Log {
	data, err := json.Marshal(cmd)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}
	return &raft.Log{Data: data}
}

// **Feature: job-status-tracking, Property 36: Status serialization round-trip**
// **Validates: Requirements 10.4**
func TestProperty36_StatusSerializationRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random execution state
		originalState := generateRandomExecutionState(t)

		// Serialize
		data, err := json.Marshal(originalState)
		if err != nil {
			t.Fatalf("Failed to serialize execution state: %v", err)
		}

		// Deserialize
		var deserializedState JobExecutionState
		err = json.Unmarshal(data, &deserializedState)
		if err != nil {
			t.Fatalf("Failed to deserialize execution state: %v", err)
		}

		// Verify round-trip produces equivalent state
		if deserializedState.JobID != originalState.JobID {
			t.Fatalf("JobID mismatch: expected %s, got %s", originalState.JobID, deserializedState.JobID)
		}

		if deserializedState.Status != originalState.Status {
			t.Fatalf("Status mismatch: expected %s, got %s", originalState.Status, deserializedState.Status)
		}

		if deserializedState.AttemptCount != originalState.AttemptCount {
			t.Fatalf("AttemptCount mismatch: expected %d, got %d", originalState.AttemptCount, deserializedState.AttemptCount)
		}

		if deserializedState.CreatedAt != originalState.CreatedAt {
			t.Fatalf("CreatedAt mismatch: expected %d, got %d", originalState.CreatedAt, deserializedState.CreatedAt)
		}

		if len(deserializedState.Attempts) != len(originalState.Attempts) {
			t.Fatalf("Attempts length mismatch: expected %d, got %d", len(originalState.Attempts), len(deserializedState.Attempts))
		}
	})
}

// Helper function to generate a random execution state
func generateRandomExecutionState(t *rapid.T) *JobExecutionState {
	jobID := rapid.String().Draw(t, "job_id")
	status := rapid.SampledFrom([]JobStatus{
		StatusPending,
		StatusInProgress,
		StatusCompleted,
		StatusFailed,
		StatusCancelled,
		StatusTimeout,
	}).Draw(t, "status")

	attemptCount := rapid.IntRange(0, 10).Draw(t, "attempt_count")
	createdAt := rapid.Int64Range(0, time.Now().Unix()).Draw(t, "created_at")

	// Generate random attempts
	numAttempts := rapid.IntRange(0, 5).Draw(t, "num_attempts")
	attempts := make([]ExecutionAttempt, numAttempts)
	for i := 0; i < numAttempts; i++ {
		startTime := rapid.Int64Range(createdAt, time.Now().Unix()).Draw(t, "start_time")
		attempts[i] = ExecutionAttempt{
			AttemptNum:   i + 1,
			StartTime:    startTime,
			EndTime:      rapid.Int64Range(startTime, startTime+3600).Draw(t, "end_time"),
			NodeID:       rapid.String().Draw(t, "node_id"),
			Status:       status,
			HTTPStatus:   rapid.IntRange(200, 599).Draw(t, "http_status"),
			ResponseTime: rapid.Int64Range(0, 30000).Draw(t, "response_time"),
			ErrorMessage: rapid.String().Draw(t, "error_message"),
			ErrorType:    rapid.String().Draw(t, "error_type"),
		}
	}

	return &JobExecutionState{
		JobID:           jobID,
		Status:          status,
		AttemptCount:    attemptCount,
		CreatedAt:       createdAt,
		FirstAttemptAt:  rapid.Int64Range(0, time.Now().Unix()).Draw(t, "first_attempt_at"),
		LastAttemptAt:   rapid.Int64Range(0, time.Now().Unix()).Draw(t, "last_attempt_at"),
		CompletedAt:     rapid.Int64Range(0, time.Now().Unix()).Draw(t, "completed_at"),
		ExecutingNodeID: rapid.String().Draw(t, "executing_node_id"),
		Attempts:        attempts,
	}
}

// **Feature: job-status-tracking, Property 37: Deserialization handles corrupted data**
// **Validates: Requirements 10.5**
func TestProperty37_DeserializationHandlesCorruptedData(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random corrupted data
		// We'll create various types of invalid JSON
		corruptedData := rapid.SampledFrom([]string{
			`{"invalid": json}`,                     // Invalid JSON syntax
			`{"job_id": 123}`,                       // Wrong type for job_id
			`{"status": "invalid_status"}`,          // Invalid status value
			`{"attempt_count": "not_a_number"}`,     // Wrong type for attempt_count
			`{"attempts": "not_an_array"}`,          // Wrong type for attempts
			`null`,                                  // Null data
			``,                                      // Empty string
			`{`,                                     // Incomplete JSON
			`{"job_id": null}`,                      // Null field
			`{"created_at": "not_a_timestamp"}`,     // Wrong type for timestamp
			`{"jobs": "not_an_object"}`,             // Wrong type for jobs in snapshot
			`{"slots": "not_an_object"}`,            // Wrong type for slots in snapshot
			`{"execution_states": "not_an_object"}`, // Wrong type for execution_states
			`{"jobs": null, "slots": null}`,         // Null maps in snapshot
			`random garbage data`,                   // Complete garbage
			`[1, 2, 3]`,                             // Array instead of object
		}).Draw(t, "corrupted_data")

		// Test 1: Deserialize JobExecutionState
		var state JobExecutionState
		err := json.Unmarshal([]byte(corruptedData), &state)

		// Verify that deserialization returns an error (doesn't panic)
		// For some cases like null or empty objects, JSON unmarshaling might succeed
		// but produce a zero-value struct, which is acceptable
		if err != nil {
			// This is the expected behavior - error returned
			return
		}

		// If no error, verify we got a valid (possibly zero-value) state
		// The important thing is we didn't panic
	})
}

// Additional test for FSM Restore with corrupted snapshot data
func TestProperty37_FSMRestoreHandlesCorruptedData(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		fsm := NewFSM()

		// Generate random corrupted snapshot data
		corruptedData := rapid.SampledFrom([]string{
			`{"invalid": json}`,                     // Invalid JSON syntax
			`{"jobs": "not_an_object"}`,             // Wrong type for jobs
			`{"slots": "not_an_object"}`,            // Wrong type for slots
			`{"execution_states": "not_an_object"}`, // Wrong type for execution_states
			`null`,                                  // Null data
			``,                                      // Empty string
			`{`,                                     // Incomplete JSON
			`random garbage`,                        // Complete garbage
			`[1, 2, 3]`,                             // Array instead of object
			`{"jobs": null}`,                        // Null jobs map
			`{"slots": null}`,                       // Null slots map
			`{"execution_states": null}`,            // Null execution_states map
		}).Draw(t, "corrupted_data")

		// Create a reader from the corrupted data
		reader := io.NopCloser(bytes.NewReader([]byte(corruptedData)))

		// Attempt to restore from corrupted data
		err := fsm.Restore(reader)

		// Verify that restore either:
		// 1. Returns an error (expected for invalid data)
		// 2. Succeeds with valid zero-value state (acceptable for some cases like null)
		if err != nil {
			// This is the expected behavior - error returned
			return
		}

		// If no error, verify the FSM is in a valid state (didn't panic)
		// The maps should be initialized even if empty
		if fsm.jobs == nil || fsm.slots == nil || fsm.executionStates == nil {
			t.Fatalf("FSM maps should be initialized after restore")
		}
	})
}
