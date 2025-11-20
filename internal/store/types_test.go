package store

import (
	"testing"
	"time"
)

func TestJobType_Constants(t *testing.T) {
	if JobUnico != "unico" {
		t.Errorf("JobUnico = %s, want 'unico'", JobUnico)
	}
	if JobRecurrente != "recurrente" {
		t.Errorf("JobRecurrente = %s, want 'recurrente'", JobRecurrente)
	}
}

func TestParseTimestamp(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantError bool
	}{
		{
			name:      "empty string",
			input:     "",
			wantError: false,
		},
		{
			name:      "epoch seconds",
			input:     "1704067200",
			wantError: false,
		},
		{
			name:      "RFC3339 format",
			input:     "2024-01-01T00:00:00Z",
			wantError: false,
		},
		{
			name:      "RFC3339 with positive timezone",
			input:     "2024-01-01T00:00:00+02:00",
			wantError: false,
		},
		{
			name:      "RFC3339 with negative timezone",
			input:     "2024-01-01T00:00:00-05:00",
			wantError: false,
		},
		{
			name:      "simple datetime format",
			input:     "2024-01-01 00:00:00",
			wantError: false,
		},
		{
			name:      "invalid format",
			input:     "not-a-timestamp",
			wantError: true,
		},
		{
			name:      "invalid date",
			input:     "2024-13-45T99:99:99Z",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseTimestamp(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("ParseTimestamp() expected error but got none, result = %d", result)
				}
			} else {
				if err != nil {
					t.Errorf("ParseTimestamp() unexpected error = %v", err)
				}
				if tt.input == "" && result != 0 {
					t.Errorf("ParseTimestamp() for empty string = %d, want 0", result)
				}
			}
		})
	}
}

func TestParseTimestamp_EpochConsistency(t *testing.T) {
	// Test that parsing epoch string returns the same value
	epoch := int64(1704067200)
	result, err := ParseTimestamp("1704067200")
	if err != nil {
		t.Fatalf("ParseTimestamp() error = %v", err)
	}
	if result != epoch {
		t.Errorf("ParseTimestamp() = %d, want %d", result, epoch)
	}
}

func TestParseTimestamp_RFC3339Consistency(t *testing.T) {
	// Test that parsing RFC3339 returns correct epoch
	input := "2024-01-01T00:00:00Z"
	result, err := ParseTimestamp(input)
	if err != nil {
		t.Fatalf("ParseTimestamp() error = %v", err)
	}

	// Parse the same string with time.Parse to verify
	expected, _ := time.Parse(time.RFC3339, input)
	if result != expected.Unix() {
		t.Errorf("ParseTimestamp() = %d, want %d", result, expected.Unix())
	}
}

func TestCreateJobRequest_ToJob_UnicoJob(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour)
	timestampStr := futureTime.Format(time.RFC3339)

	req := &CreateJobRequest{
		Type:       JobUnico,
		Timestamp:  timestampStr,
		WebhookURL: "https://example.com/webhook",
		Payload:    map[string]interface{}{"key": "value"},
	}

	job, err := req.ToJob()
	if err != nil {
		t.Fatalf("ToJob() error = %v", err)
	}

	if job.ID == "" {
		t.Error("ToJob() did not generate ID")
	}

	if job.Type != JobUnico {
		t.Errorf("Job.Type = %s, want %s", job.Type, JobUnico)
	}

	if job.Timestamp == nil {
		t.Fatal("Job.Timestamp is nil")
	}

	if job.WebhookURL != req.WebhookURL {
		t.Errorf("Job.WebhookURL = %s, want %s", job.WebhookURL, req.WebhookURL)
	}

	if job.Payload == nil {
		t.Error("Job.Payload is nil")
	}
}

func TestCreateJobRequest_ToJob_RecurrenteJob(t *testing.T) {
	req := &CreateJobRequest{
		Type:     JobRecurrente,
		CronExpr: "0 0 * * *",
	}

	job, err := req.ToJob()
	if err != nil {
		t.Fatalf("ToJob() error = %v", err)
	}

	if job.ID == "" {
		t.Error("ToJob() did not generate ID")
	}

	if job.Type != JobRecurrente {
		t.Errorf("Job.Type = %s, want %s", job.Type, JobRecurrente)
	}

	if job.CronExpr != req.CronExpr {
		t.Errorf("Job.CronExpr = %s, want %s", job.CronExpr, req.CronExpr)
	}
}

func TestCreateJobRequest_ToJob_WithCustomID(t *testing.T) {
	customID := "custom-job-id"
	futureTime := time.Now().Add(1 * time.Hour)

	req := &CreateJobRequest{
		ID:        customID,
		Type:      JobUnico,
		Timestamp: futureTime.Format(time.RFC3339),
	}

	job, err := req.ToJob()
	if err != nil {
		t.Fatalf("ToJob() error = %v", err)
	}

	if job.ID != customID {
		t.Errorf("Job.ID = %s, want %s", job.ID, customID)
	}
}

func TestCreateJobRequest_ToJob_InvalidTimestamp(t *testing.T) {
	req := &CreateJobRequest{
		Type:      JobUnico,
		Timestamp: "invalid-timestamp",
	}

	_, err := req.ToJob()
	if err == nil {
		t.Error("ToJob() expected error for invalid timestamp but got none")
	}
}

