package metrics

import (
	"context"
	"testing"
	"time"
)

func TestStatusMetrics(t *testing.T) {
	// Create metrics instance
	m, err := NewMetrics()
	if err != nil {
		t.Fatalf("failed to create metrics: %v", err)
	}

	ctx := context.Background()

	// Test UpdateJobsByStatus
	m.UpdateJobsByStatus(ctx, "pending", 1)
	m.UpdateJobsByStatus(ctx, "pending", -1)
	m.UpdateJobsByStatus(ctx, "in_progress", 1)

	// Test IncrementExecutionFailures
	m.IncrementExecutionFailures(ctx, "timeout")
	m.IncrementExecutionFailures(ctx, "connection_error")
	m.IncrementExecutionFailures(ctx, "http_error")

	// Test RecordRetryCount
	m.RecordRetryCount(ctx, 0)
	m.RecordRetryCount(ctx, 1)
	m.RecordRetryCount(ctx, 2)
	m.RecordRetryCount(ctx, 3)

	// Test RecordStatusQueryLatency
	m.RecordStatusQueryLatency(ctx, 10*time.Millisecond, "get_job_status")
	m.RecordStatusQueryLatency(ctx, 50*time.Millisecond, "get_job_executions")
	m.RecordStatusQueryLatency(ctx, 100*time.Millisecond, "list_jobs_by_status")

	// If we got here without panicking, the metrics are working
	t.Log("All status metrics recorded successfully")
}

func TestStatusMetricsWithNilMetrics(t *testing.T) {
	// Test that methods handle nil metrics gracefully
	var m *Metrics
	ctx := context.Background()

	// These should not panic
	m.UpdateJobsByStatus(ctx, "pending", 1)
	m.IncrementExecutionFailures(ctx, "timeout")
	m.RecordRetryCount(ctx, 1)
	m.RecordStatusQueryLatency(ctx, 10*time.Millisecond, "test")

	t.Log("Nil metrics handled gracefully")
}
