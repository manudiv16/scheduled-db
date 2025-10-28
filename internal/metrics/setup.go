package metrics

import (
	"context"
	"fmt"
	"os"
	"time"

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
func Setup(ctx context.Context, config *Config) (*metric.MeterProvider, func(), *prometheus.Exporter, error) {
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
		return nil, nil, nil, fmt.Errorf("failed to create resource: %w", err)
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
		return nil, nil, nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create Prometheus exporter for local metrics endpoint
	prometheusExporter, err := prometheus.New()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
	}

	// Create meter provider with both OTLP and Prometheus exporters
	meterProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(metric.NewPeriodicReader(otlpExporter, metric.WithInterval(15*time.Second))),
		metric.WithReader(prometheusExporter),
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

		// Shutdown Prometheus exporter
		if err := prometheusExporter.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down Prometheus exporter: %v\n", err)
		}

		// Shutdown meter provider
		if err := meterProvider.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down meter provider: %v\n", err)
		}
	}

	// Return the Prometheus exporter so it can be used to serve metrics
	return meterProvider, cleanup, prometheusExporter, nil

}

// InitializeWithOTLP sets up OpenTelemetry with OTLP exporter only
func InitializeWithOTLP(ctx context.Context, config *Config) (*Metrics, func(), *prometheus.Exporter, error) {
	// Setup OpenTelemetry
	_, cleanup, prometheusExporter, err := Setup(ctx, config)
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

	return metrics, cleanup, prometheusExporter, nil
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
