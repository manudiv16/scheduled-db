package store

import (
	"encoding/json"
	"fmt"
	"time"

	"scheduled-db/internal/logger"
)

// StatusChangeCallback is called when a job status changes
type StatusChangeCallback func(oldStatus, newStatus JobStatus)

// StatusTracker manages job execution status
type StatusTracker struct {
	store                *Store
	statusChangeCallback StatusChangeCallback
	pruningStop          chan struct{}
}

// NewStatusTracker creates a new StatusTracker
func NewStatusTracker(store *Store) *StatusTracker {
	return &StatusTracker{
		store:       store,
		pruningStop: make(chan struct{}),
	}
}

// SetStatusChangeCallback sets the callback for status changes
func (st *StatusTracker) SetStatusChangeCallback(callback StatusChangeCallback) {
	st.statusChangeCallback = callback
}

// GetStatus returns the current status of a job
func (st *StatusTracker) GetStatus(jobID string) (*JobExecutionState, error) {
	state, exists := st.store.fsm.GetExecutionState(jobID)
	if !exists {
		return nil, fmt.Errorf("execution state not found for job %s", jobID)
	}
	return state, nil
}

// CanExecute checks if a job can be executed (idempotency check)
func (st *StatusTracker) CanExecute(jobID string) (bool, error) {
	state, exists := st.store.fsm.GetExecutionState(jobID)
	if !exists {
		// If no state exists, job can be executed
		return true, nil
	}

	// Check if status allows execution
	switch state.Status {
	case StatusPending:
		return true, nil
	case StatusTimeout:
		return true, nil
	case StatusFailed:
		return true, nil
	case StatusCompleted:
		return false, nil
	case StatusCancelled:
		return false, nil
	case StatusInProgress:
		// Check if in-progress timeout has been exceeded
		// This will be handled by CheckTimeouts
		return false, nil
	default:
		return false, fmt.Errorf("unknown status: %s", state.Status)
	}
}

// MarkInProgress marks a job as in progress
func (st *StatusTracker) MarkInProgress(jobID string, nodeID string) error {
	if !st.store.IsLeader() {
		return fmt.Errorf("not leader")
	}

	// Get previous status for callback
	prevState, _ := st.GetStatus(jobID)
	prevStatus := StatusPending
	if prevState != nil {
		prevStatus = prevState.Status
	}

	statusCmd := &StatusCommand{
		JobID:     jobID,
		Status:    StatusInProgress,
		NodeID:    nodeID,
		Timestamp: time.Now().Unix(),
	}

	if err := st.applyStatusCommand(CommandUpdateJobStatus, statusCmd); err != nil {
		return err
	}

	// Notify callback
	if st.statusChangeCallback != nil {
		st.statusChangeCallback(prevStatus, StatusInProgress)
	}

	return nil
}

// MarkCompleted marks a job as completed
func (st *StatusTracker) MarkCompleted(jobID string, attempt *ExecutionAttempt) error {
	if !st.store.IsLeader() {
		return fmt.Errorf("not leader")
	}

	// Get previous status for callback
	prevState, _ := st.GetStatus(jobID)
	prevStatus := StatusInProgress
	if prevState != nil {
		prevStatus = prevState.Status
	}

	// First record the attempt
	if err := st.recordAttempt(jobID, attempt); err != nil {
		return fmt.Errorf("failed to record attempt: %w", err)
	}

	// Then update status
	statusCmd := &StatusCommand{
		JobID:     jobID,
		Status:    StatusCompleted,
		Timestamp: time.Now().Unix(),
	}

	if err := st.applyStatusCommand(CommandUpdateJobStatus, statusCmd); err != nil {
		return err
	}

	// Notify callback
	if st.statusChangeCallback != nil {
		st.statusChangeCallback(prevStatus, StatusCompleted)
	}

	return nil
}

// MarkFailed marks a job as failed
func (st *StatusTracker) MarkFailed(jobID string, attempt *ExecutionAttempt) error {
	if !st.store.IsLeader() {
		return fmt.Errorf("not leader")
	}

	// Get previous status for callback
	prevState, _ := st.GetStatus(jobID)
	prevStatus := StatusInProgress
	if prevState != nil {
		prevStatus = prevState.Status
	}

	// First record the attempt
	if err := st.recordAttempt(jobID, attempt); err != nil {
		return fmt.Errorf("failed to record attempt: %w", err)
	}

	// Then update status
	statusCmd := &StatusCommand{
		JobID:     jobID,
		Status:    StatusFailed,
		Timestamp: time.Now().Unix(),
	}

	if err := st.applyStatusCommand(CommandUpdateJobStatus, statusCmd); err != nil {
		return err
	}

	// Notify callback
	if st.statusChangeCallback != nil {
		st.statusChangeCallback(prevStatus, StatusFailed)
	}

	return nil
}

