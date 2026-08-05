# Observability

Ownstall emits three signals: structured logs, Prometheus metrics, and
OpenTelemetry traces. Each degrades to a no-op when its backend is not
configured, so the stack runs identically on a laptop with none of it.

## Why metrics are pushed, not scraped

Prometheus normally pulls. That does not work here:

- A Railway service has no stable address for a scraper to reach, and
  exposing `/metrics` publicly to let a hosted scraper in means
  publishing traffic shape and business counters.
- The Go Prometheus client cannot remote-write, so the process cannot
  push in Prometheus' own protocol either.
- The email worker serves no HTTP at all. There is nothing to scrape.

So the process pushes over OTLP instead. It exports the **same default
Prometheus registry** that `/metrics` serves, through the
OpenTelemetry Prometheus bridge — one set of instrumentation feeding
both a local scrape and a hosted backend, so the two can never
disagree.

`/metrics` still exists for local debugging and is guarded by
`METRICS_TOKEN`:

```bash
curl -H "X-Metrics-Token: $METRICS_TOKEN" http://localhost:8080/metrics
```

## Configuration

| Variable | Default | Purpose |
| --- | --- | --- |
| `OTLP_ENDPOINT` | *(unset)* | Enables traces and metric export. `host:port` or a full URL. |
| `OTLP_PROTOCOL` | `grpc` | Set to `http` for backends that document an HTTP OTLP gateway. |
| `OTLP_HEADERS` | *(unset)* | `key=value,key2=value2` — auth for hosted backends. |
| `OTLP_INSECURE` | `false` | Plaintext transport. Local collectors only. |
| `OTLP_SAMPLE_RATIO` | `0.1` | Fraction of traces kept. |
| `OTLP_METRIC_INTERVAL_SECONDS` | `60` | Metric push interval. |
| `SERVICE_NAME` | `ownstall-api` | The worker defaults to `ownstall-worker`. |
| `METRICS_TOKEN` | *(unset)* | Required for `/metrics`; unset means the endpoint always returns 403. |
| `SENTRY_DSN` | *(unset)* | Enables error reporting. |
| `SENTRY_TRACES_SAMPLE_RATE` | `0.05` | Sentry performance sampling. |
| `SENTRY_SAMPLE_RATE` | `1.0` | Sentry error sampling. Lower it if the free tier's 5k events/month runs short. |
| `RATE_LIMIT_REQUESTS` | `300` | Requests per window per client IP. |
| `RATE_LIMIT_WINDOW_SECONDS` | `60` | Rate limit window. |

### Grafana Cloud

Grafana Cloud ingests OTLP directly, so no agent or collector is
needed. From the stack's **OTLP** connection page, take the endpoint
and the instance/token pair, then set on both the API and worker
services:

```
OTLP_ENDPOINT=https://otlp-gateway-<region>.grafana.net/otlp
OTLP_PROTOCOL=http
OTLP_HEADERS=Authorization=Basic <base64 of instanceID:token>
```

Import `docs/grafana-dashboard.json` and point it at the stack's
Prometheus data source.

Counters reset when a process restarts, which on Railway means every
deploy. Every panel in the dashboard therefore uses `rate()` or
`increase()`; a raw counter would show a cliff on each release.

## Alert rules

Expressions to create as Grafana alert rules. Thresholds assume a
low-traffic service — tighten them once there is a baseline.

**API returning errors** — 5xx above 5% for 5 minutes:

```promql
sum(rate(http_requests_total{status_code=~"5.."}[5m]))
  / clamp_min(sum(rate(http_requests_total[5m])), 0.001) > 0.05
```

**Email worker stalled** — queue growing with nothing sent for 15
minutes. This is the alert that catches a dead worker, which nothing
else surfaces:

```promql
queue_depth{state="pending"} > 20
  and sum(rate(emails_sent_total{status="sent"}[15m])) == 0
```

**AI spend spike** — more than $1 in 24 hours. The in-process circuit
breaker already caps the month, so this exists to catch a runaway loop
long before the cap is reached:

```promql
sum(increase(ai_cost_usd_total[24h])) > 1
```

**AI provider degraded** — a third of calls failing over 30 minutes,
usually a rate limit or a retired model id:

```promql
sum(rate(ai_requests_total{success="false"}[30m]))
  / clamp_min(sum(rate(ai_requests_total[30m])), 0.001) > 0.3
```

**Latency regression** — p95 above one second for 10 minutes:

```promql
histogram_quantile(0.95,
  sum by (le) (rate(http_request_duration_seconds_bucket[5m]))) > 1
```

**Rate limiter engaged** — sustained rejection means either an attack
or a limit set too low for real traffic:

```promql
sum(rate(rate_limited_requests_total[10m])) > 1
```

## Label cardinality

Every distinct label combination is a separate stored series. Two rules
keep that bounded:

- HTTP paths are chi **route patterns** (`/admin/products/{id}`), never
  raw URLs, and anything that matched no route collapses into
  `unmatched` — otherwise a scanner probing random paths would mint
  unlimited series.
- No `tenant_id` label anywhere. It is unbounded by design; per-tenant
  breakdowns belong in SQL against `ai_logs` or `audit_logs`.

## Audit trail

`audit_logs` records every GraphQL **mutation** with operation name,
user, tenant, IP and outcome. Queries are deliberately not recorded:
they change nothing and would bury the writes.

```sql
SELECT action, ip_address, success, created_at
FROM audit_logs
ORDER BY created_at DESC
LIMIT 20;
```

The `tenant_id` and `user_id` columns are `ON DELETE SET NULL` rather
than `CASCADE`, so deleting an account does not erase the record of
what it did.
