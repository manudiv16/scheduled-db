package api

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeError_IPv4(t *testing.T) {
	err := errors.New("connection refused to 192.168.1.100:7000")
	got := sanitizeError(err)
	if strings.Contains(got, "192.168.1.100") {
		t.Errorf("sanitizeError() should remove IP addresses, got %q", got)
	}
	if !strings.Contains(got, "[addr]") {
		t.Errorf("sanitizeError() should contain [addr] placeholder, got %q", got)
	}
}

func TestSanitizeError_IPv4NoPort(t *testing.T) {
	err := errors.New("failed to connect to 10.0.0.5")
	got := sanitizeError(err)
	if strings.Contains(got, "10.0.0.5") {
		t.Errorf("sanitizeError() should remove IP addresses, got %q", got)
	}
}

func TestSanitizeError_FilePath(t *testing.T) {
	err := errors.New("open /var/data/scheduled-db/logs.db: permission denied")
	got := sanitizeError(err)
	if strings.Contains(got, "/var/data/") {
		t.Errorf("sanitizeError() should remove file paths, got %q", got)
	}
	if strings.Contains(got, ".db") {
		t.Errorf("sanitizeError() should remove database file extensions in paths, got %q", got)
	}
}

func TestSanitizeError_RaftInfo(t *testing.T) {
	err := errors.New("raft: failed to connect to peer at 10.0.1.5:7000")
	got := sanitizeError(err)
	if strings.Contains(got, "10.0.1.5") {
		t.Errorf("sanitizeError() should remove IP addresses, got %q", got)
	}
}

func TestSanitizeError_ServiceName(t *testing.T) {
	err := errors.New("scheduled-db-0.scheduled-db.default.svc.cluster.local:7000 connection failed")
	got := sanitizeError(err)
	if strings.Contains(got, "scheduled-db-0") {
		t.Errorf("sanitizeError() should remove service names, got %q", got)
	}
	if strings.Contains(got, ".svc.cluster.local") {
		t.Errorf("sanitizeError() should remove K8s domain, got %q", got)
	}
}

func TestSanitizeError_Nil(t *testing.T) {
	got := sanitizeError(nil)
	if got != "" {
		t.Errorf("sanitizeError(nil) = %q, want empty string", got)
	}
}

func TestSanitizeError_NoSensitiveInfo(t *testing.T) {
	err := errors.New("invalid timestamp format")
	got := sanitizeError(err)
	want := "invalid timestamp format"
	if got != want {
		t.Errorf("sanitizeError() = %q, want %q", got, want)
	}
}

func TestSanitizeError_MultipleIPs(t *testing.T) {
	err := errors.New("failed to connect from 10.0.0.1 to 192.168.1.100:7000")
	got := sanitizeError(err)
	if strings.Contains(got, "10.0.0.1") || strings.Contains(got, "192.168.1.100") {
		t.Errorf("sanitizeError() should remove all IP addresses, got %q", got)
	}
}

func TestSanitizeError_LocalhostIP(t *testing.T) {
	err := errors.New("connection to 127.0.0.1:8080 refused")
	got := sanitizeError(err)
	if strings.Contains(got, "127.0.0.1") {
		t.Errorf("sanitizeError() should remove localhost IP, got %q", got)
	}
}

func TestSafeErrorMessage_ClientError(t *testing.T) {
	err := errors.New("invalid timestamp: bad format")
	got := safeErrorMessage("Job validation failed", err, true)
	want := "Job validation failed: invalid timestamp: bad format"
	if got != want {
		t.Errorf("safeErrorMessage() = %q, want %q", got, want)
	}
}

func TestSafeErrorMessage_ServerError(t *testing.T) {
	err := errors.New("failed to apply command: raft: no leader")
	got := safeErrorMessage("Failed to create job", err, false)
	want := "Failed to create job"
	if got != want {
		t.Errorf("safeErrorMessage() = %q, want %q", got, want)
	}
}

func TestSafeErrorMessage_ClientErrorWithSensitiveInfo(t *testing.T) {
	err := errors.New("invalid job data: connection to 192.168.1.1:8080 failed")
	got := safeErrorMessage("Invalid job data", err, true)
	if strings.Contains(got, "192.168.1.1") {
		t.Errorf("safeErrorMessage() should sanitize IPs in client errors, got %q", got)
	}
}

func TestSafeErrorMessage_ClientErrorWithNoSensitiveInfo(t *testing.T) {
	err := errors.New("invalid cron expression")
	got := safeErrorMessage("Job validation failed", err, true)
	want := "Job validation failed: invalid cron expression"
	if got != want {
		t.Errorf("safeErrorMessage() = %q, want %q", got, want)
	}
}
