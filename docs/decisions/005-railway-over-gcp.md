# ADR-005: Railway for hosting

**Status**: Accepted
**Date**: 2026-07-28

## Context

The system needs Postgres, Redis, a long-running Go API, and a worker
process, all reachable from a public URL so the demo is a link rather than a
set of instructions. It also needs to cost nothing, because it is a
portfolio project with no revenue.

## Decision

Railway for the backend, Postgres and Redis. Vercel for the React frontend.

Deployment is through GitHub Actions: pushes to `main` that touch
`backend/**` run the test suite, then `railway up`. Migrations run on boot
from `pkg/database/migrations`, so a deploy and a schema change are the same
event.

## Rationale

GCP Cloud Run plus Cloud SQL is the shape this would take in production, and
Cloud SQL has no free tier — roughly $25/month before any traffic. Railway
provisions Postgres, Redis and a web service on its free allowance, which
covers a demo comfortably.

Vercel for the frontend because its free tier has no bandwidth cap worth
worrying about, and a Create React App build needs a static host and a SPA
rewrite, nothing more.

The important property is that neither choice reaches into the application.
The backend is a container that reads `DATABASE_URL`, `REDIS_URL` and a
port from the environment. There is no Railway SDK in `go.mod`.

## Consequences

The free tier is the performance ceiling, and it is lower than the
application's. Any load test that saturates it is measuring Railway's
allowance rather than the code, which has to be stated whenever throughput
numbers are quoted.

Railway restarts the service on every deploy. This is why the AI cost
breaker reseeds its month-to-date total from `ai_logs` rather than trusting
an in-memory counter — an in-process accumulator would reset the budget on
every deploy.

TLS terminates at Railway's edge, so the origin sees plain HTTP. HSTS is
therefore emitted on `X-Forwarded-Proto: https` as well as on a direct TLS
connection, and the client IP used for rate limiting comes from
`X-Forwarded-For` rather than the socket address.

Postgres is single-region. A user far from that region pays the round trip,
and no amount of query tuning changes it.

## Revisiting

The migration path to GCP is documented rather than automated:

1. `pg_dump` from Railway, restore into Cloud SQL.
2. Build the existing `backend/Dockerfile`, push to Artifact Registry.
3. Deploy to Cloud Run with the same environment variables.
4. Repoint DNS.

No application code changes. The connection strings are environment
variables and the container is the same one Railway runs.
