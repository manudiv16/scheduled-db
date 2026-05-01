package store

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"scheduled-db/internal/logger"

	"github.com/hashicorp/raft"
)

type CommandType string

const (
	CommandCreateJob         CommandType = "create_job"
	CommandDeleteJob         CommandType = "delete_job"
	CommandCreateSlot        CommandType = "create_slot"
	CommandDeleteSlot        CommandType = "delete_slot"
	CommandUpdateJobStatus   CommandType = "update_job_status"
	CommandRecordAttempt     CommandType = "record_attempt"
	CommandPruneAttempts     CommandType = "prune_attempts"
	CommandUpdateMemoryUsage CommandType = "update_memory_usage"
	CommandUpdateJobCount    CommandType = "update_job_count"
)

type Command struct {
	Type          CommandType        `json:"type"`
	Job           *Job               `json:"job,omitempty"`
	ID            string             `json:"id,omitempty"`
	Slot          *SlotData          `json:"slot,omitempty"`
	StatusCommand *StatusCommand     `json:"status_command,omitempty"`
	Attempts      []ExecutionAttempt `json:"attempts,omitempty"`
	MemoryDelta   int64              `json:"memory_delta,omitempty"`
	JobCountDelta int64              `json:"job_count_delta,omitempty"`
}

// StatusCommand represents a status update command
type StatusCommand struct {
	JobID              string            `json:"job_id"`
	Status             JobStatus         `json:"status"`
	NodeID             string            `json:"node_id,omitempty"`
	Attempt            *ExecutionAttempt `json:"attempt,omitempty"`
	Timestamp          int64             `json:"timestamp"`
	CancellationReason string            `json:"cancellation_reason,omitempty"`
}

// validTransitions is the canonical state machine for job status transitions.
// Pre-allocated to avoid per-call map allocation.
var validTransitions = map[JobStatus][]JobStatus{
	StatusPending:    {StatusInProgress, StatusCancelled},
	StatusInProgress: {StatusCompleted, StatusFailed, StatusTimeout, StatusCancelled},
	StatusTimeout:    {StatusInProgress},
	StatusFailed:     {StatusInProgress},
	StatusCompleted:  {},
	StatusCancelled:  {},
}

// FSM implements the raft.FSM interface
type FSM struct {
	mu              sync.RWMutex
	jobs            map[string]*Job
	slots           map[int64]*SlotData
	executionStates map[string]*JobExecutionState
	memoryUsage     atomic.Int64
	jobCount        atomic.Int64
}

func NewFSM() *FSM {
	return &FSM{
		jobs:            make(map[string]*Job),
		slots:           make(map[int64]*SlotData),
		executionStates: make(map[string]*JobExecutionState),
	}
}

