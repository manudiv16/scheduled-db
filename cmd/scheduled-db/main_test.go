package main

import (
	"testing"
)

func TestParseMemoryLimit(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{
			name:  "parse GB",
			input: "2GB",
			want:  2 * 1024 * 1024 * 1024,
		},
		{
			name:  "parse gb lowercase",
			input: "2gb",
			want:  2 * 1024 * 1024 * 1024,
		},
		{
			name:  "parse MB",
			input: "500MB",
			want:  500 * 1024 * 1024,
		},
		{
			name:  "parse KB",
			input: "1024KB",
			want:  1024 * 1024,
		},
		{
			name:  "parse bytes",
			input: "1073741824",
			want:  1073741824,
		},
		{
			name:  "parse with spaces",
			input: "  2GB  ",
			want:  2 * 1024 * 1024 * 1024,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid format",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "negative value",
			input:   "-100MB",
			wantErr: true,
		},
		{
			name:    "zero value",
			input:   "0GB",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseMemoryLimit(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMemoryLimit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseMemoryLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetectMemoryLimit(t *testing.T) {
	tests := []struct {
		name           string
		explicitLimit  string
		memoryPercent  float64
		wantPositive   bool
		wantFromConfig bool
	}{
		{
			name:           "explicit limit",
			explicitLimit:  "2GB",
			memoryPercent:  50.0,
			wantPositive:   true,
			wantFromConfig: true,
		},
		{
			name:          "auto-detect with default percent",
			explicitLimit: "",
			memoryPercent: 50.0,
			wantPositive:  true,
		},
		{
			name:          "auto-detect with custom percent",
			explicitLimit: "",
			memoryPercent: 75.0,
			wantPositive:  true,
		},
		{
			name:          "auto-detect with invalid percent",
			explicitLimit: "",
			memoryPercent: -10.0,
			wantPositive:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMemoryLimit(tt.explicitLimit, tt.memoryPercent)
			if tt.wantPositive && got <= 0 {
				t.Errorf("DetectMemoryLimit() = %v, want positive value", got)
			}
			if tt.wantFromConfig && tt.explicitLimit != "" {
				expected, _ := ParseMemoryLimit(tt.explicitLimit)
				if got != expected {
					t.Errorf("DetectMemoryLimit() = %v, want %v", got, expected)
				}
			}
		})
	}
}
