package slots

import (
	"testing"

	"scheduled-db/internal/store/types"
)

func TestSizeCalculator_CalculateSize(t *testing.T) {
	sc := NewSizeCalculator()

	tests := []struct {
		name     string
		job      *types.Job
		minSize  int64 // Minimum expected size
		validate func(t *testing.T, size int64, job *types.Job)
	}{
		{
			name: "minimal job with only ID",
			job: &types.Job{
				ID:   "test-id-123",
				Type: types.JobUnico,
			},
			minSize: 128 + 11 + 64, // base + ID length + fixed fields
			validate: func(t *testing.T, size int64, job *types.Job) {
				if size < 128 {
					t.Errorf("size should include base overhead of 128 bytes, got %d", size)
				}
			},
		},
		{
			name: "job with webhook URL",
			job: &types.Job{
				ID:         "test-id",
				Type:       types.JobUnico,
				WebhookURL: "https://example.com/webhook",
			},
			minSize: 128 + 7 + 27 + 64, // base + ID + webhook + fixed
			validate: func(t *testing.T, size int64, job *types.Job) {
				expectedMin := int64(128 + len(job.ID) + len(job.WebhookURL) + 64)
				if size < expectedMin {
					t.Errorf("size should be at least %d (includes webhook URL), got %d", expectedMin, size)
				}
			},
		},
		{
			name: "job with cron expression",
			job: &types.Job{
				ID:       "test-id",
				Type:     types.JobRecurrente,
				CronExpr: "0 0 * * *",
			},
			minSize: 128 + 7 + 9 + 64, // base + ID + cron + fixed
			validate: func(t *testing.T, size int64, job *types.Job) {
				expectedMin := int64(128 + len(job.ID) + len(job.CronExpr) + 64)
				if size < expectedMin {
					t.Errorf("size should be at least %d (includes cron expression), got %d", expectedMin, size)
				}
			},
		},
		{
			name: "job with payload",
			job: &types.Job{
				ID:   "test-id",
				Type: types.JobUnico,
				Payload: map[string]interface{}{
					"key1": "value1",
					"key2": 123,
				},
			},
			minSize: 128 + 7 + 64, // base + ID + fixed (payload size calculated separately)
			validate: func(t *testing.T, size int64, job *types.Job) {
				// Payload should add some size
				if size <= 128+7+64 {
					t.Errorf("size should include payload, got %d", size)
				}
			},
		},
		{
			name: "job with all fields",
			job: &types.Job{
				ID:         "test-id-full",
				Type:       types.JobRecurrente,
				CronExpr:   "*/5 * * * *",
				WebhookURL: "https://example.com/webhook/full",
				Payload: map[string]interface{}{
					"message": "test message",
					"count":   42,
					"nested": map[string]interface{}{
						"field": "value",
					},
				},
			},
			minSize: 128 + 12 + 11 + 32 + 64, // base + ID + cron + webhook + fixed
			validate: func(t *testing.T, size int64, job *types.Job) {
				// Should be a substantial size with all fields
				if size < 300 {
					t.Errorf("job with all fields should be at least 300 bytes, got %d", size)
				}
			},
		},
		{
			name: "job with nil payload",
			job: &types.Job{
				ID:      "test-id",
				Type:    types.JobUnico,
				Payload: nil,
			},
			minSize: 128 + 7 + 64,
			validate: func(t *testing.T, size int64, job *types.Job) {
				// Should not crash with nil payload
				if size < 128 {
					t.Errorf("size should include base overhead, got %d", size)
				}
			},
		},
		{
			name: "job with empty payload",
			job: &types.Job{
				ID:      "test-id",
				Type:    types.JobUnico,
				Payload: map[string]interface{}{},
			},
			minSize: 128 + 7 + 64,
			validate: func(t *testing.T, size int64, job *types.Job) {
				// Empty payload should serialize to "{}" which is 2 bytes
				expectedMin := int64(128 + 7 + 64 + 2)
				if size < expectedMin {
					t.Errorf("size should be at least %d (includes empty payload), got %d", expectedMin, size)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := sc.CalculateSize(tt.job)

			// Check minimum size
			if size < tt.minSize {
				t.Errorf("CalculateSize() = %d, want at least %d", size, tt.minSize)
			}

			// Check size is positive
			if size <= 0 {
				t.Errorf("CalculateSize() = %d, want positive value", size)
			}

			// Run custom validation
			if tt.validate != nil {
				tt.validate(t, size, tt.job)
			}
		})
	}
}

func TestSizeCalculator_PayloadSizeContribution(t *testing.T) {
	sc := NewSizeCalculator()

	jobWithoutPayload := &types.Job{
		ID:   "test-id",
		Type: types.JobUnico,
	}

	jobWithPayload := &types.Job{
		ID:   "test-id",
		Type: types.JobUnico,
		Payload: map[string]interface{}{
			"data": "some data here",
		},
	}

	sizeWithout := sc.CalculateSize(jobWithoutPayload)
	sizeWith := sc.CalculateSize(jobWithPayload)

	if sizeWith <= sizeWithout {
		t.Errorf("job with payload should be larger than without: with=%d, without=%d", sizeWith, sizeWithout)
	}

	// The difference should be at least the serialized payload size
	diff := sizeWith - sizeWithout
	if diff < 10 { // Minimum reasonable payload size
		t.Errorf("payload size contribution seems too small: %d bytes", diff)
	}
}

func TestSizeCalculator_calculatePayloadSize(t *testing.T) {
	sc := NewSizeCalculator()

	tests := []struct {
		name    string
		payload map[string]interface{}
		minSize int64
	}{
		{
			name:    "empty payload",
			payload: map[string]interface{}{},
			minSize: 2, // "{}"
		},
		{
			name: "simple payload",
			payload: map[string]interface{}{
				"key": "value",
			},
			minSize: 10, // Approximate JSON size
		},
		{
			name: "nested payload",
			payload: map[string]interface{}{
				"outer": map[string]interface{}{
					"inner": "value",
				},
			},
			minSize: 15,
		},
		{
			name: "payload with various types",
			payload: map[string]interface{}{
				"string": "text",
				"number": 42,
				"bool":   true,
				"null":   nil,
			},
			minSize: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size := sc.calculatePayloadSize(tt.payload)

			if size < tt.minSize {
				t.Errorf("calculatePayloadSize() = %d, want at least %d", size, tt.minSize)
			}

			if size < 0 {
				t.Errorf("calculatePayloadSize() = %d, want non-negative", size)
			}
		})
	}
}
