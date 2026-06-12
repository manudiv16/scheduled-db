package types

// WebhookPayload is the payload sent to a job's webhook URL.
type WebhookPayload struct {
	JobID     string                 `json:"job_id"`
	Type      JobType                `json:"type"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}
