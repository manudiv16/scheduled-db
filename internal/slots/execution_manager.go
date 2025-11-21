package slots

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/metrics"
	"scheduled-db/internal/store"
)

// ExecutionManager coordinates job execution with status tracking
type ExecutionManager struct {
	statusTracker    *store.StatusTracker
	store            *store.Store
	nodeID           string
	executionTimeout time.Duration
	maxAttempts      int
	cancelFuncs      map[string]context.CancelFunc
	cancelMu         sync.RWMutex
}

// NewExecutionManager creates a new ExecutionManager
func NewExecutionManager(statusTracker *store.StatusTracker, st *store.Store, nodeID string, executionTimeout time.Duration, maxAttempts int) *ExecutionManager {
	em := &ExecutionManager{
		statusTracker:    statusTracker,
		store:            st,
		nodeID:           nodeID,
		executionTimeout: executionTimeout,
		maxAttempts:      maxAttempts,
		cancelFuncs:      make(map[string]context.CancelFunc),
	}

	// Set up status change callback for metrics
	statusTracker.SetStatusChangeCallback(func(oldStatus, newStatus store.JobStatus) {
		ctx := context.Background()
		if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
			// Decrement old status, increment new status
			globalMetrics.UpdateJobsByStatus(ctx, string(oldStatus), -1)
			globalMetrics.UpdateJobsByStatus(ctx, string(newStatus), 1)
		}
	})

	return em
}

// Execute executes a job with status tracking and idempotency
func (em *ExecutionManager) Execute(job *store.Job) error {
	// Check if job can be executed (idempotency)
	canExecute, err := em.statusTracker.CanExecute(job.ID)
	if err != nil {
		return fmt.Errorf("failed to check execution status: %w", err)
	}

	if !canExecute {
		logger.Info("job %s already executed, skipping", job.ID)
		return nil
	}

	// Get current execution state to determine attempt number
	state, err := em.statusTracker.GetStatus(job.ID)
	attemptNum := 1
	if err == nil && state != nil {
		attemptNum = state.AttemptCount + 1
	}

	// Check if we've exceeded max attempts
	if attemptNum > em.maxAttempts {
		logger.JobError(job.ID, "exceeded max attempts (%d), not executing", em.maxAttempts)
		return fmt.Errorf("exceeded max attempts")
	}

	// Mark as in progress
	if err := em.statusTracker.MarkInProgress(job.ID, em.nodeID); err != nil {
		return fmt.Errorf("failed to mark job in progress: %w", err)
	}

	// Execute webhook with timeout and cancellation support
	ctx, cancel := context.WithTimeout(context.Background(), em.executionTimeout)
	defer cancel()

	// Store cancel function for this job
	em.cancelMu.Lock()
	em.cancelFuncs[job.ID] = cancel
	em.cancelMu.Unlock()

	// Clean up cancel function when done
	defer func() {
		em.cancelMu.Lock()
		delete(em.cancelFuncs, job.ID)
		em.cancelMu.Unlock()
	}()

	attempt := &store.ExecutionAttempt{
		AttemptNum: attemptNum,
		StartTime:  time.Now().Unix(),
		NodeID:     em.nodeID,
	}

	// Execute the webhook
	httpStatus, responseTime, err := em.executeWebhook(ctx, job)
	attempt.EndTime = time.Now().Unix()
	attempt.ResponseTime = responseTime

	if err != nil {
		// Check if job was cancelled during execution
		state, statusErr := em.statusTracker.GetStatus(job.ID)
		if statusErr == nil && state.Status == store.StatusCancelled {
			// Job was cancelled, don't mark as failed
			logger.Info("job %s was cancelled during execution", job.ID)
			return nil
		}

		// Execution failed
		attempt.Status = store.StatusFailed
		attempt.ErrorMessage = err.Error()
		attempt.ErrorType = em.classifyError(err)
		attempt.HTTPStatus = httpStatus

		// Record failure metrics
		ctx := context.Background()
		if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
			globalMetrics.IncrementExecutionFailures(ctx, attempt.ErrorType)
		}

		// Record retry count if this is a retry
		if attemptNum > 1 {
			if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
				globalMetrics.RecordRetryCount(ctx, int64(attemptNum-1))
			}
		}

		if markErr := em.statusTracker.MarkFailed(job.ID, attempt); markErr != nil {
			logger.JobError(job.ID, "failed to mark job as failed: %v", markErr)
		}

		return err
	}

	// Check if job was cancelled after execution completed
	state, statusErr := em.statusTracker.GetStatus(job.ID)
	if statusErr == nil && state.Status == store.StatusCancelled {
		// Job was cancelled, record the attempt but keep cancelled status
		if recordErr := em.statusTracker.RecordAttempt(job.ID, attempt); recordErr != nil {
			logger.JobError(job.ID, "failed to record attempt for cancelled job: %v", recordErr)
		}
		logger.Info("job %s was cancelled, execution completed but status remains cancelled", job.ID)
		return nil
	}

	// Execution succeeded
	attempt.Status = store.StatusCompleted
	attempt.HTTPStatus = httpStatus

	// Record retry count if this is a retry
	if attemptNum > 1 {
		ctx := context.Background()
		if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
			globalMetrics.RecordRetryCount(ctx, int64(attemptNum-1))
		}
	}

	if err := em.statusTracker.MarkCompleted(job.ID, attempt); err != nil {
		logger.JobError(job.ID, "failed to mark job as completed: %v", err)
		return err
	}

	return nil
}

// executeWebhook executes the webhook and returns HTTP status, response time, and error
func (em *ExecutionManager) executeWebhook(ctx context.Context, job *store.Job) (int, int64, error) {
	if job.WebhookURL == "" {
		return 0, 0, nil
	}

	startTime := time.Now()

	// Prepare payload
	payload := map[string]interface{}{
		"job_id":    job.ID,
		"type":      job.Type,
		"timestamp": time.Now().Unix(),
		"data":      job.Payload,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return 0, time.Since(startTime).Milliseconds(), fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", job.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return 0, time.Since(startTime).Milliseconds(), fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	client := &http.Client{Timeout: em.executionTimeout}
	resp, err := client.Do(req)
	responseTime := time.Since(startTime).Milliseconds()

	if err != nil {
		return 0, responseTime, err
	}
	defer resp.Body.Close()

	// Check HTTP status
	if resp.StatusCode >= 400 {
		return resp.StatusCode, responseTime, fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return resp.StatusCode, responseTime, nil
}

// CancelJob attempts to cancel an in-progress job by cancelling its context
func (em *ExecutionManager) CancelJob(jobID string) bool {
	em.cancelMu.RLock()
	cancel, exists := em.cancelFuncs[jobID]
	em.cancelMu.RUnlock()

	if exists {
		cancel()
		logger.Info("cancelled in-progress job %s", jobID)
		return true
	}

	return false
}

// classifyError classifies an error into a category for metrics and debugging
func (em *ExecutionManager) classifyError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// Check for context errors first
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}

	// Check for network errors
	if strings.Contains(errStr, "connection refused") {
		return "connection_error"
	}
	if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "dns") {
		return "dns_error"
	}
	if strings.Contains(errStr, "timeout") {
		return "timeout"
	}

	// Check for HTTP errors
	if strings.Contains(errStr, "status") && (strings.Contains(errStr, "4") || strings.Contains(errStr, "5")) {
		return "http_error"
	}

	return "unknown"
}
