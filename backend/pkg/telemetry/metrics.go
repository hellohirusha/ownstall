package telemetry

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metric labels must stay low-cardinality: every distinct combination
// is a separate time series held in memory and billed by the backend.
// That is why paths are route patterns, never raw URLs.
var (
	HTTPRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "HTTP request duration in seconds",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"method", "path", "status_code"})

	HTTPRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status_code"})

	// Business metrics. tenant_id is deliberately absent: it is
	// unbounded, and a per-tenant breakdown belongs in the database,
	// not in a metrics label.
	OrdersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "orders_total",
		Help: "Orders by status",
	}, []string{"status"})

	OrderValueUSD = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "order_value_usd",
		Help:    "Order value in USD",
		Buckets: []float64{5, 10, 25, 50, 100, 250, 500, 1000},
	})

	EmailsSentTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "emails_sent_total",
		Help: "Emails sent by status",
	}, []string{"status"})

	TicketsCreatedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tickets_created_total",
		Help: "Support tickets created",
	})

	ProductionOrdersTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "production_orders_total",
		Help: "Production queue transitions by stage",
	}, []string{"stage"})

	// AI metrics mirror what ai_logs already records, so a dashboard
	// can alert on spend without querying Postgres.
	AIRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ai_requests_total",
		Help: "AI API requests by feature and outcome",
	}, []string{"feature", "success"})

	AIRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "ai_request_duration_seconds",
		Help:    "AI API request duration",
		Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 10, 30},
	}, []string{"feature"})

	AICostUSDTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "ai_cost_usd_total",
		Help: "AI API cost in USD",
	}, []string{"feature"})

	QueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "queue_depth",
		Help: "Depth of background job queues",
	}, []string{"queue_name", "state"})

	RateLimitedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "rate_limited_requests_total",
		Help: "Requests rejected by the rate limiter",
	}, []string{"scope"})
)

// MetricsMiddleware records duration and count for every request.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := wrapWriter(w)

		next.ServeHTTP(wrapped, r)

		// Read the route pattern only after the handler has run — chi
		// fills it in during routing, so it is empty before this point.
		path := routePattern(r)
		status := strconv.Itoa(wrapped.statusCode)

		HTTPRequestDuration.WithLabelValues(r.Method, path, status).
			Observe(time.Since(start).Seconds())
		HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
	})
}

// routePattern returns chi's route template ("/api/products/{id}"),
// which keeps one series per route instead of one per URL. Requests
// that matched no route collapse into a single "unmatched" bucket so a
// scanner probing random paths cannot inflate the series count.
func routePattern(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pattern := rctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "unmatched"
}

// MetricsHandler serves the Prometheus exposition endpoint.
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// ObserveAICall records one AI call. Counters here reset when the
// process restarts, which is expected: dashboards should use rate()
// and increase(), and ai_logs remains the durable record.
func ObserveAICall(feature string, success bool, costUSD float64, duration time.Duration) {
	AIRequestsTotal.WithLabelValues(feature, strconv.FormatBool(success)).Inc()
	AIRequestDuration.WithLabelValues(feature).Observe(duration.Seconds())
	if costUSD > 0 {
		AICostUSDTotal.WithLabelValues(feature).Add(costUSD)
	}
}
