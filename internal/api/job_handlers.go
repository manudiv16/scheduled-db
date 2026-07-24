package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/metrics"
	"scheduled-db/internal/slots"
	"scheduled-db/internal/store"
	"scheduled-db/internal/store/types"

	"github.com/gorilla/mux"
)

func (h *Handlers) CreateJob(w http.ResponseWriter, r *http.Request) {
	// If not leader, try to proxy to leader
	if !h.store.IsLeader() {
		h.proxyToLeader(w, r)
		return
	}

	var req types.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Error("failed to decode job request: %v", err)
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	job, err := req.ToJob()
	if err != nil {
		logger.Error("invalid job data: %v", err)
		h.writeError(w, http.StatusBadRequest, safeErrorMessage("Invalid job data", err, true))
		return
	}

	if err := job.Validate(); err != nil {
		logger.Error("job validation failed: %v", err)
		h.writeError(w, http.StatusBadRequest, safeErrorMessage("Job validation failed", err, true))
		return
	}

	// Check capacity limits
	if h.limitManager != nil {
		if err := h.limitManager.CheckCapacity(job); err != nil {
			// Check if it's a capacity error
			if capErr, ok := err.(*slots.CapacityError); ok {
				// Return 507 Insufficient Storage
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(capErr.HTTPStatus())
				if encodeErr := json.NewEncoder(w).Encode(map[string]interface{}{
					"error":     capErr.Error(),
					"type":      capErr.Type,
					"current":   capErr.Current,
					"limit":     capErr.Limit,
					"requested": capErr.Requested,
				}); encodeErr != nil {
					logger.Error("failed to encode capacity error response: %v", encodeErr)
				}

				// Log rejection
				logger.Warn("job rejected due to capacity limit: %v", err)

				// Update metrics for rejection
				if m := metrics.GetGlobalMetrics(); m != nil {
					m.IncrementJobRejections(context.Background(), capErr.Type)
				}
				return
			}

			// Other errors
			logger.Error("capacity check failed: %v", err)
			h.writeError(w, http.StatusInternalServerError, "Capacity check failed")
			return
		}
	}

	logger.Debug("about to create job in store: %s", job.ID)
	if err := h.store.CreateJob(job); err != nil {
		logger.Error("failed to create job in store: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to create job")
		return
	}
	logger.Debug("job created successfully in store: %s", job.ID)

	// Update capacity tracking
	if h.limitManager != nil {
		if err := h.limitManager.RecordJobAdded(job); err != nil {
			logger.Error("failed to record job addition in limit manager: %v", err)
		}
	}

	// Record job creation metrics using OpenTelemetry
	if metrics.GlobalJobInstrumentation != nil {
		metrics.GlobalJobInstrumentation.RecordJobCreated(context.Background(), job)
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		logger.Error("failed to encode job response: %v", err)
	}
	logger.Info("created job %s via API", job.ID)
}

func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	job, exists := h.store.GetJob(id)
	if !exists {
		h.writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(job); err != nil {
		logger.Error("failed to encode job response: %v", err)
	}
}

