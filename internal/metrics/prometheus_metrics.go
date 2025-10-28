package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// PrometheusMetrics holds native Prometheus metrics for immediate visibility
type PrometheusMetrics struct {
	// Job metrics
	JobsTotal            prometheus.Counter
	JobsExecuted         prometheus.Counter
	JobsFailed           prometheus.Counter
	JobExecutionDuration prometheus.Histogram
	JobsActive           prometheus.Gauge

	// Slot metrics
	SlotsTotal             prometheus.Gauge
	SlotsActive            prometheus.Gauge
	SlotsProcessed         prometheus.Counter
	SlotProcessingDuration prometheus.Histogram

	// Worker metrics
	WorkerActive         prometheus.Gauge
	WorkerProcessingTime prometheus.Histogram
	WorkerErrors         prometheus.Counter

	// Cluster metrics
	ClusterNodes         prometheus.Gauge
	ClusterLeaderChanges prometheus.Counter
	RaftOperations       prometheus.Counter
	RaftErrors           prometheus.Counter

	// HTTP metrics
	HTTPRequests        prometheus.Counter
	HTTPRequestDuration prometheus.Histogram
	HTTPErrors          prometheus.Counter
}

// NewPrometheusMetrics creates native Prometheus metrics
func NewPrometheusMetrics() *PrometheusMetrics {
	return &PrometheusMetrics{
		// Job metrics
		JobsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_jobs_total",
			Help: "Total number of jobs created",
		}),
		JobsExecuted: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_jobs_executed_total",
			Help: "Total number of jobs executed successfully",
		}),
		JobsFailed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_jobs_failed_total",
			Help: "Total number of jobs that failed execution",
		}),
		JobExecutionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "scheduled_db_job_execution_duration_seconds",
			Help:    "Duration of job execution in seconds",
			Buckets: []float64{0.01, 0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0, 300.0},
		}),
		JobsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "scheduled_db_jobs_active",
			Help: "Number of currently active jobs",
		}),

		// Slot metrics
		SlotsTotal: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "scheduled_db_slots_total",
			Help: "Total number of time slots",
		}),
		SlotsActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "scheduled_db_slots_active",
			Help: "Number of active slots with pending jobs",
		}),
		SlotsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_slots_processed_total",
			Help: "Total number of slots processed",
		}),
		SlotProcessingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "scheduled_db_slot_processing_duration_seconds",
			Help:    "Duration of slot processing in seconds",
			Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0, 10.0},
		}),

		// Worker metrics
		WorkerActive: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "scheduled_db_worker_active",
			Help: "Whether the worker is currently active (1) or stopped (0)",
		}),
		WorkerProcessingTime: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "scheduled_db_worker_processing_time_seconds",
			Help:    "Time spent processing jobs by the worker",
			Buckets: []float64{0.001, 0.01, 0.1, 1.0, 5.0, 10.0, 30.0},
		}),
		WorkerErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_worker_errors_total",
			Help: "Total number of worker errors",
		}),

		// Cluster metrics
		ClusterNodes: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "scheduled_db_cluster_nodes",
			Help: "Number of nodes in the cluster",
		}),
		ClusterLeaderChanges: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_cluster_leader_changes_total",
			Help: "Total number of leader changes",
		}),
		RaftOperations: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_raft_operations_total",
			Help: "Total number of Raft operations",
		}),
		RaftErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_raft_errors_total",
			Help: "Total number of Raft errors",
		}),

		// HTTP metrics
		HTTPRequests: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_http_requests_total",
			Help: "Total number of HTTP requests",
		}),
		HTTPRequestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "scheduled_db_http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0, 10.0, 30.0},
		}),
		HTTPErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "scheduled_db_http_errors_total",
			Help: "Total number of HTTP errors",
		}),
	}
}

// Global Prometheus metrics instance
var GlobalPrometheusMetrics *PrometheusMetrics

// InitializePrometheusMetrics initializes the global Prometheus metrics
func InitializePrometheusMetrics() {
	GlobalPrometheusMetrics = NewPrometheusMetrics()
	
	// Initialize with some test values to make them visible
	GlobalPrometheusMetrics.JobsTotal.Add(0)
	GlobalPrometheusMetrics.JobsActive.Set(0)
	GlobalPrometheusMetrics.SlotsTotal.Set(0)
	GlobalPrometheusMetrics.ClusterNodes.Set(1) // Will be updated dynamically
	GlobalPrometheusMetrics.WorkerActive.Set(1) // Worker is active
}