// MarkCancelled marks a job as cancelled
func (st *StatusTracker) MarkCancelled(jobID string, reason string) error {
	if !st.store.IsLeader() {
		return fmt.Errorf("not leader")
	}

	// Get previous status for callback
	prevState, _ := st.GetStatus(jobID)
	prevStatus := StatusPending
	if prevState != nil {
		prevStatus = prevState.Status
	}

	statusCmd := &StatusCommand{
		JobID:              jobID,
		Status:             StatusCancelled,
		Timestamp:          time.Now().Unix(),
		CancellationReason: reason,
	}

	if err := st.applyStatusCommand(CommandUpdateJobStatus, statusCmd); err != nil {
		return err
	}

	// Notify callback
	if st.statusChangeCallback != nil {
		st.statusChangeCallback(prevStatus, StatusCancelled)
	}

	logger.Info("job %s cancelled: %s", jobID, reason)
	return nil
}

// CheckTimeouts checks for timed-out jobs and marks them
func (st *StatusTracker) CheckTimeouts(timeout time.Duration) ([]string, error) {
	if !st.store.IsLeader() {
		return nil, fmt.Errorf("not leader")
	}

	allStates := st.store.fsm.GetAllExecutionStates()
	now := time.Now().Unix()
	var timedOutJobs []string

	for jobID, state := range allStates {
		if state.Status == StatusInProgress {
			elapsed := now - state.LastAttemptAt
			if elapsed > int64(timeout.Seconds()) {
				// Mark as timeout
				statusCmd := &StatusCommand{
					JobID:     jobID,
					Status:    StatusTimeout,
					Timestamp: now,
				}

				if err := st.applyStatusCommand(CommandUpdateJobStatus, statusCmd); err != nil {
					logger.Error("failed to mark job %s as timeout: %v", jobID, err)
					continue
				}

				// Notify callback
				if st.statusChangeCallback != nil {
					st.statusChangeCallback(StatusInProgress, StatusTimeout)
				}

				timedOutJobs = append(timedOutJobs, jobID)
				logger.Warn("job %s timed out after %d seconds", jobID, elapsed)
			}
		}
	}

	return timedOutJobs, nil
}

// ListByStatus returns all jobs with a given status
func (st *StatusTracker) ListByStatus(status JobStatus) ([]*JobExecutionState, error) {
	allStates := st.store.fsm.GetAllExecutionStates()
	var result []*JobExecutionState

	for _, state := range allStates {
		if state.Status == status {
			result = append(result, state)
		}
	}

	return result, nil
}

// GetExecutionHistory returns all execution attempts for a job
func (st *StatusTracker) GetExecutionHistory(jobID string) ([]ExecutionAttempt, error) {
	state, exists := st.store.fsm.GetExecutionState(jobID)
	if !exists {
		return nil, fmt.Errorf("execution state not found for job %s", jobID)
	}

	return state.Attempts, nil
}

// RecordAttempt records an execution attempt (public method for external use)
func (st *StatusTracker) RecordAttempt(jobID string, attempt *ExecutionAttempt) error {
	return st.recordAttempt(jobID, attempt)
}

// recordAttempt records an execution attempt
func (st *StatusTracker) recordAttempt(jobID string, attempt *ExecutionAttempt) error {
	statusCmd := &StatusCommand{
		JobID:     jobID,
		Attempt:   attempt,
		Timestamp: time.Now().Unix(),
	}

	return st.applyStatusCommand(CommandRecordAttempt, statusCmd)
}

// applyStatusCommand applies a status command through Raft
func (st *StatusTracker) applyStatusCommand(cmdType CommandType, statusCmd *StatusCommand) error {
	command := Command{
		Type:          cmdType,
		StatusCommand: statusCmd,
	}

	data, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	future := st.store.raft.Apply(data, getApplyTimeout())
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply command: %w", err)
	}

	// Check if the apply returned an error
	if result := future.Response(); result != nil {
		if err, ok := result.(error); ok {
			return err
		}
	}

	return nil
}

