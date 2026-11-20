// Package telemetry sets up OpenTelemetry tracing and the Prometheus registry.
package telemetry

import (
	"context"
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Setup installs the global tracer provider and propagator. When otlpEndpoint is
// empty, spans are still created (so trace ids propagate and appear in logs) but
// not exported. The returned func flushes and shuts the provider down.
func Setup(ctx context.Context, service, version, env, otlpEndpoint string, sampleRatio float64) (func(context.Context) error, error) {
	res := resource.NewSchemaless(
		semconv.ServiceName(service),
		semconv.ServiceVersion(version),
		semconv.DeploymentEnvironment(env),
	)
	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))),
	}
	if otlpEndpoint != "" {
		endpoint := strings.TrimPrefix(strings.TrimPrefix(otlpEndpoint, "http://"), "https://")
		exp, err := otlptracegrpc.New(ctx,
			otlptracegrpc.WithEndpoint(endpoint),
			otlptracegrpc.WithInsecure(), // collector is expected on the private network
		)
		if err != nil {
			return nil, fmt.Errorf("telemetry: otlp exporter: %w", err)
		}
		opts = append(opts, sdktrace.WithBatcher(exp))
	}
	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))
	return tp.Shutdown, nil
}

// NewRegistry returns a Prometheus registry with Go runtime and process collectors.
func NewRegistry() *prometheus.Registry {
	r := prometheus.NewRegistry()
	r.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return r
}
