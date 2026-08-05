package telemetry

import (
	"fmt"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// newResource describes this process to the telemetry backend.
//
// The schema URL is taken from resource.Default() rather than from the
// semconv package we import. resource.Merge refuses to combine two
// resources with different schema URLs, and the SDK's default tracks
// whatever semconv version the SDK ships — so pinning our own here
// makes both exporters fail to start the moment the SDK is upgraded.
// That failure is silent apart from a warning, which is the worst kind:
// telemetry looks configured and sends nothing.
func newResource() (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(envOr("SERVICE_NAME", "ownstall-api")),
		attribute.String("environment", envOr("ENVIRONMENT", "development")),
	}
	if version := os.Getenv("APP_VERSION"); version != "" {
		attrs = append(attrs, semconv.ServiceVersion(version))
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(resource.Default().SchemaURL(), attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}
	return res, nil
}
