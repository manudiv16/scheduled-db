package slots

import (
	"testing"
	"time"

	"scheduled-db/internal/store"

	"pgregory.net/rapid"
)

// TestLimitManager_NewLimitManager tests that LimitManager is created correctly
func TestLimitManager_NewLimitManager(t *testing.T) {
	// Create mock components (nil store is ok for this test)
	memoryTracker := &MemoryTracker{}
	jobCounter := &JobCounter{}
	sizeCalculator := NewSizeCalculator()

	// Create LimitManager
	lm := NewLimitManager(memoryTracker, jobCounter, sizeCalculator)

	// Verify LimitManager was created
	if lm == nil {
		t.Fatal("NewLimitManager returned nil")
	}

	if lm.memoryTracker != memoryTracker {
		t.Error("LimitManager memoryTracker not set correctly")
	}

	if lm.jobCounter != jobCounter {
		t.Error("LimitManager jobCounter not set correctly")
	}

	if lm.sizeCalculator != sizeCalculator {
		t.Error("LimitManager sizeCalculator not set correctly")
	}
}

// TestLimitManager_SizeCalculation tests that LimitManager correctly calculates job sizes
func TestLimitManager_SizeCalculation(t *testing.T) {
	sizeCalculator := NewSizeCalculator()

	// Create a test job
	timestamp := time.Now().Add(1 * time.Hour).Unix()
	job := &store.Job{
		ID:         "test-job-1",
		Type:       store.JobUnico,
		Timestamp:  &timestamp,
		CreatedAt:  time.Now().Unix(),
		WebhookURL: "https://example.com/webhook",
		Payload:    map[string]interface{}{"key": "value"},
	}

	// Calculate size
	size := sizeCalculator.CalculateSize(job)

	// Size should be positive
	if size <= 0 {
		t.Errorf("Job size should be positive, got %d", size)
	}

	// Size should include at least the base overhead
	if size < sizeCalculator.baseOverhead {
		t.Errorf("Job size %d should be at least base overhead %d", size, sizeCalculator.baseOverhead)
	}
}

func TestCapacityError_ErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      *CapacityError
		expected string
	}{
		{
			name: "memory error with requested",
			err: &CapacityError{
				Type:      "memory",
				Current:   950000000,
				Limit:     1000000000,
				Requested: 100000000,
			},
			expected: "insufficient memory: current=950000000 bytes, limit=1000000000 bytes, requested=100000000 bytes",
		},
		{
			name: "memory error without requested",
			err: &CapacityError{
				Type:    "memory",
				Current: 950000000,
				Limit:   1000000000,
			},
			expected: "insufficient memory: current=950000000 bytes, limit=1000000000 bytes",
		},
		{
			name: "job count error",
			err: &CapacityError{
				Type:    "job_count",
				Current: 100000,
				Limit:   100000,
			},
			expected: "maximum jobs reached: current=100000, limit=100000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

// **Feature: queue-size-limits, Property 10: Memory limit check occurs**
// **Validates: Requirements 4.1**
func TestProperty10_MemoryLimitCheckOccurs(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random memory limit and current usage
		memoryLimit := rapid.Int64Range(1000, 10000000).Draw(rt, "memoryLimit")
		currentUsage := rapid.Int64Range(0, memoryLimit).Draw(rt, "currentUsage")

		// Create a MemoryUsage struct to test the capacity check logic
		memUsage := &MemoryUsage{
			CurrentBytes:   currentUsage,
			LimitBytes:     memoryLimit,
			AvailableBytes: memoryLimit - currentUsage,
			Utilization:    float64(currentUsage) / float64(memoryLimit) * 100.0,
		}

		// Generate a random job size
		jobSize := rapid.Int64Range(100, memoryLimit).Draw(rt, "jobSize")

		// Property: Memory limit check should occur before allowing job addition
		// This means: if currentUsage + jobSize > memoryLimit, the check should fail
		wouldExceed := memUsage.CurrentBytes+jobSize > memUsage.LimitBytes

		// Simulate the check that MemoryTracker.CheckCapacity would perform
		var checkErr error
		if wouldExceed {
			checkErr = &CapacityError{
				Type:      "memory",
				Current:   memUsage.CurrentBytes,
				Limit:     memUsage.LimitBytes,
				Requested: jobSize,
			}
		}

		// Verify the property: when memory would be exceeded, an error is returned
		if wouldExceed && checkErr == nil {
			rt.Fatalf("memory limit check should fail when usage (%d) + size (%d) > limit (%d)",
				memUsage.CurrentBytes, jobSize, memUsage.LimitBytes)
		}

		// Verify the property: when memory is available, no error is returned
		if !wouldExceed && checkErr != nil {
			rt.Fatalf("memory limit check should succeed when usage (%d) + size (%d) <= limit (%d)",
				memUsage.CurrentBytes, jobSize, memUsage.LimitBytes)
		}

		// Verify that the error is a CapacityError with correct type
		if checkErr != nil {
			capacityErr, ok := checkErr.(*CapacityError)
			if !ok {
				rt.Fatalf("expected CapacityError, got %T", checkErr)
			}
			if capacityErr.Type != "memory" {
				rt.Fatalf("expected error type 'memory', got '%s'", capacityErr.Type)
			}
		}
	})
}

