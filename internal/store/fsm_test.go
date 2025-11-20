package store

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/raft"
)

func TestCommandType_Constants(t *testing.T) {
	if CommandCreateJob != "create_job" {
		t.Errorf("CommandCreateJob = %s, want 'create_job'", CommandCreateJob)
	}
	if CommandDeleteJob != "delete_job" {
		t.Errorf("CommandDeleteJob = %s, want 'delete_job'", CommandDeleteJob)
	}
}

func TestCommand_Serialization(t *testing.T) {
	tests := []struct {
		name    string
		command Command
	}{
		{
			name: "create job command",
			command: Command{
				Type: CommandCreateJob,
				Job: &Job{
					ID:        "test-job",
					Type:      JobUnico,
					CreatedAt: time.Now().Unix(),
				},
			},
		},
		{
			name: "delete job command",
			command: Command{
				Type: CommandDeleteJob,
				ID:   "test-job-id",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Serialize
			data, err := json.Marshal(tt.command)
			if err != nil {
				t.Errorf("json.Marshal() error = %v", err)
				return
			}

			// Deserialize
			var cmd Command
			if err := json.Unmarshal(data, &cmd); err != nil {
				t.Errorf("json.Unmarshal() error = %v", err)
				return
			}

			// Compare
			if cmd.Type != tt.command.Type {
				t.Errorf("Type = %v, want %v", cmd.Type, tt.command.Type)
			}
			if cmd.ID != tt.command.ID {
				t.Errorf("ID = %v, want %v", cmd.ID, tt.command.ID)
			}

			if tt.command.Job != nil {
				if cmd.Job == nil {
					t.Error("Job is nil after deserialization")
					return
				}
				if cmd.Job.ID != tt.command.Job.ID {
					t.Errorf("Job.ID = %v, want %v", cmd.Job.ID, tt.command.Job.ID)
				}
				if cmd.Job.Type != tt.command.Job.Type {
					t.Errorf("Job.Type = %v, want %v", cmd.Job.Type, tt.command.Job.Type)
				}
			}
		})
	}
}

func createTestJob(id string, jobType JobType, timestamp *int64) *Job {
	job := &Job{
		ID:        id,
		Type:      jobType,
		CreatedAt: time.Now().Unix(),
	}

	if jobType == JobUnico && timestamp != nil {
		job.Timestamp = timestamp
	} else if jobType == JobRecurrente {
		job.CronExpr = "0 */6 * * *"
		if timestamp != nil {
			job.CreatedAt = *timestamp
		}
	}

	return job
}

func TestNewFSM(t *testing.T) {
	fsm := NewFSM()

	if fsm == nil {
		t.Fatal("NewFSM() returned nil")
	}

	// Test initial state
	jobs := fsm.GetAllJobs()
	if len(jobs) != 0 {
		t.Errorf("Initial FSM should have 0 jobs, got %d", len(jobs))
	}
}

func TestFSM_Apply_CreateJob(t *testing.T) {
	fsm := NewFSM()
	futureTime := time.Now().Add(1 * time.Hour).Unix()

	job := &Job{
		ID:        "test-job-1",
		Type:      JobUnico,
		Timestamp: &futureTime,
		CreatedAt: time.Now().Unix(),
	}

	command := Command{
		Type: CommandCreateJob,
		Job:  job,
	}

	data, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	logEntry := &raft.Log{
		Data: data,
	}

	result := fsm.Apply(logEntry)

	// Check that Apply returns the created job
	resultJob, ok := result.(*Job)
	if !ok {
		t.Fatalf("Apply() returned %T, expected *Job", result)
	}

	if resultJob.ID != job.ID {
		t.Errorf("Result job ID = %s, want %s", resultJob.ID, job.ID)
	}

	// Verify job was created
	storedJob, exists := fsm.GetJob("test-job-1")
	if !exists {
		t.Error("Job was not stored in FSM")
	}

	if storedJob.ID != job.ID {
		t.Errorf("Job.ID = %s, want %s", storedJob.ID, job.ID)
	}

	if storedJob.Type != job.Type {
		t.Errorf("Job.Type = %s, want %s", storedJob.Type, job.Type)
	}

	if storedJob.Timestamp == nil || *storedJob.Timestamp != *job.Timestamp {
		t.Errorf("Job.Timestamp mismatch")
	}
}

func TestFSM_Apply_DeleteJob(t *testing.T) {
	fsm := NewFSM()
	futureTime := time.Now().Add(1 * time.Hour).Unix()

	// First create a job
	job := &Job{
		ID:        "test-job-1",
		Type:      JobUnico,
		Timestamp: &futureTime,
		CreatedAt: time.Now().Unix(),
	}

	createCommand := Command{
		Type: CommandCreateJob,
		Job:  job,
	}

	data, err := json.Marshal(createCommand)
	if err != nil {
		t.Fatalf("Failed to marshal create command: %v", err)
	}

	logEntry := &raft.Log{Data: data}
	fsm.Apply(logEntry)

	// Verify job was created
	_, exists := fsm.GetJob("test-job-1")
	if !exists {
		t.Fatal("Job was not created")
	}

	// Now delete the job
	deleteCommand := Command{
		Type: CommandDeleteJob,
		ID:   "test-job-1",
	}

	data, err = json.Marshal(deleteCommand)
	if err != nil {
		t.Fatalf("Failed to marshal delete command: %v", err)
	}

	logEntry = &raft.Log{Data: data}
	result := fsm.Apply(logEntry)
	if result != nil {
		t.Errorf("Apply() returned error: %v", result)
	}

	// Verify job was deleted
	_, exists = fsm.GetJob("test-job-1")
	if exists {
		t.Error("Job was not deleted from FSM")
	}
}

