package metrics

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

// Config holds configuration for OpenTelemetry setup
type Config struct {
	ServiceName    string
	ServiceVersion string
	NodeID         string
	Environment    string
	MetricsPort    int
	MetricsPath    string
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		ServiceName:    "scheduled-db",
		ServiceVersion: "1.0.0",
		Environment:    "production",
		MetricsPort:    9090,
		MetricsPath:    "/metrics",
	}
}

// Setup initializes OpenTelemetry with OTLP exporter
func Setup(ctx context.Context, config *Config) (*metric.MeterProvider, func(), error) {
	// Create resource with service information
	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", config.ServiceName),
			attribute.String("service.version", config.ServiceVersion),
			attribute.String("service.instance.id", config.NodeID),
			attribute.String("deployment.environment", config.Environment),
		),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Get OTLP endpoint from environment or use default
	otlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otlpEndpoint == "" {
		otlpEndpoint = "otel-collector:4317" // gRPC endpoint without http://
	}

	// Create OTLP exporter
	otlpExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(otlpEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Also create Prometheus exporter for direct metrics endpoint
	promExporter, err := prometheus.New()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	// Create meter provider with both exporters
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(otlpExporter, metric.WithInterval(15*time.Second))),
		metric.WithReader(promExporter),
		metric.WithView(
			// Add custom views for better histograms
			metric.NewView(
				metric.Instrument{Name: "job_execution_duration_seconds"},
				metric.Stream{
					Aggregation: metric.AggregationExplicitBucketHistogram{
						Boundaries: []float64{0.01, 0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0, 300.0},
					},
				},
			),
			metric.NewView(
				metric.Instrument{Name: "http_request_duration_seconds"},
				metric.Stream{
					Aggregation: metric.AggregationExplicitBucketHistogram{
						Boundaries: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0, 10.0, 30.0},
					},
				},
			),
			metric.NewView(
				metric.Instrument{Name: "slot_processing_duration_seconds"},
				metric.Stream{
					Aggregation: metric.AggregationExplicitBucketHistogram{
						Boundaries: []float64{0.001, 0.01, 0.1, 0.5, 1.0, 5.0, 10.0},
					},
				},
			),
		),
	)

	// Set global meter provider
	otel.SetMeterProvider(meterProvider)

	// Create cleanup function
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// Shutdown OTLP exporter
		if err := otlpExporter.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down OTLP exporter: %v\n", err)
		}
		
		// Shutdown meter provider
		if err := meterProvider.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down meter provider: %v\n", err)
		}
	}

	return meterProvider, cleanup, nil
}

// StartMetricsServer starts an HTTP server to serve Prometheus metrics
func StartMetricsServer(config *Config) *http.Server {
	mux := http.NewServeMux()

	// Add Prometheus metrics endpoint
	mux.Handle(config.MetricsPath, promhttp.Handler())

	// Add health endpoint for metrics server
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"metrics"}`))
	})

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.MetricsPort),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		fmt.Printf("Starting metrics server on port %d\n", config.MetricsPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Metrics server error: %v\n", err)
		}
	}()

	return server
}

// InitializeWithPrometheus sets up OpenTelemetry with Prometheus and starts metrics server
func InitializeWithPrometheus(ctx context.Context, config *Config) (*Metrics, *http.Server, func(), error) {
	// Setup OpenTelemetry
	_, cleanup, err := Setup(ctx, config)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to setup OpenTelemetry: %w", err)
	}

	// Create metrics instance
	metrics, err := NewMetrics()
	if err != nil {
		cleanup()
		return nil, nil, nil, fmt.Errorf("failed to create metrics: %w", err)
	}

	fmt.Printf("✅ Metrics instance created successfully\n")

	// Initialize global instrumentation with the same metrics instance
	if err := initializeGlobalInstrumentationWithMetrics(metrics, config.NodeID); err != nil {
		cleanup()
		return nil, nil, nil, fmt.Errorf("failed to initialize global instrumentation: %w", err)
	}

	fmt.Printf("✅ Global instrumentation initialized successfully\n")

	// Initialize native Prometheus metrics for immediate visibility
	InitializePrometheusMetrics()
	fmt.Printf("✅ Prometheus metrics initialized\n")

	// Start periodic metrics updater
	go startPeriodicMetricsUpdater()
	fmt.Printf("✅ Periodic metrics updater started\n")

	// Test metrics by creating a simple counter
	if metrics.JobsTotal != nil {
		metrics.JobsTotal.Add(ctx, 0) // Initialize the metric
		fmt.Printf("✅ Test metric initialized\n")
	}

	// Start metrics server
	server := StartMetricsServer(config)

	// Enhanced cleanup function
	enhancedCleanup := func() {
		// Shutdown metrics server
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			fmt.Printf("Error shutting down metrics server: %v\n", err)
		}

		// Cleanup OpenTelemetry
		cleanup()
	}

	return metrics, server, enhancedCleanup, nil
}

// initializeGlobalInstrumentationWithMetrics initializes global instrumentation with provided metrics
func initializeGlobalInstrumentationWithMetrics(metrics *Metrics, nodeID string) error {
	// Set the global metrics instance
	SetGlobalMetrics(metrics)
	
	// Initialize global instrumentation helpers
	GlobalJobInstrumentation = NewJobInstrumentation(metrics)
	GlobalSlotInstrumentation = NewSlotInstrumentation(metrics)
	GlobalWorkerInstrumentation = NewWorkerInstrumentation(metrics)
	GlobalClusterInstrumentation = NewClusterInstrumentation(metrics, nodeID)
	GlobalDiscoveryInstrumentation = NewDiscoveryInstrumentation(metrics, "kubernetes")
	GlobalWebhookInstrumentation = NewWebhookInstrumentation(metrics)
	GlobalSystemInstrumentation = NewSystemInstrumentation(metrics)

	return nil
}

// startPeriodicMetricsUpdater updates cluster and system metrics periodically
func startPeriodicMetricsUpdater() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		updateClusterMetrics()
	}
}

// updateClusterMetrics updates cluster-related metrics
func updateClusterMetrics() {
	if GlobalPrometheusMetrics == nil {
		return
	}

	// Cluster nodes count will be updated dynamically by discovery
	// GlobalPrometheusMetrics.ClusterNodes.Set() - handled elsewhere
	
	// Update worker active status
	GlobalPrometheusMetrics.WorkerActive.Set(1)
}

// GetPrometheusMetrics returns the Prometheus metrics handler
func GetPrometheusMetrics() http.Handler {
	return promhttp.Handler()
}

// HealthCheck represents the health status of the metrics system
type HealthCheck struct {
	Status  string `json:"status"`
	Service string `json:"service"`
	Uptime  string `json:"uptime,omitempty"`
}

// GetHealthCheck returns the health status of the metrics system
func GetHealthCheck(startTime time.Time) *HealthCheck {
	return &HealthCheck{
		Status:  "ok",
		Service: "metrics",
		Uptime:  time.Since(startTime).String(),
	}
}
