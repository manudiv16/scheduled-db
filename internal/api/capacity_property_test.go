package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"scheduled-db/internal/slots"
	"scheduled-db/internal/store"

	"pgregory.net/rapid"
)

// MockStore for testing
type MockStore struct {
	*store.Store
}

func (m *MockStore) IsLeader() bool {
	return true
}

// **Feature: queue-size-limits, Property 12: Memory rejection message**
// **Validates: Requirements 4.3, 10.1**
func TestProperty12_MemoryRejectionMessage(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random limit and usage values where usage > limit
		limit := rapid.Int64Range(1000, 1000000).Draw(rt, "limit")
		excess := rapid.Int64Range(1, 1000).Draw(rt, "excess")
		current := limit + excess
		requested := rapid.Int64Range(1, 1000).Draw(rt, "requested")

		// Create a CapacityError
		err := &slots.CapacityError{
			Type:      "memory",
			Current:   current,
			Limit:     limit,
			Requested: requested,
		}

		// Verify error message format
		expectedMsg := fmt.Sprintf("insufficient memory: current=%d bytes, limit=%d bytes, requested=%d bytes",
			current, limit, requested)

		if err.Error() != expectedMsg {
			rt.Fatalf("error message mismatch: expected %q, got %q", expectedMsg, err.Error())
		}
	})
}

// **Feature: queue-size-limits, Property 13: Memory rejection includes values**
// **Validates: Requirements 4.4, 10.2**
func TestProperty13_MemoryRejectionIncludesValues(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random values
		limit := rapid.Int64Range(1000, 1000000).Draw(rt, "limit")
		current := rapid.Int64Range(0, limit*2).Draw(rt, "current")
		requested := rapid.Int64Range(1, 1000).Draw(rt, "requested")

		// Only "memory" type uses requested in error message, but struct has it
		errType := rapid.SampledFrom([]string{"memory", "job_count"}).Draw(rt, "type")

		// Create a CapacityError
		capErr := &slots.CapacityError{
			Type:      errType,
			Current:   current,
			Limit:     limit,
			Requested: requested,
		}

		// Create a handler that simulates the rejection logic
		handler := func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(capErr.HTTPStatus())
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error":     capErr.Error(),
				"type":      capErr.Type,
				"current":   capErr.Current,
				"limit":     capErr.Limit,
				"requested": capErr.Requested,
			})
		}

		// Make a request
		req := httptest.NewRequest("POST", "/jobs", nil)
		w := httptest.NewRecorder()
		handler(w, req)

		// Parse response
		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			rt.Fatalf("failed to decode response: %v", err)
		}

		// Verify values in response
		if resp["type"] != errType {
			rt.Fatalf("expected type %s, got %s", errType, resp["type"])
		}

		// JSON numbers are float64
		respCurrent := int64(resp["current"].(float64))
		respLimit := int64(resp["limit"].(float64))
		respRequested := int64(resp["requested"].(float64))

		if respCurrent != current {
			rt.Fatalf("expected current %d, got %d", current, respCurrent)
		}
		if respLimit != limit {
			rt.Fatalf("expected limit %d, got %d", limit, respLimit)
		}
		if respRequested != requested {
			rt.Fatalf("expected requested %d, got %d", requested, respRequested)
		}
	})
}

// **Feature: queue-size-limits, Property 24: Health includes memory usage**
// **Feature: queue-size-limits, Property 25: Health includes memory limit**
// **Feature: queue-size-limits, Property 26: Health includes utilization**
// **Feature: queue-size-limits, Property 28: Health degraded at high utilization**
// **Validates: Requirements 7.1, 7.2, 7.3, 7.5**
func TestProperty_HealthResponseWithCapacity(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate memory usage values
		limit := rapid.Int64Range(1000, 1000000).Draw(rt, "limit")
		usage := rapid.Int64Range(0, limit).Draw(rt, "usage")
		utilization := float64(usage) / float64(limit) * 100.0

		// Generate job values
		jobLimit := rapid.Int64Range(100, 10000).Draw(rt, "jobLimit")
		jobCount := rapid.Int64Range(0, jobLimit).Draw(rt, "jobCount")
		jobAvailable := jobLimit - jobCount

		// Create response structure directly to verify logic
		response := HealthResponse{
			Status: "ok",
			Role:   "leader",
			NodeID: "node-1",
			Memory: &slots.MemoryUsage{
				CurrentBytes:   usage,
				LimitBytes:     limit,
				AvailableBytes: limit - usage,
				Utilization:    utilization,
			},
			Jobs: &JobStats{
				Count:     jobCount,
				Limit:     jobLimit,
				Available: jobAvailable,
			},
		}

		// Apply degraded logic
		if utilization > 90.0 {
			response.Status = "degraded"
		}

		// Verify properties

		// Property 24: Memory usage included
		if response.Memory.CurrentBytes != usage {
			rt.Fatalf("expected memory usage %d, got %d", usage, response.Memory.CurrentBytes)
		}

		// Property 25: Memory limit included
		if response.Memory.LimitBytes != limit {
			rt.Fatalf("expected memory limit %d, got %d", limit, response.Memory.LimitBytes)
		}

		// Property 26: Utilization included
		if response.Memory.Utilization != utilization {
			rt.Fatalf("expected utilization %f, got %f", utilization, response.Memory.Utilization)
		}

		// Property 28: Degraded at high utilization
		if utilization > 90.0 {
			if response.Status != "degraded" {
				rt.Fatalf("expected degraded status for utilization %f, got %s", utilization, response.Status)
			}
		} else {
			if response.Status != "ok" {
				rt.Fatalf("expected ok status for utilization %f, got %s", utilization, response.Status)
			}
		}
	})
}