func TestCreateJobRequest_ToJob_WithLastDate(t *testing.T) {
	lastDate := time.Now().Add(30 * 24 * time.Hour)

	req := &CreateJobRequest{
		Type:     JobRecurrente,
		CronExpr: "0 0 * * *",
		LastDate: lastDate.Format(time.RFC3339),
	}

	job, err := req.ToJob()
	if err != nil {
		t.Fatalf("ToJob() error = %v", err)
	}

	if job.LastDate == nil {
		t.Fatal("Job.LastDate is nil")
	}

	expectedEpoch := lastDate.Unix()
	if *job.LastDate != expectedEpoch {
		t.Errorf("Job.LastDate = %d, want %d", *job.LastDate, expectedEpoch)
	}
}

func TestJob_Validate_UnicoJob(t *testing.T) {
	tests := []struct {
		name      string
		job       *Job
		wantError bool
	}{
		{
			name: "valid unico job",
			job: &Job{
				ID:        "test-job",
				Type:      JobUnico,
				Timestamp: func() *int64 { t := time.Now().Add(1 * time.Hour).Unix(); return &t }(),
				CreatedAt: time.Now().Unix(),
			},
			wantError: false,
		},
		{
			name: "unico job without timestamp",
			job: &Job{
				ID:        "test-job",
				Type:      JobUnico,
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
		},
		{
			name: "unico job with past timestamp",
			job: &Job{
				ID:        "test-job",
				Type:      JobUnico,
				Timestamp: func() *int64 { t := time.Now().Add(-1 * time.Hour).Unix(); return &t }(),
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
		},
		{
			name: "job without ID",
			job: &Job{
				Type:      JobUnico,
				Timestamp: func() *int64 { t := time.Now().Add(1 * time.Hour).Unix(); return &t }(),
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.job.Validate()

			if tt.wantError {
				if err == nil {
					t.Error("Validate() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestJob_Validate_RecurrenteJob(t *testing.T) {
	tests := []struct {
		name      string
		job       *Job
		wantError bool
	}{
		{
			name: "valid recurrente job",
			job: &Job{
				ID:        "test-job",
				Type:      JobRecurrente,
				CronExpr:  "0 0 * * *",
				CreatedAt: time.Now().Unix(),
			},
			wantError: false,
		},
		{
			name: "recurrente job without cron expression",
			job: &Job{
				ID:        "test-job",
				Type:      JobRecurrente,
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
		},
		{
			name: "recurrente job with invalid cron expression",
			job: &Job{
				ID:        "test-job",
				Type:      JobRecurrente,
				CronExpr:  "invalid cron",
				CreatedAt: time.Now().Unix(),
			},
			wantError: true,
		},
		{
			name: "recurrente job with valid complex cron",
			job: &Job{
				ID:        "test-job",
				Type:      JobRecurrente,
				CronExpr:  "*/15 * * * *", // Every 15 minutes
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
					t.Error("Validate() expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestJob_Validate_InvalidType(t *testing.T) {
	job := &Job{
		ID:        "test-job",
		Type:      JobType("invalid"),
		CreatedAt: time.Now().Unix(),
	}

	err := job.Validate()
	if err == nil {
		t.Error("Validate() expected error for invalid job type but got none")
	}
}

func TestJob_Serialization(t *testing.T) {
	futureTime := time.Now().Add(1 * time.Hour).Unix()
	originalJob := &Job{
		ID:         "test-job",
		Type:       JobUnico,
		Timestamp:  &futureTime,
		CreatedAt:  time.Now().Unix(),
		WebhookURL: "https://example.com/webhook",
		Payload:    map[string]interface{}{"key": "value"},
	}

	// Serialize
	data, err := originalJob.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes() error = %v", err)
	}

	// Deserialize
	deserializedJob, err := JobFromBytes(data)
	if err != nil {
		t.Fatalf("JobFromBytes() error = %v", err)
	}

	// Compare
	if deserializedJob.ID != originalJob.ID {
		t.Errorf("Job.ID = %s, want %s", deserializedJob.ID, originalJob.ID)
	}

	if deserializedJob.Type != originalJob.Type {
		t.Errorf("Job.Type = %s, want %s", deserializedJob.Type, originalJob.Type)
	}

	if deserializedJob.Timestamp == nil || *deserializedJob.Timestamp != *originalJob.Timestamp {
		t.Error("Job.Timestamp mismatch after serialization")
	}

	if deserializedJob.WebhookURL != originalJob.WebhookURL {
		t.Errorf("Job.WebhookURL = %s, want %s", deserializedJob.WebhookURL, originalJob.WebhookURL)
	}
}

func TestJobFromBytes_InvalidJSON(t *testing.T) {
	invalidJSON := []byte(`{"invalid": json}`)

	_, err := JobFromBytes(invalidJSON)
	if err == nil {
		t.Error("JobFromBytes() expected error for invalid JSON but got none")
	}
}

func TestSlotData_Structure(t *testing.T) {
	slot := &SlotData{
		Key:     100,
		MinTime: 1000,
		MaxTime: 1099,
		JobIDs:  []string{"job-1", "job-2"},
	}

	if slot.Key != 100 {
		t.Errorf("SlotData.Key = %d, want 100", slot.Key)
	}

	if slot.MinTime != 1000 {
		t.Errorf("SlotData.MinTime = %d, want 1000", slot.MinTime)
	}

	if slot.MaxTime != 1099 {
		t.Errorf("SlotData.MaxTime = %d, want 1099", slot.MaxTime)
	}

	if len(slot.JobIDs) != 2 {
		t.Errorf("SlotData.JobIDs length = %d, want 2", len(slot.JobIDs))
	}
}
