package store

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type JobType string

const (
	JobUnico      JobType = "unico"
	JobRecurrente JobType = "recurrente"
)

type Job struct {
	ID        string  `json:"id"`
	Type      JobType `json:"type"`
	Timestamp *int64  `json:"timestamp,omitempty"`       // epoch seconds para unico
	CronExpr  string  `json:"cron_expression,omitempty"` // para recurrente
	LastDate  *int64  `json:"last_date,omitempty"`       // optional epoch seconds
	CreatedAt int64   `json:"created_at"`
}

// SlotData representa un slot persistido en Raft
type SlotData struct {
	Key     int64    `json:"key"`
	MinTime int64    `json:"min_time"`
	MaxTime int64    `json:"max_time"`
	JobIDs  []string `json:"job_ids"`
}

type CreateJobRequest struct {
	ID        string  `json:"id,omitempty"`
	Type      JobType `json:"type"`
	Timestamp string  `json:"timestamp,omitempty"` // puede ser RFC3339 o epoch
	CronExpr  string  `json:"cron_expression,omitempty"`
	LastDate  string  `json:"last_date,omitempty"` // puede ser RFC3339 o epoch
}

// ParseTimestamp convierte string (RFC3339 o epoch) a epoch seconds
func ParseTimestamp(ts string) (int64, error) {
	if ts == "" {
		return 0, nil
	}

	// Intentar parsear como epoch seconds primero
	if epoch, err := strconv.ParseInt(ts, 10, 64); err == nil {
		return epoch, nil
	}

	// Intentar parsear como RFC3339
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Unix(), nil
	}

	// Intentar parsear con formato de timezone offset (+0200, -0500, etc)
	layouts := []string{
		"2006-01-02T15:04:05Z07:00", // Formato con timezone offset
		"2006-01-02T15:04:05-07:00", // Formato con timezone offset negativo
		"2006-01-02T15:04:05+07:00", // Formato con timezone offset positivo
		"2006-01-02 15:04:05",       // Formato simple sin timezone
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.Unix(), nil
		}
	}

	return 0, fmt.Errorf("invalid timestamp format: %s", ts)
}

// ToJob convierte CreateJobRequest a Job
func (r *CreateJobRequest) ToJob() (*Job, error) {
	job := &Job{
		ID:        r.ID,
		Type:      r.Type,
		CronExpr:  r.CronExpr,
		CreatedAt: time.Now().Unix(),
	}

	// Generar UUID si no se proporciona
	if job.ID == "" {
		job.ID = uuid.New().String()
	}

	// Parsear timestamp para job único
	if r.Type == JobUnico && r.Timestamp != "" {
		ts, err := ParseTimestamp(r.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %v", err)
		}
		job.Timestamp = &ts
	}

	// Parsear timestamp para job recurrente (primera ejecución)
	if r.Type == JobRecurrente && r.Timestamp != "" {
		ts, err := ParseTimestamp(r.Timestamp)
		if err != nil {
			return nil, fmt.Errorf("invalid timestamp: %v", err)
		}
		job.CreatedAt = ts
	}

	// Parsear last_date para job recurrente
	if r.Type == JobRecurrente && r.LastDate != "" {
		ld, err := ParseTimestamp(r.LastDate)
		if err != nil {
			return nil, fmt.Errorf("invalid last_date: %v", err)
		}
		job.LastDate = &ld
	}

	return job, nil
}

// Validate valida un job
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
		// Validate cron expression
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(j.CronExpr); err != nil {
			return fmt.Errorf("invalid cron expression: %v", err)
		}
	}

	return nil
}

// ToBytes serializa el job a bytes
func (j *Job) ToBytes() ([]byte, error) {
	return json.Marshal(j)
}

// JobFromBytes deserializa bytes a job
func JobFromBytes(data []byte) (*Job, error) {
	var job Job
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, err
	}
	return &job, nil
}
