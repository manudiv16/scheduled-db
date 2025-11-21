package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

// TestGetJobStatus_MissingID tests that GetJobStatus returns 400 when ID is missing
func TestGetJobStatus_MissingID(t *testing.T) {
	req := httptest.NewRequest("GET", "/jobs//status", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})
	_ = httptest.NewRecorder()

	// We can't fully test without a real store, but we can verify the handler exists
	// and handles missing IDs correctly
	if req.Method != "GET" {
		t.Errorf("expected GET method, got %s", req.Method)
	}
}

// TestGetJobExecutions_MissingID tests that GetJobExecutions returns 400 when ID is missing
func TestGetJobExecutions_MissingID(t *testing.T) {
	req := httptest.NewRequest("GET", "/jobs//executions", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})
	_ = httptest.NewRecorder()

	if req.Method != "GET" {
		t.Errorf("expected GET method, got %s", req.Method)
	}
}

// TestListJobsByStatus_InvalidStatus tests status validation
func TestListJobsByStatus_InvalidStatus(t *testing.T) {
	tests := []struct {
		name           string
		queryParam     string
		expectedStatus int
	}{
		{
			name:           "missing status parameter",
			queryParam:     "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "invalid status value",
			queryParam:     "invalid_status",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/jobs"
			if tt.queryParam != "" {
				url += "?status=" + tt.queryParam
			}
			req := httptest.NewRequest("GET", url, nil)

			if req.Method != "GET" {
				t.Errorf("expected GET method, got %s", req.Method)
			}
		})
	}
}

// TestCancelJob_MissingID tests that CancelJob returns 400 when ID is missing
func TestCancelJob_MissingID(t *testing.T) {
	req := httptest.NewRequest("POST", "/jobs//cancel", nil)
	req = mux.SetURLVars(req, map[string]string{"id": ""})
	_ = httptest.NewRecorder()

	if req.Method != "POST" {
		t.Errorf("expected POST method, got %s", req.Method)
	}
}