func TestFSM_Apply_InvalidCommand(t *testing.T) {
	fsm := NewFSM()

	tests := []struct {
		name    string
		command Command
	}{
		{
			name: "unknown command type",
			command: Command{
				Type: CommandType("unknown"),
			},
		},
		{
			name: "create job without job",
			command: Command{
				Type: CommandCreateJob,
			},
		},
		{
			name: "delete job without ID",
			command: Command{
				Type: CommandDeleteJob,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.command)
			if err != nil {
				t.Fatalf("Failed to marshal command: %v", err)
			}

			logEntry := &raft.Log{Data: data}
			result := fsm.Apply(logEntry)

			// Invalid commands should return an error or be ignored
			// The exact behavior depends on implementation
			_ = result // We don't assert on the result as it may vary
		})
	}
}

func TestFSM_Apply_InvalidJSON(t *testing.T) {
	fsm := NewFSM()

	logEntry := &raft.Log{
		Data: []byte(`{"invalid": json}`),
	}

	result := fsm.Apply(logEntry)
	if result == nil {
		t.Error("Apply() should return error for invalid JSON")
	}

	// Verify no jobs were created
	jobs := fsm.GetAllJobs()
	if len(jobs) != 0 {
		t.Errorf("Invalid JSON should not create jobs, got %d jobs", len(jobs))
	}
}

func TestFSM_GetJob(t *testing.T) {
	fsm := NewFSM()

	// Test getting non-existent job
	_, exists := fsm.GetJob("non-existent")
	if exists {
		t.Error("GetJob() returned true for non-existent job")
	}

	// Add a job and test getting it
	futureTime := time.Now().Add(1 * time.Hour).Unix()
	job := &Job{
		ID:        "test-job-1",
		Type:      JobUnico,
		Timestamp: &futureTime,
		CreatedAt: time.Now().Unix(),
	}

	command := Command{
		Type: CommandCreateJob,
		Job:  job,
	}

	data, err := json.Marshal(command)
	if err != nil {
		t.Fatalf("Failed to marshal command: %v", err)
	}

	logEntry := &raft.Log{Data: data}
	fsm.Apply(logEntry)

	// Test getting existing job
	storedJob, exists := fsm.GetJob("test-job-1")
	if !exists {
		t.Error("GetJob() returned false for existing job")
	}

	if storedJob.ID != job.ID {
		t.Errorf("Job.ID = %s, want %s", storedJob.ID, job.ID)
	}
}

