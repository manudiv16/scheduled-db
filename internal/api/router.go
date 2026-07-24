package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/metrics"

	"github.com/gorilla/mux"
)

// NewRouter creates a new router with all API routes and middleware.
func NewRouter(handlers *Handlers, maxBodySize int64, authToken string) *mux.Router {
	router := mux.NewRouter()

	// API routes
	api := router.PathPrefix("/").Subrouter()
	api.HandleFunc("/jobs", handlers.CreateJob).Methods("POST")
	api.HandleFunc("/jobs", handlers.ListJobsByStatus).Methods("GET")
	api.HandleFunc("/jobs/{id}", handlers.GetJob).Methods("GET")
	api.HandleFunc("/jobs/{id}", handlers.DeleteJob).Methods("DELETE")
	api.HandleFunc("/jobs/{id}/status", handlers.GetJobStatus).Methods("GET")
	api.HandleFunc("/jobs/{id}/executions", handlers.GetJobExecutions).Methods("GET")
	api.HandleFunc("/jobs/{id}/cancel", handlers.CancelJob).Methods("POST")
	api.HandleFunc("/health", handlers.Health).Methods("GET")
	api.HandleFunc("/join", authMiddleware(authToken, handlers.JoinCluster)).Methods("POST")
	api.HandleFunc("/debug/cluster", handlers.ClusterDebug).Methods("GET")

	// Add CORS middleware
	router.Use(corsMiddleware)
	router.Use(metricsMiddleware)
	router.Use(loggingMiddleware)

	// Add request body size limiter middleware
	if maxBodySize > 0 {
		router.Use(maxBodySizeMiddleware(maxBodySize))
	}

	return router
}

// statusRecorder wraps http.ResponseWriter to capture the status code
// actually written by the handler, defaulting to 200 if WriteHeader is
// never called explicitly (matching net/http's own default behavior).
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// authMiddleware protects a handler with shared-secret Bearer token authentication.
func authMiddleware(expectedToken string, next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if expectedToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(auth, "Bearer ")
		if token != expectedToken {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// maxBodySizeMiddleware limits the request body size using http.MaxBytesReader.
func maxBodySizeMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		if globalMetrics := metrics.GetGlobalMetrics(); globalMetrics != nil {
			globalMetrics.IncrementHTTPRequests(context.Background(), r.Method, r.URL.Path, rec.status)
			globalMetrics.RecordHTTPRequestDuration(context.Background(), duration, r.Method, r.URL.Path)
		}
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec, ok := w.(*statusRecorder)
		if !ok {
			rec = &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		}
		start := time.Now()

		next.ServeHTTP(rec, r)

		logger.Info("%s %s %d %s", r.Method, r.URL.RequestURI(), rec.status, time.Since(start))
	})
}
