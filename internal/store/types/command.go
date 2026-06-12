package types

// CommandType represents the type of command to apply to the FSM.
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

// Command represents a command to apply to the FSM.
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

// StatusCommand represents a status update command.
type StatusCommand struct {
	JobID              string            `json:"job_id"`
	Status             JobStatus         `json:"status"`
	NodeID             string            `json:"node_id,omitempty"`
	Attempt            *ExecutionAttempt `json:"attempt,omitempty"`
	Timestamp          int64             `json:"timestamp"`
	CancellationReason string            `json:"cancellation_reason,omitempty"`
}
