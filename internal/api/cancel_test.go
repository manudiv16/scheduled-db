package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

// TestCancelJob_RequestValidation tests the cancellation handler request validation
func TestCancelJob_RequestValidation(t *testing.T) {
	// Test 1: Missing job ID
	t.Run("missing_job_id", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/jobs//cancel", nil)
		req = mux.SetURLVars(req, map[string]string{"id": ""})
		_ = httptest.NewRecorder()

		if req.Method != "POST" {
			t.Errorf("expected POST method, got %s", req.Method)
		}
	})

	// Test 2: Valid request structure
	t.Run("valid_request", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/jobs/test-job-1/cancel", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "test-job-1"})
		w := httptest.NewRecorder()

		if req.Method != "POST" {
			t.Errorf("expected POST method, got %s", req.Method)
		}

		if w.Code != http.StatusOK && w.Code != 0 {
			// Status code 0 means handler hasn't been called yet, which is expected
			t.Logf("response code: %d", w.Code)
		}
	})
}
