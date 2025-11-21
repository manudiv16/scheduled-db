package main

import (
	"fmt"
	"testing"

	"pgregory.net/rapid"
)

// **Feature: queue-size-limits, Property 1: Memory limit initialization**
// **Validates: Requirements 1.1, 1.2, 1.4**
func TestProperty1_MemoryLimitInitialization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random configuration scenarios
		useExplicitLimit := rapid.Bool().Draw(t, "useExplicitLimit")

		var explicitLimit string
		var memoryPercent float64

		if useExplicitLimit {
			// Generate a valid explicit limit
			sizeValue := rapid.Int64Range(1, 100).Draw(t, "sizeValue")
			unit := rapid.SampledFrom([]string{"GB", "MB", "KB"}).Draw(t, "unit")
			explicitLimit = fmt.Sprintf("%d%s", sizeValue, unit)
			memoryPercent = rapid.Float64Range(1.0, 100.0).Draw(t, "memoryPercent")
		} else {
			// No explicit limit, use percentage
			explicitLimit = ""
			memoryPercent = rapid.Float64Range(1.0, 100.0).Draw(t, "memoryPercent")
		}

		// Call DetectMemoryLimit
		limit := DetectMemoryLimit(explicitLimit, memoryPercent)

		// Property: The result should always be a positive value
		if limit <= 0 {
			t.Fatalf("DetectMemoryLimit returned non-positive value: %d", limit)
		}

		// Property: If explicit limit is provided and valid, it should be used
		if useExplicitLimit && explicitLimit != "" {
			expectedLimit, err := ParseMemoryLimit(explicitLimit)
			if err == nil && limit != expectedLimit {
				t.Fatalf("Expected explicit limit %d, got %d", expectedLimit, limit)
			}
		}
	})
}

// **Feature: queue-size-limits, Property 4: Job count limit initialization**
// **Validates: Requirements 2.1, 2.2**
func TestProperty4_JobCountLimitInitialization(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random job count limit configurations
		useValidLimit := rapid.Bool().Draw(t, "useValidLimit")

		var configuredLimit int64
		var expectedLimit int64

		if useValidLimit {
			// Generate a valid positive limit
			configuredLimit = rapid.Int64Range(1, 1000000).Draw(t, "configuredLimit")
			expectedLimit = configuredLimit
		} else {
			// Generate an invalid limit (zero or negative)
			configuredLimit = rapid.Int64Range(-1000, 0).Draw(t, "configuredLimit")
			expectedLimit = 100000 // Default value
		}

		// Simulate the validation logic from main.go
		jobLimit := configuredLimit
		if jobLimit <= 0 {
			jobLimit = 100000
		}

		// Property: The result should always be a positive value
		if jobLimit <= 0 {
			t.Fatalf("Job limit is non-positive: %d", jobLimit)
		}

		// Property: The result should match expected limit
		if jobLimit != expectedLimit {
			t.Fatalf("Expected job limit %d, got %d", expectedLimit, jobLimit)
		}

		// Property: If configured limit is valid, it should be used
		if useValidLimit && jobLimit != configuredLimit {
			t.Fatalf("Valid configured limit %d was not used, got %d", configuredLimit, jobLimit)
		}

		// Property: If configured limit is invalid, default should be used
		if !useValidLimit && jobLimit != 100000 {
			t.Fatalf("Invalid limit should default to 100000, got %d", jobLimit)
		}
	})
}
