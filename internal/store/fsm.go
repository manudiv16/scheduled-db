package store

import (
	"fmt"
	"sync"
	"sync/atomic"

	"scheduled-db/internal/logger"
)

type CommandType string

const (
	CommandCreateJob         CommandType = "create_job"
	CommandDeleteJob         CommandType = "delete_job"
	CommandCreateSlot        CommandType = "create_slot"
	CommandDeleteSlot        CommandType = "delete_slot"
	CommandArchiveSlot       CommandType = "archive_slot"
	CommandUnarchiveSlot     CommandType = "unarchive_slot"
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
	ColdSlot      *SlotData          `json:"cold_slot,omitempty"`
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

type ColdSlotStore interface {
	PutColdSlot(key int64, data *SlotData) error
	GetColdSlot(key int64) (*SlotData, error)
	DeleteColdSlot(key int64) error
	Close() error
}

type FSM struct {
	mu              sync.RWMutex
	jobs            map[string]*Job
	slots           map[int64]*SlotData
	coldSlotMeta    map[int64]bool
	coldStore       ColdSlotStore
	executionStates map[string]*JobExecutionState
	memoryUsage     atomic.Int64
	jobCount        atomic.Int64
}

func NewFSM() *FSM {
	return &FSM{
		jobs:            make(map[string]*Job),
		slots:           make(map[int64]*SlotData),
		coldSlotMeta:    make(map[int64]bool),
		executionStates: make(map[string]*JobExecutionState),
	}
}

func NewFSMWithColdStore(coldStore ColdSlotStore) *FSM {
	return &FSM{
		jobs:            make(map[string]*Job),
		slots:           make(map[int64]*SlotData),
		coldSlotMeta:    make(map[int64]bool),
		coldStore:       coldStore,
		executionStates: make(map[string]*JobExecutionState),
	}
}

// ApplyCommand applies a command directly to the FSM without Raft.
// This enables non-consensus usage (simulator, WASM).
func (f *FSM) ApplyCommand(cmd *Command) interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.applyCommandInternal(cmd)
}

// applyCommandInternal processes a command. Caller must hold f.mu.
func (f *FSM) applyCommandInternal(cmd *Command) interface{} {
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
		delete(f.coldSlotMeta, key)
		if f.coldStore != nil {
			if err := f.coldStore.DeleteColdSlot(key); err != nil {
				logger.Debug("Failed to delete cold slot %d: %v", key, err)
			}
		}
		logger.Debug("Deleted slot %d from FSM", key)
		return nil
	case CommandArchiveSlot:
		if cmd.ColdSlot == nil {
			return fmt.Errorf("cold slot data is required for archive slot command")
		}
		if f.coldStore == nil {
			return fmt.Errorf("cold store not enabled, cannot archive slot")
		}
		key := cmd.ColdSlot.Key
		if err := f.coldStore.PutColdSlot(key, cmd.ColdSlot); err != nil {
			return fmt.Errorf("failed to archive slot %d to cold store: %v", key, err)
		}
		delete(f.slots, key)
		f.coldSlotMeta[key] = true
		logger.Debug("Archived slot %d to cold store", key)
		return nil
	case CommandUnarchiveSlot:
		if cmd.ID == "" {
			return fmt.Errorf("slot key is required for unarchive slot command")
		}
		if f.coldStore == nil {
			return fmt.Errorf("cold store not enabled, cannot unarchive slot")
		}
		var key int64
		if _, err := fmt.Sscanf(cmd.ID, "%d", &key); err != nil {
			return fmt.Errorf("invalid slot key: %s", cmd.ID)
		}
		slotData, err := f.coldStore.GetColdSlot(key)
		if err != nil {
			return fmt.Errorf("failed to unarchive slot %d from cold store: %v", key, err)
		}
		if slotData != nil {
			f.slots[key] = slotData
		}
		if err := f.coldStore.DeleteColdSlot(key); err != nil {
			logger.Debug("Failed to delete cold slot %d after unarchive: %v", key, err)
		}
		delete(f.coldSlotMeta, key)
		logger.Debug("Unarchived slot %d from cold store", key)
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
	slot, exists := f.slots[key]
	if exists {
		f.mu.RUnlock()
		return slot.Clone(), true
	}
	isCold := f.coldSlotMeta[key]
	f.mu.RUnlock()

	if isCold && f.coldStore != nil {
		coldSlot, err := f.coldStore.GetColdSlot(key)
		if err != nil || coldSlot == nil {
			return nil, false
		}
		return coldSlot.Clone(), true
	}

	return nil, false
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

func (f *FSM) IsSlotCold(key int64) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.coldSlotMeta[key]
}

