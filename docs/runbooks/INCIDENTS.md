# Incident Response

One playbook per alert rule defined in [observability.md](../observability.md).
If an alert fires and there is no playbook here, that is a gap worth closing
while the incident is still fresh.

## Before you need this

Have these to hand. Finding out you lack access during an incident is the
most common reason a five-minute fix takes an hour.

- Railway dashboard access, and the CLI logged in (`railway login`)
- `METRICS_TOKEN` — without it `/metrics` returns 403 and you are blind
- Grafana Cloud login
- Stripe dashboard access
- Resend dashboard access

## Two things that will surprise you

**`/health` does not check the database.** It is a static 200:

```go
r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    _, _ = w.Write([]byte(`{"status":"ok","version":"0.1.0"}`))
})
```

A green health check means the process is running and routing. It says
nothing about Postgres, Redis, or any provider. Never close an incident on
`/health` alone.

**There is no `psql` on the operator's machine.** To run SQL against
production, either use the Railway dashboard's Postgres **Data** tab, or
write a one-off Go script under `backend/scripts/` with a `//go:build ignore`
tag and run it with `go run` — the pattern
[`cleanup_loadtest.go`](../../backend/scripts/cleanup_loadtest.go) follows.
Queries in this document assume one of those two paths.

## Reading metrics

```bash
curl -s -H "X-Metrics-Token: $METRICS_TOKEN" \
  https://ownstall.up.railway.app/metrics | grep -v '^#'
```

Counters reset when the process restarts, which Railway does on every
deploy. Use `rate()` and `increase()` in Grafana rather than raw counter
values, and treat a counter that dropped to zero as a restart until proven
otherwise.

---

## API returning errors

**Alert**: 5xx above 5% for 5 minutes.

1. Confirm it is real and not a single scanner hitting one broken route:

   ```promql
   sum by (path, status_code) (rate(http_requests_total{status_code=~"5.."}[5m]))
   ```

   `path` is chi's route pattern; anything that matched no route collapses
   into `unmatched`.

2. Read the logs — they are structured JSON carrying status, duration,
   client IP and trace id:

   ```bash
   railway logs --service backend
   ```

3. Check Sentry for the stack trace. Sentry has the exception; Prometheus
   only has the count.

