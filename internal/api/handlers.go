package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"scheduled-db/internal/store"

	"github.com/gorilla/mux"
)

type Handlers struct {
	store      *store.Store
	addressMap map[string]string // Map Raft address to HTTP address
}

type HealthResponse struct {
	Status string `json:"status"`
	Role   string `json:"role"`
	Leader string `json:"leader,omitempty"`
	NodeID string `json:"node_id"`
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

func NewHandlers(store *store.Store) *Handlers {
	handlers := &Handlers{
		store:      store,
		addressMap: make(map[string]string),
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

			h.addressMap[raftAddr] = httpAddr
			log.Printf("Mapped Raft %s -> HTTP %s", raftAddr, httpAddr)
		}
	}

	// Fallback: create default mapping for common development setup
	if len(h.addressMap) == 0 {
		h.createDefaultMapping()
	}
}

// createDefaultMapping creates standard development mapping
func (h *Handlers) createDefaultMapping() {
	// No default mappings - all ports should come from environment variables
	log.Println("No default port mappings - using environment variables only")
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

	if err := h.store.CreateJob(job); err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to create job: %v", err))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(job); err != nil {
		log.Printf("Failed to encode job response: %v", err)
	}
	log.Printf("Created job %s via API", job.ID)
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
		log.Printf("Failed to encode job response: %v", err)
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

	// Check if job exists
	_, exists := h.store.GetJob(id)
	if !exists {
		h.writeError(w, http.StatusNotFound, "Job not found")
		return
	}

	if err := h.store.DeleteJob(id); err != nil {
		h.writeError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to delete job: %v", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
	log.Printf("Deleted job %s via API", id)
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

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode health response: %v", err)
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
		log.Printf("Failed to encode cluster debug response: %v", err)
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
				log.Printf("Failed to write proxy response: %v", err)
				break
			}
		}
		if err != nil {
			break
		}
	}

	log.Printf("Proxied %s %s to leader %s", r.Method, r.URL.Path, leader)
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
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode join response: %v", err)
	}
	log.Printf("Added node %s (%s) to cluster via join API", req.NodeID, req.Address)
}

func (h *Handlers) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(ErrorResponse{Error: message}); err != nil {
		log.Printf("Failed to encode error response: %v", err)
	}
}
