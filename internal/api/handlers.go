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
		if sanitized == "" {
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
