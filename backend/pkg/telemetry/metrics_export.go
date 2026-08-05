package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	prombridge "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/sdk/metric"
)

// defaultMetricInterval is how often metrics are pushed. Grafana
// Cloud's free tier bills by series and ingest, and a 60s resolution
// is plenty for dashboards and alerting on a service this size.
const defaultMetricInterval = 60 * time.Second

// InitMetricsExport pushes metrics to an OTLP backend.
//
// Nothing scrapes a Railway service: it has no stable private address
// and Prometheus' Go client cannot remote-write. So instead of being
// pulled, the process pushes — and it pushes the *same* registry that
// /metrics serves, via the OpenTelemetry Prometheus bridge. One set of
// instrumentation feeds both a local scrape and a hosted backend, so
// the numbers can never disagree.
//
// Returns a no-op shutdown when OTLP_ENDPOINT is unset.
func InitMetricsExport(ctx context.Context) (func(context.Context), error) {
	endpoint := os.Getenv("OTLP_ENDPOINT")
	if endpoint == "" {
		return func(context.Context) {}, nil
	}

	exporter, err := newMetricExporter(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	res, err := newResource()
	if err != nil {
		return nil, fmt.Errorf("metric resource: %w", err)
	}

	// The bridge reads the default Prometheus registry, so every
	// promauto counter declared in metrics.go is exported without
	// being redefined here.
	producer := prombridge.NewMetricProducer()

	reader := metric.NewPeriodicReader(exporter,
		metric.WithInterval(metricInterval()),
		metric.WithProducer(producer),
	)

	provider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)
	otel.SetMeterProvider(provider)

	Log.Info("metrics export enabled (endpoint " + endpoint + ", every " + metricInterval().String() + ")")

	return func(shutdownCtx context.Context) {
		// Forces a final collection, so the last minute of counters is
		// not lost on every deploy.
		if err := provider.Shutdown(shutdownCtx); err != nil {
			Log.Warn("metrics exporter shutdown failed: " + err.Error())
		}
	}, nil
}

// newMetricExporter builds the exporter for the configured protocol.
// gRPC is the default to match tracing; hosted backends (Grafana
// Cloud among them) document an HTTP OTLP gateway, hence the switch.
func newMetricExporter(ctx context.Context, endpoint string) (metric.Exporter, error) {
	isURL := strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://")
	insecure := os.Getenv("OTLP_INSECURE") == "true"
	headers := otlpHeaders()

	if strings.EqualFold(os.Getenv("OTLP_PROTOCOL"), "http") {
		opts := []otlpmetrichttp.Option{}
		if isURL {
			// WithEndpointURL takes the full base URL and appends the
			// signal path, which is how Grafana Cloud's gateway is
			// documented ("https://otlp-gateway-….grafana.net/otlp").
			opts = append(opts, otlpmetrichttp.WithEndpointURL(endpoint))
		} else {
			opts = append(opts, otlpmetrichttp.WithEndpoint(endpoint))
			if insecure {
				opts = append(opts, otlpmetrichttp.WithInsecure())
			}
		}
		if len(headers) > 0 {
			opts = append(opts, otlpmetrichttp.WithHeaders(headers))
		}
		return otlpmetrichttp.New(ctx, opts...)
	}

	opts := []otlpmetricgrpc.Option{}
	if isURL {
		opts = append(opts, otlpmetricgrpc.WithEndpointURL(endpoint))
	} else {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(endpoint))
		if insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
	}
	if len(headers) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(headers))
	}
	return otlpmetricgrpc.New(ctx, opts...)
}

func metricInterval() time.Duration {
	raw := os.Getenv("OTLP_METRIC_INTERVAL_SECONDS")
	if raw == "" {
		return defaultMetricInterval
	}
	seconds, err := strconv.Atoi(raw)
	if err != nil || seconds <= 0 {
		return defaultMetricInterval
	}
	return time.Duration(seconds) * time.Second
}
