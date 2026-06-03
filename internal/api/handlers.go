package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"regexp"
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
	addressMap             map[string]string // Map Raft address to HTTP address (populated once at init)
	healthFailureThreshold float64
	proxyClient            *http.Client // reusable HTTP client for proxy requests
}

type JobStats struct {
	Count     int64 `json:"count"`
	Limit     int64 `json:"limit"`
	Available int64 `json:"available"`
}

type HealthResponse struct {
	Status    string                `json:"status"`
	Role      string                `json:"role"`
	Leader    string                `json:"leader,omitempty"`
	NodeID    string                `json:"node_id"`
	Memory    *slots.MemoryUsage    `json:"memory,omitempty"`
	Jobs      *JobStats             `json:"jobs,omitempty"`      // Capacity stats
	Execution *store.ExecutionStats `json:"execution,omitempty"` // Status stats
}

type ErrorResponse struct {
	Error string `json:"error"`
}

// ipPattern matches IPv4 and IPv6 addresses
var ipPattern = regexp.MustCompile(`(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?|\[?[0-9a-fA-F:]+\]?(?::\d+)?`)

// pathPattern matches common filesystem paths (Unix and Windows)
var pathPattern = regexp.MustCompile(`(?:/[\w.-]+)+/?[\w.-]*\.(?:db|log|tmp|snap|dat|bin)`)

// raftPattern matches Raft-specific error patterns
var raftPattern = regexp.MustCompile(`(?:raft|Raft)[\s:]+.*?(?:address|peer|server|node|leader|follower|candidate)[\s:]*[\w.:\[\]-]+`)

// sanitizeError removes sensitive information from error messages.
// It strips IP addresses, file paths, and Raft topology details
// to prevent leaking internal infrastructure information to API clients.
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()

	// Replace IP addresses (both standalone and with ports)
	msg = ipPattern.ReplaceAllStringFunc(msg, func(match string) string {
		// Validate it looks like an IP before replacing
		host := match
		port := ""
		if idx := strings.LastIndex(match, ":"); idx > 0 {
			host = match[:idx]
			port = match[idx:]
		}
		// Strip brackets from IPv6
		if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
			host = host[1 : len(host)-1]
		}
		if net.ParseIP(host) != nil {
			if port != "" {
				return "[addr]:<port>"
			}
			return "[addr]"
		}
		return match
	})

	// Replace file paths
	msg = pathPattern.ReplaceAllString(msg, "[path]")

	// Replace Raft-specific information
	msg = raftPattern.ReplaceAllString(msg, "[raft error]")

	// Replace common internal service patterns
	msg = strings.ReplaceAll(msg, "scheduled-db", "[service]")
	msg = strings.ReplaceAll(msg, ".svc.cluster.local", "")

	return msg
}

// safeErrorMessage returns a user-safe error message for API responses.
// For client errors (4xx), it provides sanitized but informative messages.
// For server errors (5xx), it returns a generic message to avoid leaking internals.
func safeErrorMessage(prefix string, err error, isClientError bool) string {
	if isClientError {
		sanitized := sanitizeError(err)
		if sanitized == "" || sanitized == err.Error() {
			return fmt.Sprintf("%s", prefix)
		}
		return fmt.Sprintf("%s: %s", prefix, sanitized)
	}
	// Server errors: use generic message, log the real error
	return prefix
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
		proxyClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
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
		// Use the full FQDN for proper DNS resolution in Kubernetes
		// (e.g., scheduled-db-2.scheduled-db.default.svc.cluster.local:8080)
		return fmt.Sprintf("http://%s:8080", host), nil
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

	// Add execution stats
	statusTracker := store.NewStatusTracker(h.store)
	if execStats, err := statusTracker.GetExecutionStats(); err == nil {
		response.Execution = execStats

		// Check for degraded status based on failure rate
		// Only check if we have enough data (e.g. at least 10 finished jobs)
		finishedJobs := execStats.Completed + execStats.Failed + execStats.Timeout
		if finishedJobs >= 10 && execStats.FailureRate > h.healthFailureThreshold {
			// If already degraded (e.g. by memory), keep it, otherwise set to degraded
			if response.Status == "ok" {
				response.Status = "degraded"
			} else if response.Status == "degraded" && execStats.FailureRate > 0.5 {
				// If failure rate is very high (>50%), mark as unhealthy
				response.Status = "unhealthy"
			}
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
		logger.Error("failed to get cluster configuration: %v", err)
		serverList = []map[string]string{{"error": "Unable to retrieve cluster configuration"}}
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
		logger.Error("failed to resolve leader HTTP address: %v", err)
		h.writeError(w, http.StatusServiceUnavailable, "Leader unavailable")
		return
	}

	// Create proxy request
	proxyReq, err := http.NewRequest(r.Method, httpAddr+r.URL.Path, r.Body)
	if err != nil {
		logger.Error("failed to create proxy request: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Internal error")
		return
	}

	// Copy headers
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Execute proxy request using reusable client
	resp, err := h.proxyClient.Do(proxyReq)
	if err != nil {
		logger.Error("failed to proxy request to leader: %v", err)
		h.writeError(w, http.StatusBadGateway, "Unable to reach leader")
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

	// Copy response body efficiently
	if _, err := io.Copy(w, resp.Body); err != nil {
		logger.Error("failed to write proxy response: %v", err)
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
		logger.Error("failed to decode join request: %v", err)
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.NodeID == "" || req.Address == "" {
		h.writeError(w, http.StatusBadRequest, "node_id and address are required")
		return
	}

	// Add peer to Raft cluster
	if err := h.store.AddPeer(req.NodeID, req.Address); err != nil {
		logger.Error("failed to add peer: %v", err)
		h.writeError(w, http.StatusInternalServerError, "Failed to join cluster")
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
