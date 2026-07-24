package api

import (
	"context"
	"net/http"
	"time"

	"scheduled-db/internal/logger"
	"scheduled-db/internal/metrics"

	"github.com/gorilla/mux"
)

func NewRouter(handlers *Handlers) *mux.Router {
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
	api.HandleFunc("/join", handlers.JoinCluster).Methods("POST")
	api.HandleFunc("/debug/cluster", handlers.ClusterDebug).Methods("GET")

	// Add CORS middleware
	router.Use(corsMiddleware)
	router.Use(metricsMiddleware)
	router.Use(loggingMiddleware)

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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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