// PruneOldAttempts removes execution attempts older than the retention period
func (st *StatusTracker) PruneOldAttempts(retention time.Duration) error {
	if !st.store.IsLeader() {
		return fmt.Errorf("not leader")
	}

	allStates := st.store.fsm.GetAllExecutionStates()
	cutoffTime := time.Now().Add(-retention).Unix()

	for jobID, state := range allStates {
		if len(state.Attempts) == 0 {
			continue
		}

		// Filter out attempts older than retention period
		var keptAttempts []ExecutionAttempt
		var prunedCount int

		for _, attempt := range state.Attempts {
			if attempt.StartTime >= cutoffTime {
				keptAttempts = append(keptAttempts, attempt)
			} else {
				prunedCount++
			}
		}

		// Only apply pruning if we actually removed attempts
		if prunedCount > 0 {
			statusCmd := &StatusCommand{
				JobID:     jobID,
				Timestamp: time.Now().Unix(),
			}

			// Apply pruning through Raft using a special command
			if err := st.applyPruneCommand(statusCmd, keptAttempts); err != nil {
				logger.Error("failed to prune attempts for job %s: %v", jobID, err)
				continue
			}

			logger.Debug("pruned %d old attempts for job %s", prunedCount, jobID)
		}
	}

	return nil
}

// applyPruneCommand applies a prune command through Raft
func (st *StatusTracker) applyPruneCommand(statusCmd *StatusCommand, keptAttempts []ExecutionAttempt) error {
	command := Command{
		Type:          CommandPruneAttempts,
		StatusCommand: statusCmd,
		Attempts:      keptAttempts,
	}

	data, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	future := st.store.raft.Apply(data, getApplyTimeout())
	if err := future.Error(); err != nil {
		return fmt.Errorf("failed to apply command: %w", err)
	}

	// Check if the apply returned an error
	if result := future.Response(); result != nil {
		if err, ok := result.(error); ok {
			return err
		}
	}

	return nil
}

// StartPruning starts a background goroutine that periodically prunes old execution attempts
func (st *StatusTracker) StartPruning(retention time.Duration) {
	go func() {
		// Run pruning every hour
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		logger.Info("started execution history pruning with retention period: %v", retention)

		for {
			select {
			case <-ticker.C:
				// Only prune if we're the leader
				if st.store.IsLeader() {
					logger.Debug("running execution history pruning")
					if err := st.PruneOldAttempts(retention); err != nil {
						logger.Error("failed to prune old attempts: %v", err)
					}
				}
			case <-st.pruningStop:
				logger.Info("stopping execution history pruning")
				return
			}
		}
	}()
}

// StopPruning stops the pruning goroutine
func (st *StatusTracker) StopPruning() {
	close(st.pruningStop)
}

// ExecutionStats holds aggregated execution statistics
type ExecutionStats struct {
	Pending      int64   `json:"pending"`
	InProgress   int64   `json:"in_progress"`
	Completed    int64   `json:"completed"`
	Failed       int64   `json:"failed"`
	Cancelled    int64   `json:"cancelled"`
	Timeout      int64   `json:"timeout"`
	Total        int64   `json:"total"`
	LastExecuted int64   `json:"last_executed_at,omitempty"`
	FailureRate  float64 `json:"failure_rate"`
}

// GetExecutionStats calculates and returns execution statistics
func (st *StatusTracker) GetExecutionStats() (*ExecutionStats, error) {
	allStates := st.store.fsm.GetAllExecutionStates()

	stats := &ExecutionStats{
		Total: int64(len(allStates)),
	}

	for _, state := range allStates {
		switch state.Status {
		case StatusPending:
			stats.Pending++
		case StatusInProgress:
			stats.InProgress++
		case StatusCompleted:
			stats.Completed++
			if state.CompletedAt > stats.LastExecuted {
				stats.LastExecuted = state.CompletedAt
			}
		case StatusFailed:
			stats.Failed++
			if state.CompletedAt > stats.LastExecuted {
				stats.LastExecuted = state.CompletedAt
			}
		case StatusCancelled:
			stats.Cancelled++
		case StatusTimeout:
			stats.Timeout++
		}
	}

	// Calculate failure rate
	// We consider completed, failed, and timeout as "terminal" states that represent attempted executions
	finishedCount := stats.Completed + stats.Failed + stats.Timeout
	if finishedCount > 0 {
		// Failure rate = (Failed + Timeout) / (Completed + Failed + Timeout)
		stats.FailureRate = float64(stats.Failed+stats.Timeout) / float64(finishedCount)
	}

	return stats, nil
}
