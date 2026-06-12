package types

// JobStatus represents the execution state of a job.
type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusInProgress JobStatus = "in_progress"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusCancelled  JobStatus = "cancelled"
	StatusTimeout    JobStatus = "timeout"
)

// ExecutionAttempt records a single execution attempt.
type ExecutionAttempt struct {
	AttemptNum   int       `json:"attempt_num"`
	StartTime    int64     `json:"start_time"`
	EndTime      int64     `json:"end_time,omitempty"`
	NodeID       string    `json:"node_id"`
	Status       JobStatus `json:"status"`
	HTTPStatus   int       `json:"http_status,omitempty"`
	ResponseTime int64     `json:"response_time_ms,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
	ErrorType    string    `json:"error_type,omitempty"`
}

// JobExecutionState tracks the complete execution state of a job.
type JobExecutionState struct {
	JobID              string             `json:"job_id"`
	Status             JobStatus          `json:"status"`
	AttemptCount       int                `json:"attempt_count"`
	CreatedAt          int64              `json:"created_at"`
	FirstAttemptAt     int64              `json:"first_attempt_at,omitempty"`
	LastAttemptAt      int64              `json:"last_attempt_at,omitempty"`
	CompletedAt        int64              `json:"completed_at,omitempty"`
	CancelledAt        int64              `json:"cancelled_at,omitempty"`
	CancellationReason string             `json:"cancellation_reason,omitempty"`
	ExecutingNodeID    string             `json:"executing_node_id,omitempty"`
	Attempts           []ExecutionAttempt `json:"attempts"`
}

// Clone returns a deep copy of the JobExecutionState.
func (s *JobExecutionState) Clone() *JobExecutionState {
	if s == nil {
		return nil
	}
	clone := *s
	if s.Attempts != nil {
		clone.Attempts = make([]ExecutionAttempt, len(s.Attempts))
		copy(clone.Attempts, s.Attempts)
	}
	return &clone
}
