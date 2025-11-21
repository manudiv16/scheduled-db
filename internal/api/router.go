package api

import (
	"net/http"

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
	router.Use(loggingMiddleware)

	return router
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

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
