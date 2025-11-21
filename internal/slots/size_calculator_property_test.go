package slots

import (
	"testing"
	"time"

	"pgregory.net/rapid"

	"scheduled-db/internal/store"
)

// **Feature: queue-size-limits, Property 6: Job size includes all components**
// **Validates: Requirements 3.1, 3.2**
func TestProperty6_JobSizeIncludesAllComponents(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate random job components
		id := rapid.String().Draw(t, "id")
		webhookURL := rapid.String().Draw(t, "webhookURL")
		cronExpr := rapid.String().Draw(t, "cronExpr")

		job := &store.Job{
			ID:         id,
			Type:       store.JobUnico,
			WebhookURL: webhookURL,
			CronExpr:   cronExpr,
		}

		sc := NewSizeCalculator()
		size := sc.CalculateSize(job)

		// Calculate minimum expected size
		// Base overhead + ID length + webhook length + cron length + fixed fields
		minExpected := sc.baseOverhead + int64(len(id)) + int64(len(webhookURL)) + int64(len(cronExpr)) + 64

		if size < minExpected {
			t.Fatalf("job size %d should be at least %d (base=%d + id=%d + webhook=%d + cron=%d + fixed=64)",
				size, minExpected, sc.baseOverhead, len(id), len(webhookURL), len(cronExpr))
		}
	})
}

// **Feature: queue-size-limits, Property 7: Payload size contribution**
// **Validates: Requirements 3.3**
func TestProperty7_PayloadSizeContribution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random job ID
		id := rapid.String().Draw(t, "id")

		// Create job without payload
		jobWithoutPayload := &store.Job{
			ID:   id,
			Type: store.JobUnico,
		}

		// Generate a random payload with at least one entry
		payloadSize := rapid.IntRange(1, 10).Draw(t, "payloadSize")
		payload := make(map[string]interface{})
		for i := 0; i < payloadSize; i++ {
			key := rapid.String().Draw(t, "payloadKey")
			value := rapid.String().Draw(t, "payloadValue")
			payload[key] = value
		}

		// Create job with payload
		jobWithPayload := &store.Job{
			ID:      id,
			Type:    store.JobUnico,
			Payload: payload,
		}

		sc := NewSizeCalculator()
		sizeWithout := sc.CalculateSize(jobWithoutPayload)
		sizeWith := sc.CalculateSize(jobWithPayload)

		// Job with payload should be larger than without payload
		if sizeWith <= sizeWithout {
			t.Fatalf("job with payload (size=%d) should be larger than without payload (size=%d)",
				sizeWith, sizeWithout)
		}
	})
}

// **Feature: queue-size-limits, Property 8: Webhook URL size contribution**
// **Validates: Requirements 3.4**
func TestProperty8_WebhookURLSizeContribution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random job ID
		id := rapid.String().Draw(t, "id")

		// Create job without webhook URL
		jobWithoutWebhook := &store.Job{
			ID:   id,
			Type: store.JobUnico,
		}

		// Generate a random webhook URL (non-empty)
		webhookURL := rapid.StringMatching(`https?://[a-z]+\.[a-z]+/.*`).Draw(t, "webhookURL")

		// Create job with webhook URL
		jobWithWebhook := &store.Job{
			ID:         id,
			Type:       store.JobUnico,
			WebhookURL: webhookURL,
		}

		sc := NewSizeCalculator()
		sizeWithout := sc.CalculateSize(jobWithoutWebhook)
		sizeWith := sc.CalculateSize(jobWithWebhook)

		// The difference should be at least the length of the webhook URL
		expectedDiff := int64(len(webhookURL))
		actualDiff := sizeWith - sizeWithout

		if actualDiff < expectedDiff {
			t.Fatalf("size difference (%d) should be at least webhook URL length (%d)",
				actualDiff, expectedDiff)
		}
	})
}

// **Feature: queue-size-limits, Property 34: Size calculation performance**
// **Validates: Requirements 9.1**
func TestProperty34_SizeCalculationPerformance(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// Generate a random job with various fields
		id := rapid.String().Draw(t, "id")
		webhookURL := rapid.String().Draw(t, "webhookURL")
		cronExpr := rapid.String().Draw(t, "cronExpr")

		// Generate a payload with random size
		payloadSize := rapid.IntRange(0, 20).Draw(t, "payloadSize")
		payload := make(map[string]interface{})
		for i := 0; i < payloadSize; i++ {
			key := rapid.String().Draw(t, "payloadKey")
			value := rapid.String().Draw(t, "payloadValue")
			payload[key] = value
		}

		job := &store.Job{
			ID:         id,
			Type:       store.JobUnico,
			WebhookURL: webhookURL,
			CronExpr:   cronExpr,
			Payload:    payload,
		}

		sc := NewSizeCalculator()

		// Measure time for size calculation
		start := time.Now()
		_ = sc.CalculateSize(job)
		duration := time.Since(start)

		// Should complete in less than 1 millisecond
		maxDuration := time.Millisecond
		if duration > maxDuration {
			t.Fatalf("size calculation took %v, should be less than 1ms", duration)
		}
	})
}
