package store

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
		expected  int64
	}{
		{
			name:      "empty string",
			input:     "",
			wantError: false,
			expected:  0,
		},
		{
			name:      "epoch seconds",
			input:     "1640995200",
			wantError: false,
			expected:  1640995200,
		},
		{
			name:      "RFC3339 format",
			input:     "2022-01-01T00:00:00Z",
			wantError: false,
			expected:  1640995200,
		},
		{
			name:      "RFC3339 with timezone",
			input:     "2022-01-01T02:00:00+02:00",
			wantError: false,
			expected:  1640995200,
		},
		{
			name:      "RFC3339 with negative timezone",
			input:     "2021-12-31T19:00:00-05:00",
			wantError: false,
			expected:  1640995200,
		},
		{
			name:      "simple format without timezone",
			input:     "2022-01-01 00:00:00",
			wantError: false,
			expected:  1640995200,
		},
		{
			name:      "invalid format",
			input:     "invalid-timestamp",
			wantError: true,
			expected:  0,
		},
		{
			name:      "invalid epoch",
			input:     "not-a-number",
			wantError: true,
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTimestamp(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("ParseTimestamp() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ParseTimestamp() unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("ParseTimestamp() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestCreateJobRequest_ToJob(t *testing.T) {
	tests := []struct {
		name      string
		request   CreateJobRequest
		wantError bool
		validate  func(*testing.T, *Job)
	}{
		{
			name: "valid unique job with RFC3339 timestamp",
			request: CreateJobRequest{
				ID:        "test-job-1",
				Type:      JobUnico,
				Timestamp: "2025-01-01T12:00:00Z",
			},
			wantError: false,
			validate: func(t *testing.T, job *Job) {
				if job.ID != "test-job-1" {
					t.Errorf("Job.ID = %s, want test-job-1", job.ID)
				}
				if job.Type != JobUnico {
					t.Errorf("Job.Type = %s, want %s", job.Type, JobUnico)
				}
				if job.Timestamp == nil {
					t.Error("Job.Timestamp is nil")
				} else if *job.Timestamp != 1735732800 {
					t.Errorf("Job.Timestamp = %d, want 1735732800", *job.Timestamp)
				}
			},
		},
		{
			name: "valid unique job with epoch timestamp",
			request: CreateJobRequest{
				Type:      JobUnico,
				Timestamp: "1735732800",
			},
			wantError: false,
			validate: func(t *testing.T, job *Job) {
				if job.ID == "" {
					t.Error("Job.ID should be generated when empty")
				}
				if job.Type != JobUnico {
					t.Errorf("Job.Type = %s, want %s", job.Type, JobUnico)
				}
				if job.Timestamp == nil {
					t.Error("Job.Timestamp is nil")
				} else if *job.Timestamp != 1735732800 {
					t.Errorf("Job.Timestamp = %d, want 1735732800", *job.Timestamp)
				}
			},
		},
		{
			name: "valid recurring job",
			request: CreateJobRequest{
				Type:     JobRecurrente,
				CronExpr: "0 0 * * *",
			},
			wantError: false,
			validate: func(t *testing.T, job *Job) {
				if job.Type != JobRecurrente {
					t.Errorf("Job.Type = %s, want %s", job.Type, JobRecurrente)
				}
				if job.CronExpr != "0 0 * * *" {
					t.Errorf("Job.CronExpr = %s, want '0 0 * * *'", job.CronExpr)
				}
			},
		},
		{
			name: "recurring job with timestamp and last_date",
			request: CreateJobRequest{
				Type:      JobRecurrente,
				CronExpr:  "0 */6 * * *",
				Timestamp: "1735732800",
				LastDate:  "1735646400",
			},
			wantError: false,
			validate: func(t *testing.T, job *Job) {
				if job.CreatedAt != 1735732800 {
					t.Errorf("Job.CreatedAt = %d, want 1735732800", job.CreatedAt)
				}
				if job.LastDate == nil {
					t.Error("Job.LastDate is nil")
				} else if *job.LastDate != 1735646400 {
					t.Errorf("Job.LastDate = %d, want 1735646400", *job.LastDate)
				}
			},
		},
		{
			name: "invalid timestamp format",
			request: CreateJobRequest{
				Type:      JobUnico,
				Timestamp: "invalid-timestamp",
			},
			wantError: true,
		},
		{
			name: "invalid last_date format",
			request: CreateJobRequest{
				Type:     JobRecurrente,
				CronExpr: "0 0 * * *",
				LastDate: "invalid-date",
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job, err := tt.request.ToJob()

			if tt.wantError {
				if err == nil {
					t.Errorf("ToJob() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ToJob() unexpected error: %v", err)
				return
			}

			if job == nil {
				t.Error("ToJob() returned nil job")
				return
			}

			// Validate generated UUID when ID is empty
			if tt.request.ID == "" {
				if _, err := uuid.Parse(job.ID); err != nil {
					t.Errorf("Generated job ID is not a valid UUID: %s", job.ID)
				}
			}

			// Validate CreatedAt is set
			if job.CreatedAt == 0 && tt.request.Timestamp == "" {
				t.Error("Job.CreatedAt should be set")
			}

			if tt.validate != nil {
				tt.validate(t, job)
			}
		})
	}
}

func TestJob_Validate(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour).Unix()
	pastTime := time.Now().Add(-1 * time.Hour).Unix()

	tests := []struct {
		name      string
		job       Job
		wantError bool
		errorMsg  string
	}{
		{
			name: "valid unique job",
			job: Job{
				ID:        "test-job-1",
				Type:      JobUnico,
				Timestamp: &futureTime,
				CreatedAt: time.Now().Unix(),
			},
			wantError: false,
		},
		{
			name: "valid recurring job",
			job: Job{
				ID:        "test-job-2",
				Type:      JobRecurrente,
				CronExpr:  "0 0 * * *",
				CreatedAt: time.Now().Unix(),
			},
			wantError: false,
		},
		{
			name: "missing job ID",
			job: Job{
				Type:      JobUnico,
				Timestamp: &futureTime,
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
			errorMsg:  "job ID is required",
		},
		{
			name: "invalid job type",
			job: Job{
				ID:        "test-job-3",
				Type:      JobType("invalid"),
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
			errorMsg:  "invalid job type: invalid",
		},
		{
			name: "unique job missing timestamp",
			job: Job{
				ID:        "test-job-4",
				Type:      JobUnico,
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
			errorMsg:  "timestamp is required for unico jobs",
		},
		{
			name: "unique job with past timestamp",
			job: Job{
				ID:        "test-job-5",
				Type:      JobUnico,
				Timestamp: &pastTime,
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
			errorMsg:  "timestamp must be in the future",
		},
		{
			name: "recurring job missing cron expression",
			job: Job{
				ID:        "test-job-6",
				Type:      JobRecurrente,
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
			errorMsg:  "cron_expression is required for recurrente jobs",
		},
		{
			name: "recurring job with invalid cron expression",
			job: Job{
				ID:        "test-job-7",
				Type:      JobRecurrente,
				CronExpr:  "invalid-cron",
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
			errorMsg:  "invalid cron expression",
		},
		{
			name: "recurring job with valid complex cron",
			job: Job{
				ID:        "test-job-8",
				Type:      JobRecurrente,
				CronExpr:  "0 */15 * * *", // Every 15 minutes
				CreatedAt: time.Now().Unix(),
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()

			if tt.wantError {
				if err == nil {
					t.Errorf("Validate() expected error but got none")
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Validate() error = %v, want containing %v", err, tt.errorMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("Validate() unexpected error: %v", err)
			}
		})
	}
}

func TestJob_Serialization(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour).Unix()
	lastDate := time.Now().Add(-1 * time.Hour).Unix()

	originalJob := &Job{
		ID:        "test-serialization",
		Type:      JobRecurrente,
		CronExpr:  "0 0 * * *",
		LastDate:  &lastDate,
		CreatedAt: futureTime,
	}

	// Test ToBytes
	data, err := originalJob.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("ToBytes() returned empty data")
	}

	// Verify it's valid JSON
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(data, &jsonMap); err != nil {
		t.Errorf("ToBytes() did not produce valid JSON: %v", err)
	}

	// Test JobFromBytes
	deserializedJob, err := JobFromBytes(data)
	if err != nil {
		t.Fatalf("JobFromBytes() error = %v", err)
	}

	if deserializedJob == nil {
		t.Fatal("JobFromBytes() returned nil job")
	}

	// Compare all fields
	if deserializedJob.ID != originalJob.ID {
		t.Errorf("ID: got %s, want %s", deserializedJob.ID, originalJob.ID)
	}
	if deserializedJob.Type != originalJob.Type {
		t.Errorf("Type: got %s, want %s", deserializedJob.Type, originalJob.Type)
	}
	if deserializedJob.CronExpr != originalJob.CronExpr {
		t.Errorf("CronExpr: got %s, want %s", deserializedJob.CronExpr, originalJob.CronExpr)
	}
	if deserializedJob.CreatedAt != originalJob.CreatedAt {
		t.Errorf("CreatedAt: got %d, want %d", deserializedJob.CreatedAt, originalJob.CreatedAt)
	}

	// Check LastDate pointer
	if originalJob.LastDate == nil && deserializedJob.LastDate != nil {
		t.Error("LastDate: original nil but deserialized not nil")
	} else if originalJob.LastDate != nil && deserializedJob.LastDate == nil {
		t.Error("LastDate: original not nil but deserialized nil")
	} else if originalJob.LastDate != nil && deserializedJob.LastDate != nil {
		if *deserializedJob.LastDate != *originalJob.LastDate {
			t.Errorf("LastDate: got %d, want %d", *deserializedJob.LastDate, *originalJob.LastDate)
		}
	}
}

func TestJobFromBytes_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{"invalid": json}`)

	job, err := JobFromBytes(invalidJSON)
	if err == nil {
		t.Error("JobFromBytes() expected error for invalid JSON but got none")
	}
	if job != nil {
		t.Error("JobFromBytes() expected nil job for invalid JSON")
	}
}

func TestJobTypes_Constants(t *testing.T) {
	if JobUnico != "unico" {
		t.Errorf("JobUnico = %s, want 'unico'", JobUnico)
	}
	if JobRecurrente != "recurrente" {
		t.Errorf("JobRecurrente = %s, want 'recurrente'", JobRecurrente)
	}
}

func TestCreateJobRequest_EmptyID(t *testing.T) {
	request := CreateJobRequest{
		Type:      JobUnico,
		Timestamp: "2025-01-01T12:00:00Z",
	}

	job, err := request.ToJob()
	if err != nil {
		t.Fatalf("ToJob() error = %v", err)
	}

	if job.ID == "" {
		t.Error("ToJob() should generate UUID when ID is empty")
	}

	// Validate it's a proper UUID
	if _, err := uuid.Parse(job.ID); err != nil {
		t.Errorf("Generated ID is not a valid UUID: %s, error: %v", job.ID, err)
	}
}

func TestJob_ValidateCronExpressions(t *testing.T) {
	validCronExpressions := []string{
		"0 0 * * *",      // Daily at midnight
		"0 */6 * * *",    // Every 6 hours
		"*/15 * * * *",   // Every 15 minutes
		"0 9-17 * * 1-5", // Business hours
		"0 0 1 * *",      // First day of month
		"0 0 * * 0",      // Every Sunday
	}

	for _, cronExpr := range validCronExpressions {
		t.Run("valid_cron_"+cronExpr, func(t *testing.T) {
			job := Job{
				ID:        "test-cron",
				Type:      JobRecurrente,
				CronExpr:  cronExpr,
				CreatedAt: time.Now().Unix(),
			}

			if err := job.Validate(); err != nil {
				t.Errorf("Validate() failed for valid cron expression %s: %v", cronExpr, err)
			}
		})
	}

	invalidCronExpressions := []string{
		"* * * * * *", // Too many fields
		"60 * * * *",  // Invalid minute
		"* 25 * * *",  // Invalid hour
		"invalid",     // Not a cron expression
		"",            // Empty string
	}

	for _, cronExpr := range invalidCronExpressions {
		t.Run("invalid_cron_"+cronExpr, func(t *testing.T) {
			job := Job{
				ID:        "test-cron",
				Type:      JobRecurrente,
				CronExpr:  cronExpr,
				CreatedAt: time.Now().Unix(),
			}

			if err := job.Validate(); err == nil {
				t.Errorf("Validate() should fail for invalid cron expression: %s", cronExpr)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			indexOfSubstring(s, substr) >= 0)))
}

func indexOfSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
