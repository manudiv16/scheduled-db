package metrics

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// HTTPMiddleware creates a middleware that captures HTTP request metrics
func HTTPMiddleware(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Create a response writer wrapper to capture status code
			rw := &responseWriter{
				ResponseWriter: w,
				statusCode:     200, // default status code
			}

			// Extract endpoint pattern from mux route
			route := mux.CurrentRoute(r)
			var endpoint string
			if route != nil {
				if pathTemplate, err := route.GetPathTemplate(); err == nil {
					endpoint = pathTemplate
				}
			}
			if endpoint == "" {
				endpoint = r.URL.Path
			}

			// Clean up endpoint for metrics (remove IDs and query parameters)
			endpoint = cleanEndpoint(endpoint)

			// Process the request
			next.ServeHTTP(rw, r)

			// Record metrics
			duration := time.Since(start)
			ctx := context.Background()

			metrics.IncrementHTTPRequests(ctx, r.Method, endpoint, rw.statusCode)
			metrics.RecordHTTPRequestDuration(ctx, duration, r.Method, endpoint)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.WriteHeader(200)
	}
	return rw.ResponseWriter.Write(b)
}

// cleanEndpoint normalizes endpoint paths for consistent metrics
func cleanEndpoint(endpoint string) string {
	// Remove query parameters
	if idx := strings.Index(endpoint, "?"); idx != -1 {
		endpoint = endpoint[:idx]
	}

	// Replace common ID patterns with placeholders
	parts := strings.Split(endpoint, "/")
	for i, part := range parts {
		// Check if part looks like a UUID or ID
		if isID(part) {
			parts[i] = "{id}"
		}
	}

	return strings.Join(parts, "/")
}

// isID checks if a string looks like an ID (UUID, numeric, etc.)
func isID(s string) bool {
	if s == "" {
		return false
	}

	// Check if it's a number
	if _, err := strconv.Atoi(s); err == nil {
		return true
	}

	// Check if it's a UUID (basic pattern matching)
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		return true
	}

	// Check if it's a long alphanumeric string (likely an ID)
	if len(s) > 10 {
		alphanumeric := true
		for _, r := range s {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
				alphanumeric = false
				break
			}
		}
		if alphanumeric {
			return true
		}
	}

	return false
}