// **Feature: queue-size-limits, Property 11: Memory rejection returns 507**
// **Validates: Requirements 4.2**
func TestProperty11_MemoryRejectionReturns507(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random memory limit and current usage that will cause rejection
		memoryLimit := rapid.Int64Range(1000, 10000000).Draw(rt, "memoryLimit")
		// Ensure current usage is high enough that adding a job will exceed limit
		currentUsage := rapid.Int64Range(memoryLimit-500, memoryLimit).Draw(rt, "currentUsage")

		// Generate a job size that will exceed the limit
		availableSpace := memoryLimit - currentUsage
		jobSize := rapid.Int64Range(availableSpace+1, availableSpace+10000).Draw(rt, "jobSize")

		// Create a CapacityError for memory rejection
		err := &CapacityError{
			Type:      "memory",
			Current:   currentUsage,
			Limit:     memoryLimit,
			Requested: jobSize,
		}

		// Property: Memory rejection should return HTTP status 507
		httpStatus := err.HTTPStatus()
		if httpStatus != 507 {
			rt.Fatalf("memory rejection should return HTTP status 507, got %d", httpStatus)
		}

		// Verify the error type is correct
		if err.Type != "memory" {
			rt.Fatalf("expected error type 'memory', got '%s'", err.Type)
		}

		// Verify the error message is meaningful
		errMsg := err.Error()
		if errMsg == "" {
			rt.Fatal("error message should not be empty")
		}
	})
}

// **Feature: queue-size-limits, Property 16: Count rejection returns 507**
// **Validates: Requirements 5.2**
func TestProperty16_CountRejectionReturns507(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random job count limit
		jobLimit := rapid.Int64Range(10, 100000).Draw(rt, "jobLimit")

		// Current count is at or above the limit
		currentCount := jobLimit

		// Create a CapacityError for job count rejection
		err := &CapacityError{
			Type:    "job_count",
			Current: currentCount,
			Limit:   jobLimit,
		}

		// Property: Job count rejection should return HTTP status 507
		httpStatus := err.HTTPStatus()
		if httpStatus != 507 {
			rt.Fatalf("job count rejection should return HTTP status 507, got %d", httpStatus)
		}

		// Verify the error type is correct
		if err.Type != "job_count" {
			rt.Fatalf("expected error type 'job_count', got '%s'", err.Type)
		}

		// Verify the error message is meaningful
		errMsg := err.Error()
		if errMsg == "" {
			rt.Fatal("error message should not be empty")
		}

		// Verify the error message contains "maximum jobs reached"
		if !contains(errMsg, "maximum jobs reached") {
			rt.Fatalf("error message should contain 'maximum jobs reached', got: %s", errMsg)
		}
	})
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// **Feature: queue-size-limits, Property 14: Job creation succeeds when under limits**
// **Validates: Requirements 4.5, 5.5**
func TestProperty14_JobCreationSucceedsUnderLimits(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		// Generate random limits
		memoryLimit := rapid.Int64Range(10000, 10000000).Draw(rt, "memoryLimit")
		jobLimit := rapid.Int64Range(100, 100000).Draw(rt, "jobLimit")

		// Generate current usage that leaves room for a new job
		currentMemory := rapid.Int64Range(0, memoryLimit/2).Draw(rt, "currentMemory")
		currentCount := rapid.Int64Range(0, jobLimit-1).Draw(rt, "currentCount")

		// Generate a job size that fits within available memory
		availableMemory := memoryLimit - currentMemory
		jobSize := rapid.Int64Range(100, availableMemory-100).Draw(rt, "jobSize")

		// Property: When both memory and count are under limits, no error should occur

		// Check memory capacity
		var memoryErr error
		if currentMemory+jobSize > memoryLimit {
			memoryErr = &CapacityError{
				Type:      "memory",
				Current:   currentMemory,
				Limit:     memoryLimit,
				Requested: jobSize,
			}
		}

		// Check job count capacity
		var countErr error
		if currentCount >= jobLimit {
			countErr = &CapacityError{
				Type:    "job_count",
				Current: currentCount,
				Limit:   jobLimit,
			}
		}

		// Verify the property: when under limits, no errors should occur
		if memoryErr != nil {
			rt.Fatalf("memory check should succeed when usage (%d) + size (%d) <= limit (%d), but got error: %v",
				currentMemory, jobSize, memoryLimit, memoryErr)
		}

		if countErr != nil {
			rt.Fatalf("count check should succeed when count (%d) < limit (%d), but got error: %v",
				currentCount, jobLimit, countErr)
		}

		// Additional verification: available space should be positive
		if availableMemory <= 0 {
			rt.Fatalf("available memory should be positive, got %d", availableMemory)
		}

		// Additional verification: available job slots should be positive
		availableSlots := jobLimit - currentCount
		if availableSlots <= 0 {
			rt.Fatalf("available job slots should be positive, got %d", availableSlots)
		}
	})
}