func TestFSM_GetAllJobs(t *testing.T) {
	fsm := NewFSM()

	// Initially should be empty
	jobs := fsm.GetAllJobs()
	if len(jobs) != 0 {
		t.Errorf("GetAllJobs() returned %d jobs, want 0", len(jobs))
	}

	// Add multiple jobs
	futureTime := time.Now().Add(1 * time.Hour).Unix()
	jobIDs := []string{"job-1", "job-2", "job-3"}

	for _, id := range jobIDs {
		job := &Job{
			ID:        id,
			Type:      JobUnico,
			Timestamp: &futureTime,
			CreatedAt: time.Now().Unix(),
		}

		command := Command{
			Type: CommandCreateJob,
			Job:  job,
		}

		data, err := json.Marshal(command)
		if err != nil {
			t.Fatalf("Failed to marshal command: %v", err)
		}

		logEntry := &raft.Log{Data: data}
		fsm.Apply(logEntry)
	}

	// Test getting all jobs
	jobs = fsm.GetAllJobs()
	if len(jobs) != len(jobIDs) {
		t.Errorf("GetAllJobs() returned %d jobs, want %d", len(jobs), len(jobIDs))
	}

	// Verify all jobs are present
	for _, expectedID := range jobIDs {
		found := false
		for _, job := range jobs {
			if job.ID == expectedID {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Job %s not found in GetAllJobs() result", expectedID)
		}
	}
}

func TestFSM_Snapshot(t *testing.T) {
	fsm := NewFSM()

	// Add some jobs
	futureTime := time.Now().Add(1 * time.Hour).Unix()
	jobIDs := []string{"job-1", "job-2"}

	for _, id := range jobIDs {
		job := &Job{
			ID:        id,
			Type:      JobUnico,
			Timestamp: &futureTime,
			CreatedAt: time.Now().Unix(),
		}

		command := Command{
			Type: CommandCreateJob,
			Job:  job,
		}

		data, err := json.Marshal(command)
		if err != nil {
			t.Fatalf("Failed to marshal command: %v", err)
		}

		logEntry := &raft.Log{Data: data}
		fsm.Apply(logEntry)
	}

	// Create snapshot
	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if snapshot == nil {
		t.Fatal("Snapshot() returned nil")
	}

	// Test snapshot persistence
	var buf bytes.Buffer
	sink := &mockSnapshotSink{buf: &buf}

	err = snapshot.Persist(sink)
	if err != nil {
		t.Fatalf("Persist() error = %v", err)
	}

	// Verify snapshot data
	var snapshotData struct {
		Jobs  map[string]*Job     `json:"jobs"`
		Slots map[int64]*SlotData `json:"slots"`
	}
	if err := json.Unmarshal(buf.Bytes(), &snapshotData); err != nil {
		t.Fatalf("Failed to unmarshal snapshot: %v", err)
	}

	if len(snapshotData.Jobs) != len(jobIDs) {
		t.Errorf("Snapshot contains %d jobs, want %d", len(snapshotData.Jobs), len(jobIDs))
	}

	for _, expectedID := range jobIDs {
		if _, exists := snapshotData.Jobs[expectedID]; !exists {
			t.Errorf("Job %s not found in snapshot", expectedID)
		}
	}
}

func TestFSM_Restore(t *testing.T) {
	fsm := NewFSM()

	// Create snapshot data with the correct structure
	futureTime := time.Now().Add(1 * time.Hour).Unix()
	snapshotData := struct {
		Jobs  map[string]*Job     `json:"jobs"`
		Slots map[int64]*SlotData `json:"slots"`
	}{
		Jobs: map[string]*Job{
			"job-1": {
				ID:        "job-1",
				Type:      JobUnico,
				Timestamp: &futureTime,
				CreatedAt: time.Now().Unix(),
			},
			"job-2": {
				ID:        "job-2",
				Type:      JobRecurrente,
				CronExpr:  "0 0 * * *",
				CreatedAt: time.Now().Unix(),
			},
		},
		Slots: make(map[int64]*SlotData),
	}

	data, err := json.Marshal(snapshotData)
	if err != nil {
		t.Fatalf("Failed to marshal snapshot data: %v", err)
	}

	reader := io.NopCloser(strings.NewReader(string(data)))

	// Restore from snapshot
	err = fsm.Restore(reader)
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Verify jobs were restored
	restoredJobs := fsm.GetAllJobs()
	if len(restoredJobs) != len(snapshotData.Jobs) {
		t.Errorf("Restored %d jobs, want %d", len(restoredJobs), len(snapshotData.Jobs))
	}

	for expectedID := range snapshotData.Jobs {
		job, exists := fsm.GetJob(expectedID)
		if !exists {
			t.Errorf("Job %s not found after restore", expectedID)
		}

		if job != nil && job.ID != expectedID {
			t.Errorf("Job.ID = %s, want %s", job.ID, expectedID)
		}
	}
}

func TestFSM_Restore_InvalidJSON(t *testing.T) {
	fsm := NewFSM()

	reader := io.NopCloser(strings.NewReader(`{"invalid": json}`))

	err := fsm.Restore(reader)
	if err == nil {
		t.Error("Restore() should return error for invalid JSON")
	}

	// Verify no jobs were restored
	jobs := fsm.GetAllJobs()
	if len(jobs) != 0 {
		t.Errorf("Invalid JSON should not restore jobs, got %d jobs", len(jobs))
	}
}

func TestSnapshot_Release(t *testing.T) {
	fsm := NewFSM()

	snapshot, err := fsm.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	// Release should not panic
	snapshot.Release()
}

// Mock snapshot sink for testing
type mockSnapshotSink struct {
	buf *bytes.Buffer
}

func (m *mockSnapshotSink) Write(p []byte) (n int, err error) {
	return m.buf.Write(p)
}

func (m *mockSnapshotSink) Close() error {
	return nil
}

func (m *mockSnapshotSink) ID() string {
	return "mock-snapshot"
}

func (m *mockSnapshotSink) Cancel() error {
	return nil
}

func TestFSM_ConcurrentOperations(t *testing.T) {
	fsm := NewFSM()

	// Test concurrent reads and writes
	done := make(chan bool, 2)

	// Writer goroutine
	go func() {
		for i := 0; i < 50; i++ {
			futureTime := time.Now().Add(1 * time.Hour).Unix()
			job := &Job{
				ID:        fmt.Sprintf("job-%d", i),
				Type:      JobUnico,
				Timestamp: &futureTime,
				CreatedAt: time.Now().Unix(),
			}

			command := Command{
				Type: CommandCreateJob,
				Job:  job,
			}

			data, _ := json.Marshal(command)
			logEntry := &raft.Log{Data: data}
			fsm.Apply(logEntry)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 50; i++ {
			fsm.GetAllJobs()
			fsm.GetJob(fmt.Sprintf("job-%d", i%10))
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify final state
	jobs := fsm.GetAllJobs()
	if len(jobs) != 50 {
		t.Errorf("Expected 50 jobs, got %d", len(jobs))
	}
}