4. If it started at a deploy, roll back (see [Deploying a fix](#deploying-a-fix)).

5. If every route is failing, suspect Postgres rather than the application —
   go to [Database unreachable](#database-unreachable).

---

## Latency regression

**Alert**: p95 above one second for 10 minutes.

1. Find which route:

   ```promql
   histogram_quantile(0.95,
     sum by (le, path) (rate(http_request_duration_seconds_bucket[5m])))
   ```

2. If it is `/query`, it is almost certainly a GraphQL resolver issuing a
   query per row. The resolvers do not use DataLoader — this is a known and
   documented gap ([ADR-002](../decisions/002-graphql-over-rest.md)), and a
   catalogue that grew is the usual trigger.

3. If it is `/api/checkout/session`, check [status.stripe.com](https://status.stripe.com)
   before touching anything. That handler calls Stripe synchronously, so
   Stripe's latency is our latency on that route.

4. If every route degraded together, it is the database or the Railway
   instance, not any one query. Railway's free tier is shared CPU; a noisy
   neighbour looks exactly like a code regression.

**Do not** change `MaxConns` reflexively. The pool is 25
([database.go](../../backend/pkg/database/database.go)); raising it under
contention usually moves the queue from the application into Postgres.
Confirm connection acquisition is actually the bottleneck first.

---

## Email worker stalled

**Alert**: `queue_depth{state="pending"} > 20` with no sends for 15 minutes.

This is the alert that catches a dead worker — nothing else surfaces it,
because the API keeps accepting work and returning 200 while the queue
grows behind it.

1. Is the worker running at all?

   ```bash
   railway logs --service worker
   ```

   On boot it prints `Worker listening on queue: queue:email`.

2. Check queue depth by name. The queues are `queue:email`,
   `queue:email:campaign`, `queue:notify` and `queue:ai`:

   ```promql
   queue_depth{state="pending"}
   ```

3. If the worker is alive but not draining, the likely cause is a job that
   fails and retries forever. Jobs carry an attempt count and a ceiling;
   look for the same job id repeating in the logs.

4. If Redis is the problem, note what degrades: the API logs
   `Redis unavailable — order emails and rate limiting disabled` at boot and
   keeps serving. Orders are still taken and still paid. Confirmation emails
   are skipped, not queued — they are lost, not delayed.

5. After recovery, check Resend for anything that went out twice.

---

## AI spend spike

**Alert**: more than $1 of AI cost in 24 hours.

The in-process breaker already caps the month at
`AI_MONTHLY_COST_LIMIT_USD` (default $50). This alert exists to catch a
runaway loop long before that cap.

1. Which feature:

   ```promql
   sum by (feature) (increase(ai_cost_usd_total[24h]))
   ```

2. `ai_logs` is the durable record — the Prometheus counters reset on
   deploy, the table does not:

   ```sql
   SELECT feature, COUNT(*), SUM(cost_usd), AVG(latency_ms)
   FROM ai_logs
   WHERE created_at > NOW() - INTERVAL '24 hours'
   GROUP BY feature
   ORDER BY SUM(cost_usd) DESC;
   ```

3. Repeated near-identical calls for the same `reference_id` mean a retry
   loop, not real demand.

4. To stop it immediately, unset `GROQ_API_KEY` and `OPENAI_API_KEY` on the
   service. AI features return `ErrDisabled` and become no-ops; products
   keep their existing descriptions and every non-AI feature is unaffected.
   This is a designed degradation path, not a blunt instrument.

5. If the spend is legitimate, raise `AI_MONTHLY_COST_LIMIT_USD` rather than
   removing the breaker.

---

## AI provider degraded

**Alert**: a third of AI calls failing over 30 minutes.

Two causes account for nearly all of these.

**A retired model id.** Providers decommission models with little notice —
the `llama-3.1-70b` generation named in the original build plan was retired
mid-build. The error text carries the provider's message:

```sql
SELECT model, error_message, COUNT(*)
FROM ai_logs
WHERE success = false AND created_at > NOW() - INTERVAL '1 hour'
GROUP BY model, error_message
ORDER BY COUNT(*) DESC;
```

Fix by setting `GROQ_MODEL` to a current model. No code change.

**Rate limiting.** Groq's free tier is roughly 30 requests/minute and has a
per-minute token ceiling. The client already retries with backoff and
honours `Retry-After`, so sustained failure means the limit is being
exceeded faster than backoff can absorb. Reduce concurrency or move to
OpenAI by setting `OPENAI_API_KEY` — the client prefers Groq when both are
present, so also unset `GROQ_API_KEY` to force the switch.

Nothing here is customer-facing on its own: failed AI calls degrade to
no-ops. Copy generation returns fewer variants; unscored variants keep a
neutral 0.5 and are never auto-published.

---

## Rate limiter engaged

**Alert**: sustained rejections.

```promql
sum by (scope) (rate(rate_limited_requests_total[10m]))
```

`scope` is `ip` or `tenant`.

- **`ip`** — one source. Either an attack, or a legitimate integration
  hitting the default 300 requests/minute.
- **`tenant`** — one merchant's script. The tenant limit exists so one
  tenant cannot consume the capacity of every other tenant on the
  deployment; this alert firing means it did its job.

Raise `RATE_LIMIT_REQUESTS` if the traffic is legitimate. Remember to put it
back — a raised limit left in place is the protection silently switched off.

Webhook paths (`/webhooks/`) and `/health` are exempt by design: Stripe and
Resend retry on a 429, and dropping a payment webhook is worse than serving
a burst. They are authenticated by signature instead.

---

## Order stuck in `pending`

No alert covers this; it arrives as a customer email. The customer paid and
Stripe shows the charge, but `/admin/orders` says pending.

The order row is written before the Stripe session is created, and only the
webhook moves it to `paid`. A pending order with a real charge means the
webhook did not arrive or was rejected.

1. Stripe Dashboard → Developers → Webhooks → the endpoint. Non-200
   responses are listed with the failure reason.
2. A 400 with `invalid signature` means `STRIPE_WEBHOOK_SECRET` does not
   match the endpoint's secret. The Stripe **CLI** secret only works
   locally — a common mix-up after a redeploy.
3. Stripe retries failed webhooks for up to three days. Fix the secret and
   the backlog resolves itself; you usually do not need to replay by hand.
4. To confirm the fix, replay one event from the Stripe dashboard and watch
   for the order to flip.

Forged webhooks are rejected before any database work, and that path is
unit-tested — a signature failure is a configuration problem, never a
tampering problem you need to investigate.

---

## Database unreachable

No alert covers this directly; it shows up as every route returning 5xx
while `/health` stays green.

1. Railway dashboard → Postgres service. Check it is running and within its
   free-tier limits.
2. Free-tier Postgres has a connection ceiling. The API pool is 25 and the
   worker opens its own — two API instances plus a worker can exhaust it.
   Symptom is connection timeouts rather than query errors.
3. Confirm `DATABASE_URL` on the backend service still references the
   Postgres service. A recreated database issues a new URL.
4. Restart the API after the database comes back so the pool reconnects
   cleanly.

---

## Deploying a fix

```bash
railway logs --service backend        # confirm the current state first
railway redeploy --service backend    # or roll back from the dashboard
curl https://ownstall.up.railway.app/health
```

**The free plan restricts deploys during peak US hours.** Assume you may be
unable to ship a fix for several hours. This changes triage: prefer
mitigations that need no deploy.

Anything reachable through environment variables can be changed without a
code deploy, though setting one still restarts the service:

| Lever                       | Effect                                          |
| --------------------------- | ----------------------------------------------- |
| `RATE_LIMIT_REQUESTS`       | Loosen or tighten abuse protection              |
| `GROQ_MODEL`                | Move off a retired model                        |
| `GROQ_API_KEY` unset        | Disable AI features; everything else keeps working |
| `AI_MONTHLY_COST_LIMIT_USD` | Raise or lower the spend cap                    |
| `STRIPE_WEBHOOK_SECRET`     | Fix webhook signature rejection                 |

Migrations run on boot from `pkg/database/migrations`, so a deploy and a
schema change are the same event. A migration that fails takes the boot with
it — check the logs immediately after any deploy that adds one.

---

## After the incident

Write down what actually happened, not what should have. If an alert did not
fire when it should have, add the rule to
[observability.md](../observability.md). If a playbook here sent you down
the wrong path, fix it — a runbook that has never been corrected has never
been used.
