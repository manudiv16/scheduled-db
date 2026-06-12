package store

import (
	"scheduled-db/internal/store/types"
)

// Type aliases — these allow existing code to keep using store.Job, store.JobStatus, etc.
// while the actual definitions live in internal/store/types/.
// These aliases will be removed in a follow-up cleanup after all callers have been updated
// to import the types package directly.

type (
	Job               = types.Job
	JobType           = types.JobType
	SlotData          = types.SlotData
	JobStatus         = types.JobStatus
	ExecutionAttempt  = types.ExecutionAttempt
	JobExecutionState = types.JobExecutionState
	CreateJobRequest  = types.CreateJobRequest
	CommandType       = types.CommandType
	Command           = types.Command
	StatusCommand     = types.StatusCommand
	WebhookPayload    = types.WebhookPayload
)

// Constants
const (
	JobUnico                 = types.JobUnico
	JobRecurrente            = types.JobRecurrente
	StatusPending            = types.StatusPending
	StatusInProgress         = types.StatusInProgress
	StatusCompleted          = types.StatusCompleted
	StatusFailed             = types.StatusFailed
	StatusCancelled          = types.StatusCancelled
	StatusTimeout            = types.StatusTimeout
	CommandCreateJob         = types.CommandCreateJob
	CommandDeleteJob         = types.CommandDeleteJob
	CommandCreateSlot        = types.CommandCreateSlot
	CommandDeleteSlot        = types.CommandDeleteSlot
	CommandArchiveSlot       = types.CommandArchiveSlot
	CommandUnarchiveSlot     = types.CommandUnarchiveSlot
	CommandUpdateJobStatus   = types.CommandUpdateJobStatus
	CommandRecordAttempt     = types.CommandRecordAttempt
	CommandPruneAttempts     = types.CommandPruneAttempts
	CommandUpdateMemoryUsage = types.CommandUpdateMemoryUsage
	CommandUpdateJobCount    = types.CommandUpdateJobCount
)

// Wrapper functions for standalone functions moved to the types package.
// These are used by tests and by tests only; they exist to avoid breaking compilation
// during the migration and will be removed alongside the type aliases.

func ParseTimestamp(ts string) (int64, error) {
	return types.ParseTimestamp(ts)
}

func JobFromBytes(data []byte) (*Job, error) {
	return types.JobFromBytes(data)
}
