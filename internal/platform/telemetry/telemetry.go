// Package telemetry sets up OpenTelemetry tracing and the Prometheus registry.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// SetupTracing installs a global tracer provider. Spans are exported over
// OTLP/HTTP only when OTEL_EXPORTER_OTLP_ENDPOINT (or the traces-specific
// variant) is set; otherwise tracing stays a no-op apart from propagation.
// The returned function flushes and stops the exporter.
func SetupTracing(ctx context.Context, service, version, env string) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" && os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") == "" {
		return func(context.Context) error { return nil }, nil
	}

	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", service),
		attribute.String("service.version", version),
		attribute.String("deployment.environment.name", env),
	))
	if err != nil && !errors.Is(err, resource.ErrSchemaURLConflict) {
		return nil, fmt.Errorf("otel resource: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplingRatio()))),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// samplingRatio reads OTEL_TRACES_SAMPLER_ARG, defaulting to 1 (sample all).
func samplingRatio() float64 {
	var r float64
	if _, err := fmt.Sscanf(os.Getenv("OTEL_TRACES_SAMPLER_ARG"), "%g", &r); err != nil || r < 0 || r > 1 {
		return 1
	}
	return r
}

// NewRegistry returns a Prometheus registry with Go runtime and process metrics.
func NewRegistry() *prometheus.Registry {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return reg
}
