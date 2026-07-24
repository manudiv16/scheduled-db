package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/store"
)

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

	statusCode := http.StatusOK
	if response.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
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
