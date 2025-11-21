package slots

import (
	"encoding/json"

	"scheduled-db/internal/store"
)

// SizeCalculator calculates job memory footprint
type SizeCalculator struct {
	baseOverhead int64 // Fixed overhead per job
}

// NewSizeCalculator creates a new SizeCalculator with default overhead
func NewSizeCalculator() *SizeCalculator {
	return &SizeCalculator{
		baseOverhead: 128, // Base overhead for job struct and metadata
	}
}

// CalculateSize returns job size in bytes
// Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5
func (sc *SizeCalculator) CalculateSize(job *store.Job) int64 {
	size := sc.baseOverhead

	// Job ID (string length)
	size += int64(len(job.ID))

	// Webhook URL (string length)
	if job.WebhookURL != "" {
		size += int64(len(job.WebhookURL))
	}

	// Cron expression (string length)
	if job.CronExpr != "" {
		size += int64(len(job.CronExpr))
	}

	// Payload (serialized size)
	if job.Payload != nil {
		payloadSize := sc.calculatePayloadSize(job.Payload)
		size += payloadSize
	}

	// Fixed fields (timestamps, type, etc.)
	// - Type (JobType string): ~16 bytes
	// - Timestamp (*int64): 8 bytes
	// - LastDate (*int64): 8 bytes
	// - CreatedAt (int64): 8 bytes
	// Total: ~40 bytes, round up to 64 for safety
	size += 64

	return size
}

// calculatePayloadSize estimates serialized payload size
func (sc *SizeCalculator) calculatePayloadSize(payload map[string]interface{}) int64 {
	// Serialize to JSON to get accurate size
	data, err := json.Marshal(payload)
	if err != nil {
		// If serialization fails, return 0 (no payload size)
		return 0
	}
	return int64(len(data))
}
