package telemetry

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

// sentryEnabled records whether Init actually configured a DSN, so the
// middleware and capture helpers can skip work when it did not.
var sentryEnabled bool

// InitSentry configures error reporting. With no SENTRY_DSN it is a
// no-op and reports no error: error tracking is optional, and its
// absence must not stop the API from booting.
func InitSentry() error {
	dsn := os.Getenv("SENTRY_DSN")
	if dsn == "" {
		return nil
	}

	err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		Environment:      envOr("ENVIRONMENT", "development"),
		Release:          os.Getenv("APP_VERSION"),
		TracesSampleRate: floatEnv("SENTRY_TRACES_SAMPLE_RATE", 0.05),
		SampleRate:       floatEnv("SENTRY_SAMPLE_RATE", 1.0),
		BeforeSend:       scrub,
	})
	if err != nil {
		return err
	}

	sentryEnabled = true
	return nil
}

// SentryEnabled reports whether errors are actually being sent.
func SentryEnabled() bool { return sentryEnabled }

// scrub strips credentials from an event before it leaves the process.
// Sentry stores what it receives, so anything sensitive has to be
// removed here rather than filtered in the UI later.
func scrub(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event.Request != nil {
		for _, header := range []string{"Authorization", "Cookie", "X-Metrics-Token", "Stripe-Signature"} {
			delete(event.Request.Headers, header)
		}
		// Query strings can carry tokens from email links
		event.Request.QueryString = ""
		// Request bodies hold GraphQL variables: passwords on the
		// signup mutation, customer details everywhere else.
		event.Request.Data = ""
	}
	return event
}

// SentryMiddleware recovers panics and attaches request context to
// reported errors.
func SentryMiddleware(next http.Handler) http.Handler {
	if !sentryEnabled {
		return next
	}

	handler := sentryhttp.New(sentryhttp.Options{
		// chi's Recoverer already returns a 500 for panics; re-panicking
		// here lets it keep doing that after Sentry has recorded them.
		Repanic: true,
		Timeout: 2 * time.Second,
	})
	return handler.Handle(next)
}

// CaptureError reports an error with tags. Safe to call when Sentry is
// disabled.
func CaptureError(ctx context.Context, err error, tags map[string]string) {
	if err == nil || !sentryEnabled {
		return
	}

	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		hub = sentry.CurrentHub().Clone()
	}

	hub.WithScope(func(scope *sentry.Scope) {
		for k, v := range tags {
			scope.SetTag(k, v)
		}
		if traceID := TraceIDFromContext(ctx); traceID != "" {
			scope.SetTag("trace_id", traceID)
		}
		hub.CaptureException(err)
	})
}

// FlushSentry drains the send queue at shutdown.
func FlushSentry() {
	if sentryEnabled {
		sentry.Flush(2 * time.Second)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func floatEnv(key string, fallback float64) float64 {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil || value < 0 || value > 1 {
		return fallback
	}
	return value
}
