package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Tracer is the process tracer. Before InitTracing it is a no-op
// tracer from the global provider, so instrumented code can call
// StartSpan unconditionally.
var Tracer trace.Tracer = otel.Tracer("ownstall")

// defaultSampleRatio keeps 10% of traces. Full sampling on a free-tier
// backend burns the monthly event budget in hours.
const defaultSampleRatio = 0.1

// InitTracing wires up OTLP export when OTLP_ENDPOINT is set, and
// otherwise leaves tracing as a no-op. It returns a shutdown function
// that flushes pending spans.
func InitTracing(ctx context.Context) (func(context.Context), error) {
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "ownstall-api"
	}

	endpoint := os.Getenv("OTLP_ENDPOINT")
	if endpoint == "" {
		// No backend configured. Set the propagator anyway so inbound
		// traceparent headers still flow through to logs.
		otel.SetTextMapPropagator(defaultPropagator())
		Tracer = otel.Tracer(serviceName)
		return func(context.Context) {}, nil
	}

	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(endpoint)}
	// Local collectors are plaintext; hosted ones are not. Opt in
	// explicitly rather than defaulting to insecure transport.
	if os.Getenv("OTLP_INSECURE") == "true" {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if headers := otlpHeaders(); len(headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(headers))
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("OTLP exporter: %w", err)
	}

	res, err := newResource()
	if err != nil {
		return nil, fmt.Errorf("trace resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		// ParentBased keeps a sampled trace intact end to end instead
		// of dropping half its spans on the way through.
		sdktrace.WithSampler(sdktrace.ParentBased(
			sdktrace.TraceIDRatioBased(sampleRatio()),
		)),
	)

	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(defaultPropagator())
	Tracer = provider.Tracer(serviceName)

	return func(shutdownCtx context.Context) {
		if err := provider.Shutdown(shutdownCtx); err != nil {
			Log.Warn("tracer shutdown failed: " + err.Error())
		}
	}, nil
}

func defaultPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func sampleRatio() float64 {
	raw := os.Getenv("OTLP_SAMPLE_RATIO")
	if raw == "" {
		return defaultSampleRatio
	}
	ratio, err := strconv.ParseFloat(raw, 64)
	if err != nil || ratio < 0 || ratio > 1 {
		return defaultSampleRatio
	}
	return ratio
}

// otlpHeaders parses OTLP_HEADERS ("key=value,key2=value2"), which is
// how hosted backends carry their auth token.
func otlpHeaders() map[string]string {
	raw := os.Getenv("OTLP_HEADERS")
	if raw == "" {
		return nil
	}

	headers := make(map[string]string)
	for _, pair := range splitAndTrim(raw, ",") {
		key, value, found := cut(pair, "=")
		if found && key != "" {
			headers[key] = value
		}
	}
	return headers
}

// StartSpan opens a child span of whatever is current in ctx.
func StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	if len(attrs) == 0 {
		return Tracer.Start(ctx, name)
	}
	return Tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// TraceIDFromContext returns the current trace id, or "" when the
// request is not being traced. Used to stitch logs to traces.
func TraceIDFromContext(ctx context.Context) string {
	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.HasTraceID() {
		return ""
	}
	return spanCtx.TraceID().String()
}
