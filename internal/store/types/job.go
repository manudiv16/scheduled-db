package types

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// cronParser is a shared parser to avoid per-call allocation.
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

type JobType string

const (
	JobUnico      JobType = "unico"
	JobRecurrente JobType = "recurrente"
)

type Job struct {
	ID         string                 `json:"id"`
	Type       JobType                `json:"type"`
	Timestamp  *int64                 `json:"timestamp,omitempty"`       // epoch seconds para unico
	CronExpr   string                 `json:"cron_expression,omitempty"` // para recurrente
	LastDate   *int64                 `json:"last_date,omitempty"`       // optional epoch seconds
	CreatedAt  int64                  `json:"created_at"`
	WebhookURL string                 `json:"webhook_url,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

// Clone returns a deep copy of the Job.
func (j *Job) Clone() *Job {
	if j == nil {
		return nil
	}
	clone := *j
	if j.Timestamp != nil {
		ts := *j.Timestamp
		clone.Timestamp = &ts
	}
	if j.LastDate != nil {
		ld := *j.LastDate
		clone.LastDate = &ld
	}
	if j.Payload != nil {
		clone.Payload = make(map[string]interface{}, len(j.Payload))
		for k, v := range j.Payload {
			clone.Payload[k] = v
		}
	}
	return &clone
}

// Validate validates the job.
func (j *Job) Validate() error {
	if j.ID == "" {
		return fmt.Errorf("job ID is required")
	}

	if j.Type != JobUnico && j.Type != JobRecurrente {
		return fmt.Errorf("invalid job type: %s", j.Type)
	}

	if j.Type == JobUnico {
		if j.Timestamp == nil {
			return fmt.Errorf("timestamp is required for unico jobs")
		}
		if *j.Timestamp <= time.Now().Unix() {
			return fmt.Errorf("timestamp must be in the future")
		}
	}

	if j.Type == JobRecurrente {
		if j.CronExpr == "" {
			return fmt.Errorf("cron_expression is required for recurrente jobs")
		}
		if _, err := cronParser.Parse(j.CronExpr); err != nil {
			return fmt.Errorf("invalid cron expression: %v", err)
		}
	}

	return nil
}

// ToBytes serializes the job to bytes.
func (j *Job) ToBytes() ([]byte, error) {
	return json.Marshal(j)
}

// JobFromBytes deserializes bytes to a job.
func JobFromBytes(data []byte) (*Job, error) {
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

// CreateJobRequest represents a request to create a job.
type CreateJobRequest struct {
	ID         string                 `json:"id,omitempty"`
	Type       JobType                `json:"type"`
	Timestamp  string                 `json:"timestamp,omitempty"` // puede ser RFC3339 o epoch
	CronExpr   string                 `json:"cron_expression,omitempty"`
	LastDate   string                 `json:"last_date,omitempty"` // puede ser RFC3339 o epoch
	WebhookURL string                 `json:"webhook_url,omitempty"`
	Payload    map[string]interface{} `json:"payload,omitempty"`
}

// ParseTimestamp converts a string (RFC3339 or epoch) to epoch seconds.
func ParseTimestamp(ts string) (int64, error) {
	if ts == "" {
		return 0, nil
	}

	// Try to parse as epoch seconds first
	if epoch, err := strconv.ParseInt(ts, 10, 64); err == nil {
		return epoch, nil
	}

	// Try to parse as RFC3339
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Unix(), nil
	}

	// Try additional layouts
	layouts := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05+07:00",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.Unix(), nil
		}
	}

	return 0, fmt.Errorf("invalid timestamp format: %s", ts)
}

// ToJob converts CreateJobRequest to Job.
func (r *CreateJobRequest) ToJob() (*Job, error) {
	job := &Job{
		ID:         r.ID,
		Type:       r.Type,
		CronExpr:   r.CronExpr,
		CreatedAt:  time.Now().Unix(),
		WebhookURL: r.WebhookURL,
		Payload:    r.Payload,
	}

	// Generate UUID if not provided
	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	// Parse timestamp for unique job
	if r.Type == JobUnico && r.Timestamp != "" {
		ts, err := ParseTimestamp(r.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %v", err)
		}
		job.Timestamp = &ts
	}

	// Parse timestamp for recurring job (first execution)
	if r.Type == JobRecurrente && r.Timestamp != "" {
		ts, err := ParseTimestamp(r.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %v", err)
		}
		job.CreatedAt = ts
	}

	// Parse last_date for recurring job
	if r.Type == JobRecurrente && r.LastDate != "" {
		ld, err := ParseTimestamp(r.LastDate)
		if err != nil {
			return nil, fmt.Errorf("invalid last_date: %v", err)
		}
		job.LastDate = &ld
	}

	return job, nil
}