func (f *FSM) GetColdSlotKeys() []int64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	keys := make([]int64, 0, len(f.coldSlotMeta))
	for k := range f.coldSlotMeta {
		keys = append(keys, k)
	}
	return keys
}

func (f *FSM) GetHotSlotCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.slots)
}

func (f *FSM) GetColdSlotCount() int {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return len(f.coldSlotMeta)
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

// CreateJob creates a job in the FSM (no Raft)
func (f *FSM) CreateJob(job *Job) error {
	result := f.ApplyCommand(&Command{Type: CommandCreateJob, Job: job})
	if err, ok := result.(error); ok {
		return err
	}
	return nil
}

// DeleteJob deletes a job from the FSM (no Raft)
func (f *FSM) DeleteJob(id string) error {
	result := f.ApplyCommand(&Command{Type: CommandDeleteJob, ID: id})
	if err, ok := result.(error); ok {
		return err
	}
	return nil
}

// CreateSlot creates a slot in the FSM (no Raft)
func (f *FSM) CreateSlot(slot *SlotData) error {
	result := f.ApplyCommand(&Command{Type: CommandCreateSlot, Slot: slot})
	if err, ok := result.(error); ok {
		return err
	}
	return nil
}

// DeleteSlot deletes a slot from the FSM (no Raft)
func (f *FSM) DeleteSlot(key int64) error {
	result := f.ApplyCommand(&Command{Type: CommandDeleteSlot, ID: fmt.Sprintf("%d", key)})
	if err, ok := result.(error); ok {
		return err
	}
	return nil
}

// UpdateJobStatus updates a job's execution status in the FSM (no Raft)
func (f *FSM) UpdateJobStatus(jobID string, status JobStatus, nodeID string, timestamp int64) error {
	result := f.ApplyCommand(&Command{
		Type: CommandUpdateJobStatus,
		StatusCommand: &StatusCommand{
			JobID:     jobID,
			Status:    status,
			NodeID:    nodeID,
			Timestamp: timestamp,
		},
	})
	if err, ok := result.(error); ok {
		return err
	}
	return nil
}

// GetSnapshot returns a snapshot of the entire FSM state
func (f *FSM) GetSnapshot() *Snapshot {
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

	coldSlotMeta := make(map[int64]bool, len(f.coldSlotMeta))
	for k, v := range f.coldSlotMeta {
		coldSlotMeta[k] = v
	}

	return &Snapshot{
		jobs:            jobs,
		slots:           slots,
		coldSlotMeta:    coldSlotMeta,
		executionStates: executionStates,
		memoryUsage:     f.memoryUsage.Load(),
		jobCount:        f.jobCount.Load(),
	}
}

// Snapshot holds a point-in-time copy of FSM state
type Snapshot struct {
	jobs            map[string]*Job
	slots           map[int64]*SlotData
	coldSlotMeta    map[int64]bool
	executionStates map[string]*JobExecutionState
	memoryUsage     int64
	jobCount        int64
}

func (s *Snapshot) Jobs() map[string]*Job                             { return s.jobs }
func (s *Snapshot) Slots() map[int64]*SlotData                        { return s.slots }
func (s *Snapshot) ExecutionStates() map[string]*JobExecutionState     { return s.executionStates }
func (s *Snapshot) MemoryUsage() int64                                { return s.memoryUsage }
func (s *Snapshot) JobCount() int64                                   { return s.jobCount }

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