// Apply applies a Raft log entry to the FSM
func (f *FSM) Apply(logEntry *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(logEntry.Data, &cmd); err != nil {
		logger.Debug("Failed to unmarshal command: %v", err)
		return fmt.Errorf("failed to unmarshal command: %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	switch cmd.Type {
	case CommandCreateJob:
		if cmd.Job == nil {
			return fmt.Errorf("job is required for create command")
		}
		f.jobs[cmd.Job.ID] = cmd.Job
		f.executionStates[cmd.Job.ID] = &JobExecutionState{
			JobID:     cmd.Job.ID,
			Status:    StatusPending,
			CreatedAt: cmd.Job.CreatedAt,
			Attempts:  []ExecutionAttempt{},
		}
		logger.Debug("Created job %s in FSM", cmd.Job.ID)
		return cmd.Job
	case CommandDeleteJob:
		if cmd.ID == "" {
			return fmt.Errorf("job ID is required for delete command")
		}
		delete(f.jobs, cmd.ID)
		delete(f.executionStates, cmd.ID)
		logger.Debug("Deleted job %s from FSM", cmd.ID)
		return nil
	case CommandCreateSlot:
		if cmd.Slot == nil {
			return fmt.Errorf("slot is required for create slot command")
		}
		f.slots[cmd.Slot.Key] = cmd.Slot
		logger.Debug("Created slot %d in FSM", cmd.Slot.Key)
		return cmd.Slot
	case CommandDeleteSlot:
		if cmd.ID == "" {
			return fmt.Errorf("slot key is required for delete slot command")
		}
		var key int64
		if _, err := fmt.Sscanf(cmd.ID, "%d", &key); err != nil {
			return fmt.Errorf("invalid slot key: %s", cmd.ID)
		}
		delete(f.slots, key)
		logger.Debug("Deleted slot %d from FSM", key)
		return nil
	case CommandUpdateJobStatus:
		if cmd.StatusCommand == nil {
			return fmt.Errorf("status command is required for update job status")
		}
		return f.applyStatusUpdate(cmd.StatusCommand)
	case CommandRecordAttempt:
		if cmd.StatusCommand == nil {
			return fmt.Errorf("status command is required for record attempt")
		}
		return f.applyRecordAttempt(cmd.StatusCommand)
	case CommandPruneAttempts:
		if cmd.StatusCommand == nil {
			return fmt.Errorf("status command is required for prune attempts")
		}
		return f.applyPruneAttempts(cmd.StatusCommand, cmd.Attempts)
	case CommandUpdateMemoryUsage:
		return f.applyMemoryUpdate(cmd.MemoryDelta)
	case CommandUpdateJobCount:
		return f.applyJobCountUpdate(cmd.JobCountDelta)
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// Snapshot creates a snapshot of the FSM state.
// Deep-copies all data to prevent races with Apply() during Persist().
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	jobs := make(map[string]*Job, len(f.jobs))
	for k, v := range f.jobs {
		jobs[k] = v.Clone()
	}

	slots := make(map[int64]*SlotData, len(f.slots))
	for k, v := range f.slots {
		slots[k] = v.Clone()
	}

	executionStates := make(map[string]*JobExecutionState, len(f.executionStates))
	for k, v := range f.executionStates {
		executionStates[k] = v.Clone()
	}

	return &Snapshot{
		jobs:            jobs,
		slots:           slots,
		executionStates: executionStates,
		memoryUsage:     f.memoryUsage.Load(),
		jobCount:        f.jobCount.Load(),
	}, nil
}

// Restore restores the FSM state from a snapshot
func (f *FSM) Restore(reader io.ReadCloser) error {
	defer reader.Close()

	var snapshot struct {
		Jobs            map[string]*Job               `json:"jobs"`
		Slots           map[int64]*SlotData           `json:"slots"`
		ExecutionStates map[string]*JobExecutionState `json:"execution_states"`
		MemoryUsage     int64                         `json:"memory_usage"`
		JobCount        int64                         `json:"job_count"`
	}

	if err := json.NewDecoder(reader).Decode(&snapshot); err != nil {
		return fmt.Errorf("failed to decode snapshot: %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.jobs = snapshot.Jobs
	if f.jobs == nil {
		f.jobs = make(map[string]*Job)
	}

	f.slots = snapshot.Slots
	if f.slots == nil {
		f.slots = make(map[int64]*SlotData)
	}

	f.executionStates = snapshot.ExecutionStates
	if f.executionStates == nil {
		f.executionStates = make(map[string]*JobExecutionState)
	}

	f.memoryUsage.Store(snapshot.MemoryUsage)
	f.jobCount.Store(snapshot.JobCount)

	logger.Debug("Restored %d jobs, %d slots, %d execution states, %d bytes memory, %d job count from snapshot",
		len(f.jobs), len(f.slots), len(f.executionStates), snapshot.MemoryUsage, snapshot.JobCount)
	return nil
}

// GetJob returns a deep copy of a job by ID
func (f *FSM) GetJob(id string) (*Job, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	job, exists := f.jobs[id]
	if !exists {
		return nil, false
	}
	return job.Clone(), true
}

// GetAllJobs returns deep copies of all jobs
func (f *FSM) GetAllJobs() map[string]*Job {
	f.mu.RLock()
	defer f.mu.RUnlock()

	jobs := make(map[string]*Job, len(f.jobs))
	for k, v := range f.jobs {
		jobs[k] = v.Clone()
	}
	return jobs
}

// GetSlot returns a deep copy of a slot by key
func (f *FSM) GetSlot(key int64) (*SlotData, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	slot, exists := f.slots[key]
	if !exists {
		return nil, false
	}
	return slot.Clone(), true
}

// GetAllSlots returns deep copies of all slots
func (f *FSM) GetAllSlots() map[int64]*SlotData {
	f.mu.RLock()
	defer f.mu.RUnlock()

	slots := make(map[int64]*SlotData, len(f.slots))
	for k, v := range f.slots {
		slots[k] = v.Clone()
	}
	return slots
}

// GetExecutionState returns a deep copy of the execution state for a job
func (f *FSM) GetExecutionState(jobID string) (*JobExecutionState, bool) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	state, exists := f.executionStates[jobID]
	if !exists {
		return nil, false
	}
	return state.Clone(), true
}

// GetAllExecutionStates returns deep copies of all execution states
func (f *FSM) GetAllExecutionStates() map[string]*JobExecutionState {
	f.mu.RLock()
	defer f.mu.RUnlock()

	states := make(map[string]*JobExecutionState, len(f.executionStates))
	for k, v := range f.executionStates {
		states[k] = v.Clone()
	}
	return states
}

// GetMemoryUsage returns the current memory usage in bytes (lock-free via atomic)
func (f *FSM) GetMemoryUsage() int64 {
	return f.memoryUsage.Load()
}

// GetJobCount returns the current job count (lock-free via atomic)
func (f *FSM) GetJobCount() int64 {
	return f.jobCount.Load()
}

// applyStatusUpdate applies a status update command
func (f *FSM) applyStatusUpdate(cmd *StatusCommand) error {
	state, exists := f.executionStates[cmd.JobID]
	if !exists {
		state = &JobExecutionState{
			JobID:     cmd.JobID,
			Status:    StatusPending,
			CreatedAt: cmd.Timestamp,
			Attempts:  []ExecutionAttempt{},
		}
		f.executionStates[cmd.JobID] = state
	}

	if err := validateStateTransition(state.Status, cmd.Status); err != nil {
		return err
	}

	state.Status = cmd.Status

	switch cmd.Status {
	case StatusInProgress:
		if state.FirstAttemptAt == 0 {
			state.FirstAttemptAt = cmd.Timestamp
		}
		state.LastAttemptAt = cmd.Timestamp
		state.ExecutingNodeID = cmd.NodeID
		state.AttemptCount++
	case StatusCompleted, StatusFailed:
		state.CompletedAt = cmd.Timestamp
		state.ExecutingNodeID = ""
	case StatusCancelled:
		state.CompletedAt = cmd.Timestamp
		state.CancelledAt = cmd.Timestamp
		state.CancellationReason = cmd.CancellationReason
		state.ExecutingNodeID = ""
	case StatusTimeout:
		state.ExecutingNodeID = ""
	}

	logger.Debug("Updated job %s status to %s", cmd.JobID, cmd.Status)
	return nil
}

// applyRecordAttempt records an execution attempt
func (f *FSM) applyRecordAttempt(cmd *StatusCommand) error {
	state, exists := f.executionStates[cmd.JobID]
	if !exists {
		return fmt.Errorf("execution state not found for job %s", cmd.JobID)
	}

	if cmd.Attempt == nil {
		return fmt.Errorf("attempt is required for record attempt command")
	}

	state.Attempts = append(state.Attempts, *cmd.Attempt)

	logger.Debug("Recorded attempt %d for job %s", cmd.Attempt.AttemptNum, cmd.JobID)
	return nil
}

// applyPruneAttempts replaces the attempts list with the pruned list
func (f *FSM) applyPruneAttempts(cmd *StatusCommand, keptAttempts []ExecutionAttempt) error {
	state, exists := f.executionStates[cmd.JobID]
	if !exists {
		return fmt.Errorf("execution state not found for job %s", cmd.JobID)
	}

	oldCount := len(state.Attempts)
	state.Attempts = keptAttempts
	newCount := len(state.Attempts)

	logger.Debug("Pruned %d attempts for job %s (kept %d)", oldCount-newCount, cmd.JobID, newCount)
	return nil
}

// applyMemoryUpdate updates the memory usage by the given delta
func (f *FSM) applyMemoryUpdate(delta int64) interface{} {
	newVal := f.memoryUsage.Add(delta)
	if newVal < 0 {
		f.memoryUsage.Store(0)
	}

	logger.Debug("Updated memory usage by %d bytes, total: %d bytes", delta, f.memoryUsage.Load())
	return f.memoryUsage.Load()
}

// applyJobCountUpdate updates the job count by the given delta
func (f *FSM) applyJobCountUpdate(delta int64) interface{} {
	newVal := f.jobCount.Add(delta)
	if newVal < 0 {
		f.jobCount.Store(0)
	}

	logger.Debug("Updated job count by %d, total: %d jobs", delta, f.jobCount.Load())
	return f.jobCount.Load()
}

// validateStateTransition validates if a state transition is allowed
func validateStateTransition(from, to JobStatus) error {
	allowed, exists := validTransitions[from]
	if !exists {
		return fmt.Errorf("unknown status: %s", from)
	}

	for _, valid := range allowed {
		if valid == to {
			return nil
		}
	}

	return fmt.Errorf("invalid state transition from %s to %s", from, to)
}

// Snapshot implements raft.FSMSnapshot
type Snapshot struct {
	jobs            map[string]*Job
	slots           map[int64]*SlotData
	executionStates map[string]*JobExecutionState
	memoryUsage     int64
	jobCount        int64
}

// Persist saves the snapshot to the given sink
func (s *Snapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		encoder := json.NewEncoder(sink)
		data := struct {
			Jobs            map[string]*Job               `json:"jobs"`
			Slots           map[int64]*SlotData           `json:"slots"`
			ExecutionStates map[string]*JobExecutionState `json:"execution_states"`
			MemoryUsage     int64                         `json:"memory_usage"`
			JobCount        int64                         `json:"job_count"`
		}{
			Jobs:            s.jobs,
			Slots:           s.slots,
			ExecutionStates: s.executionStates,
			MemoryUsage:     s.memoryUsage,
			JobCount:        s.jobCount,
		}
		return encoder.Encode(data)
	}()

	if err != nil {
		if err := sink.Cancel(); err != nil {
			logger.Debug("Failed to cancel sink: %v", err)
		}
		return err
	}

	return sink.Close()
}

// Release is called when the snapshot is no longer needed
func (s *Snapshot) Release() {}
