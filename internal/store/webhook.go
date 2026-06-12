//go:build !wasm

package store

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"scheduled-db/internal/logger"
)

func ExecuteWebhook(job *Job) {
	if job.WebhookURL == "" {
		return
	}

	go func() {
		payload := WebhookPayload{
			JobID:     job.ID,
			Type:      job.Type,
			Timestamp: time.Now().Unix(),
			Data:      job.Payload,
		}

		jsonData, err := json.Marshal(payload)
		if err != nil {
			logger.JobError(job.ID, "webhook marshal failed: %v", err)
			return
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Post(job.WebhookURL, "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			logger.JobError(job.ID, "webhook request failed: %v", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			logger.JobError(job.ID, "webhook returned status %d", resp.StatusCode)
		}
	}()
}