func (h *Handlers) DeleteJob(w http.ResponseWriter, r *http.Request) {
	// If not leader, try to proxy to leader
	if !h.store.IsLeader() {
		h.proxyToLeader(w, r)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	// Check if job exists and get it for metrics
	job, exists := h.store.GetJob(id)
	if !exists {
		h.writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	if err := h.store.DeleteJob(id); err != nil {
		logger.Error("failed to delete job: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to delete job")
		return
	}

	// Update capacity tracking
	if h.limitManager != nil {
		if err := h.limitManager.RecordJobRemoved(job); err != nil {
			logger.Error("failed to record job removal in limit manager: %v", err)
		}
	}

	// Record job deletion metrics using OpenTelemetry
	if metrics.GlobalJobInstrumentation != nil {
		metrics.GlobalJobInstrumentation.RecordJobDeleted(context.Background(), job)
	}

	w.WriteHeader(http.StatusNoContent)
	logger.Info("deleted job %s via API", id)
}

// GetJobStatus returns the execution status of a job
func (h *Handlers) GetJobStatus(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		// Record status query latency
		ctx := context.Background()
		if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
			globalMetrics.RecordStatusQueryLatency(ctx, duration, "get_job_status")
		}
	}()

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	// Check if job exists first
	_, exists := h.store.GetJob(id)
	if !exists {
		h.writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	// Get status from FSM (works on both leader and follower)
	statusTracker := store.NewStatusTracker(h.store)
	state, err := statusTracker.GetStatus(id)
	if err != nil {
		// If no execution state exists yet, return pending status
		state = &types.JobExecutionState{
			JobID:     id,
			Status:    types.StatusPending,
			CreatedAt: time.Now().Unix(),
			Attempts:  []types.ExecutionAttempt{},
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		logger.Error("failed to encode job status response: %v", err)
	}
}

// GetJobExecutions returns the execution history of a job
func (h *Handlers) GetJobExecutions(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		// Record status query latency
		ctx := context.Background()
		if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
			globalMetrics.RecordStatusQueryLatency(ctx, duration, "get_job_executions")
		}
	}()

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	// Check if job exists first
	_, exists := h.store.GetJob(id)
	if !exists {
		h.writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	// Get execution history from FSM (works on both leader and follower)
	statusTracker := store.NewStatusTracker(h.store)
	attempts, err := statusTracker.GetExecutionHistory(id)
	if err != nil {
		// If no execution state exists yet, return empty attempts
		attempts = []types.ExecutionAttempt{}
	}

	response := map[string]interface{}{
		"job_id":   id,
		"attempts": attempts,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to encode job executions response: %v", err)
	}
}

// CancelJob cancels a job
func (h *Handlers) CancelJob(w http.ResponseWriter, r *http.Request) {
	// If not leader, try to proxy to leader
	if !h.store.IsLeader() {
		h.proxyToLeader(w, r)
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Job ID is required")
		return
	}

	// Check if job exists first
	_, exists := h.store.GetJob(id)
	if !exists {
		h.writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	// Parse optional cancellation reason from request body
	var req struct {
		Reason string `json:"reason,omitempty"`
	}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// If body is empty or invalid JSON, just use empty reason
			req.Reason = ""
		}
	}

	// Check current status to see if job is in progress
	statusTracker := store.NewStatusTracker(h.store)
	state, err := statusTracker.GetStatus(id)
	wasInProgress := false
	if err == nil && state.Status == types.StatusInProgress {
		wasInProgress = true
		// Attempt to cancel in-progress execution
		if h.executionManager != nil {
			cancelled := h.executionManager.CancelJob(id)
			if cancelled {
				logger.Info("cancelled in-progress execution for job %s", id)
			}
		}
	}

	// Mark job as cancelled in Raft
	if err := statusTracker.MarkCancelled(id, req.Reason); err != nil {
		logger.Error("failed to cancel job: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to cancel job")
		return
	}

	// Get updated status
	state, err = statusTracker.GetStatus(id)
	if err != nil {
		logger.Error("failed to get job status: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to get job status")
		return
	}

	response := map[string]interface{}{
		"job_id":       id,
		"status":       state.Status,
		"cancelled_at": state.CancelledAt,
	}

	if wasInProgress {
		response["message"] = "Job was in progress, cancellation attempted"
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to encode cancel job response: %v", err)
	}
	logger.Info("cancelled job %s via API", id)
}

// ListJobsByStatus returns all jobs with a given status
func (h *Handlers) ListJobsByStatus(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		// Record status query latency
		ctx := context.Background()
		if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
			globalMetrics.RecordStatusQueryLatency(ctx, duration, "list_jobs_by_status")
		}
	}()

	statusParam := r.URL.Query().Get("status")

	if statusParam == "" {
		h.writeError(w, http.StatusBadRequest, "status query parameter is required")
		return
	}

	// Validate status parameter
	status := types.JobStatus(statusParam)
	validStatuses := []types.JobStatus{
		types.StatusPending,
		types.StatusInProgress,
		types.StatusCompleted,
		types.StatusFailed,
		types.StatusCancelled,
		types.StatusTimeout,
	}

	isValid := false
	for _, validStatus := range validStatuses {
		if status == validStatus {
			isValid = true
			break
		}
	}

	if !isValid {
		h.writeError(w, http.StatusBadRequest, "invalid status parameter")
		return
	}

	// Get jobs by status from FSM (works on both leader and follower)
	statusTracker := store.NewStatusTracker(h.store)
	states, err := statusTracker.ListByStatus(status)
	if err != nil {
		logger.Error("failed to list jobs by status: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to list jobs")
		return
	}

	// If no states found, return empty array
	if states == nil {
		states = []*types.JobExecutionState{}
	}

	response := map[string]interface{}{
		"jobs":  states,
		"total": len(states),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to encode jobs by status response: %v", err)
	}
}
