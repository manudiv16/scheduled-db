package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

// Setup initializes OpenTelemetry with Prometheus exporter
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

	// Create Prometheus exporter
	exporter, err := prometheus.New()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
	}

	// Create meter provider with Prometheus exporter
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(exporter),
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
