package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/metrics"
	"scheduled-db/internal/slots"
	"scheduled-db/internal/store"

	"github.com/gorilla/mux"
)

type Handlers struct {
	store                  *store.Store
	executionManager       *slots.ExecutionManager
	limitManager           *slots.LimitManager
	addressMap             map[string]string // Map Raft address to HTTP address
	healthFailureThreshold float64
}

type JobStats struct {
	Count     int64 `json:"count"`
	Limit     int64 `json:"limit"`
	Available int64 `json:"available"`
}

type HealthResponse struct {
	Status string             `json:"status"`
	Role   string             `json:"role"`
	Leader string             `json:"leader,omitempty"`
	NodeID string             `json:"node_id"`
	Memory *slots.MemoryUsage `json:"memory,omitempty"`
	Jobs   *JobStats          `json:"jobs,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type JoinRequest struct {
	NodeID  string `json:"node_id"`
	Address string `json:"address"`
}

type JoinResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ClusterDebugResponse struct {
	NodeID    string              `json:"node_id"`
	IsLeader  bool                `json:"is_leader"`
	Leader    string              `json:"leader"`
	RaftState string              `json:"raft_state"`
	Servers   []map[string]string `json:"servers"`
	JobCount  int                 `json:"job_count"`
}

func NewHandlers(store *store.Store, executionManager *slots.ExecutionManager, limitManager *slots.LimitManager, healthFailureThreshold float64) *Handlers {
	handlers := &Handlers{
		store:                  store,
		executionManager:       executionManager,
		limitManager:           limitManager,
		addressMap:             make(map[string]string),
		healthFailureThreshold: healthFailureThreshold,
	}

	// Build initial address mapping from environment variables
	handlers.buildAddressMapping()

	return handlers
}

// buildAddressMapping creates mapping from Raft addresses to HTTP addresses
func (h *Handlers) buildAddressMapping() {
	// Environment-based configuration
	// Format: CLUSTER_NODE_1=raft_host:raft_port,http_host:http_port
	// Example: CLUSTER_NODE_1=127.0.0.1:7000,127.0.0.1:8080

	for i := 1; i <= 10; i++ { // Support up to 10 nodes
		envKey := fmt.Sprintf("CLUSTER_NODE_%d", i)
		envValue := os.Getenv(envKey)

		if envValue == "" {
			continue
		}

		parts := strings.Split(envValue, ",")
		if len(parts) == 2 {
			raftAddr := strings.TrimSpace(parts[0])
			httpAddr := strings.TrimSpace(parts[1])

			// Ensure HTTP address has protocol
			if !strings.HasPrefix(httpAddr, "http://") && !strings.HasPrefix(httpAddr, "https://") {
				httpAddr = "http://" + httpAddr
			}
			logger.Debug("mapped Raft %s -> HTTP %s", raftAddr, httpAddr)
		}
	}

	// Fallback: create default mapping for common development setup
	if len(h.addressMap) == 0 {
		h.createDefaultMapping()
	}
}

// createDefaultMapping creates standard development mapping
func (h *Handlers) createDefaultMapping() {
	logger.Debug("no default port mappings - using environment variables only")
}

// getHTTPAddressForRaft converts Raft address to HTTP address
func (h *Handlers) getHTTPAddressForRaft(raftAddr string) (string, error) {
	// First try direct lookup
	if httpAddr, exists := h.addressMap[raftAddr]; exists {
		return httpAddr, nil
	}

	// Try with hostname conversion
	if strings.HasPrefix(raftAddr, "localhost:") {
		localAddr := strings.Replace(raftAddr, "localhost:", "127.0.0.1:", 1)
		if httpAddr, exists := h.addressMap[localAddr]; exists {
			return httpAddr, nil
		}
	}

	if strings.HasPrefix(raftAddr, "127.0.0.1:") {
		localhostAddr := strings.Replace(raftAddr, "127.0.0.1:", "localhost:", 1)
		if httpAddr, exists := h.addressMap[localhostAddr]; exists {
			return httpAddr, nil
		}
	}

	// Dynamic calculation fallback
	return h.calculateHTTPAddress(raftAddr)
}

// calculateHTTPAddress attempts to calculate HTTP address from Raft address
func (h *Handlers) calculateHTTPAddress(raftAddr string) (string, error) {
	parts := strings.Split(raftAddr, ":")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid Raft address format: %s", raftAddr)
	}

	host := parts[0]

	// For Kubernetes DNS names, convert to HTTP service address
	if strings.Contains(host, ".svc.cluster.local") {
		// Extract pod name from DNS (e.g., scheduled-db-2.scheduled-db.default.svc.cluster.local -> scheduled-db-2)
		podName := strings.Split(host, ".")[0]
		return fmt.Sprintf("http://%s:8080", podName), nil
	}

	// Convert localhost to 127.0.0.1 for consistency
	if host == "localhost" {
		host = "127.0.0.1"
	}

	// Use HTTP port from environment
	httpPort := 8080
	if portStr := os.Getenv("HTTP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			httpPort = port
		}
	}

	httpAddr := fmt.Sprintf("http://%s:%d", host, httpPort)
	return httpAddr, nil
}

func (h *Handlers) CreateJob(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		// Record HTTP metrics using OpenTelemetry
		ctx := context.Background()
		if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
			globalMetrics.IncrementHTTPRequests(ctx, r.Method, r.URL.Path, 200)
			globalMetrics.RecordHTTPRequestDuration(ctx, duration, r.Method, r.URL.Path)
		}
	}()

	// If not leader, try to proxy to leader
	if !h.store.IsLeader() {
		h.proxyToLeader(w, r)
		return
	}

	var req store.CreateJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	job, err := req.ToJob()
	if err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid job data: %v", err))
		return
	}

	if err := job.Validate(); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("Job validation failed: %v", err))
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
			h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Capacity check failed: %v", err))
			return
		}
	}

	logger.Debug("about to create job in store: %s", job.ID)
	if err := h.store.CreateJob(job); err != nil {
		logger.Debug("failed to create job in store: %v", err)
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create job: %v", err))
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
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete job: %v", err))
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

	logger.Info("deleted job %s via API", id)
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status: "ok",
		NodeID: h.store.GetNodeID(), // Get actual node ID from store
	}

	if h.store.IsLeader() {
		response.Role = "leader"
	} else {
		response.Role = "follower"
		response.Leader = h.store.GetLeader()
	}

	// Add capacity info if available
	if h.limitManager != nil {
		memUsage := h.limitManager.GetMemoryUsage()
		response.Memory = memUsage

		// Check for degraded status
		if memUsage.Utilization > 90.0 {
			response.Status = "degraded"
		}

		jobCount := h.limitManager.GetJobCount()
		jobLimit := h.limitManager.GetJobLimit()
		jobAvailable := jobLimit - jobCount
		if jobAvailable < 0 {
			jobAvailable = 0
		}

		response.Jobs = &JobStats{
			Count:     jobCount,
			Limit:     jobLimit,
			Available: jobAvailable,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to encode health response: %v", err)
	}
}

func (h *Handlers) ClusterDebug(w http.ResponseWriter, r *http.Request) {
	servers, err := h.store.GetClusterConfiguration()
	var serverList []map[string]string
	if err != nil {
		serverList = []map[string]string{{"error": err.Error()}}
	} else {
		serverList = make([]map[string]string, len(servers))
		for i, server := range servers {
			serverList[i] = map[string]string{
				"id":      string(server.ID),
				"address": string(server.Address),
			}
		}
	}

	jobs := h.store.GetAllJobs()

	response := ClusterDebugResponse{
		NodeID:    h.store.GetNodeID(), // Get actual node ID from store
		IsLeader:  h.store.IsLeader(),
		Leader:    h.store.GetLeader(),
		RaftState: h.store.GetRaftState(),
		Servers:   serverList,
		JobCount:  len(jobs),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to encode cluster debug response: %v", err)
	}
}

func (h *Handlers) proxyToLeader(w http.ResponseWriter, r *http.Request) {
	leader := h.store.GetLeader()
	if leader == "" {
		h.writeError(w, http.StatusServiceUnavailable, "No leader available")
		return
	}

	// Get HTTP address for the leader's Raft address
	httpAddr, err := h.getHTTPAddressForRaft(leader)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to resolve leader HTTP address: %v", err))
		return
	}

	// Create proxy request
	proxyReq, err := http.NewRequest(r.Method, httpAddr+r.URL.Path, r.Body)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create proxy request: %v", err))
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Execute proxy request
	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		h.writeError(w, http.StatusBadGateway, fmt.Sprintf("Failed to proxy request: %v", err))
		return
	}
	defer resp.Body.Close()

	// Copy response headers
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Copy status code and body
	w.WriteHeader(resp.StatusCode)

	// Copy response body
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, err := w.Write(buf[:n]); err != nil {
				logger.Error("failed to write proxy response: %v", err)
				break
			}
		}
		if err != nil {
			break
		}
	}
	logger.Debug("proxied %s %s to leader %s", r.Method, r.URL.Path, leader)
}

func (h *Handlers) JoinCluster(w http.ResponseWriter, r *http.Request) {
	// Only leader can accept join requests
	if !h.store.IsLeader() {
		h.writeError(w, http.StatusForbidden, "not leader, cannot accept join requests")
		return
	}

	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid JSON: %v", err))
		return
	}

	if req.NodeID == "" || req.Address == "" {
		h.writeError(w, http.StatusBadRequest, "node_id and address are required")
		return
	}

	// Add peer to Raft cluster
	if err := h.store.AddPeer(req.NodeID, req.Address); err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to add peer: %v", err))
		return
	}

	response := JoinResponse{
		Success: true,
		Message: fmt.Sprintf("Node %s successfully joined cluster", req.NodeID),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to encode join response: %v", err)
	}
	logger.Info("added node %s (%s) to cluster via join API", req.NodeID, req.Address)
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
		state = &store.JobExecutionState{
			JobID:     id,
			Status:    store.StatusPending,
			CreatedAt: time.Now().Unix(),
			Attempts:  []store.ExecutionAttempt{},
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
		attempts = []store.ExecutionAttempt{}
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
	if err == nil && state.Status == store.StatusInProgress {
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
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to cancel job: %v", err))
		return
	}

	// Get updated status
	state, err = statusTracker.GetStatus(id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to get job status: %v", err))
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
	status := store.JobStatus(statusParam)
	validStatuses := []store.JobStatus{
		store.StatusPending,
		store.StatusInProgress,
		store.StatusCompleted,
		store.StatusFailed,
		store.StatusCancelled,
		store.StatusTimeout,
	}

	isValid := false
	for _, validStatus := range validStatuses {
		if status == validStatus {
			isValid = true
			break
		}
	}

	if !isValid {
		h.writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid status: %s", statusParam))
		return
	}

	// Get jobs by status from FSM (works on both leader and follower)
	statusTracker := store.NewStatusTracker(h.store)
	states, err := statusTracker.ListByStatus(status)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to list jobs: %v", err))
		return
	}

	// If no states found, return empty array
	if states == nil {
		states = []*store.JobExecutionState{}
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

func (h *Handlers) writeError(w http.ResponseWriter, status int, message string) {
	if status >= 400 {
		ctx := context.Background()
		if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
			globalMetrics.IncrementHTTPRequests(ctx, "ERROR", "error", status)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": message}); err != nil {
		logger.Error("failed to encode error response: %v", err)
	}
}
