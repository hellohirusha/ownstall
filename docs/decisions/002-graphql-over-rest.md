# ADR-002: GraphQL via gqlgen as the primary API

**Status**: Accepted
**Date**: 2026-07-29

## Context

Three clients read the same data: the admin dashboard, the public
storefront, and the Expo mobile app. They need overlapping but different
slices of it. The product detail page alone pulls the product, its variants,
its images, and its recommendations; the mobile product screen wants the
first image and the price and nothing else.

## Decision

A single GraphQL endpoint at `POST /query`, served by
[gqlgen](https://gqlgen.com) with a schema-first workflow. REST is kept for
the handful of operations that are not data graphs: auth, Stripe checkout
session creation, image upload, and inbound webhooks.

## Rationale

The split is the point. GraphQL earns its place on the read paths, where
three clients want three shapes of the same entity graph and the
alternative is either over-fetching or a proliferation of
`/products?include=variants,images` query parameters that slowly reinvent a
query language.

It earns nothing on `POST /webhooks/stripe`, which receives a fixed payload
from a third party that has never heard of our schema, or on file upload,
where multipart bodies fit REST and fit GraphQL badly.

gqlgen specifically, over a runtime-reflection library: the schema generates
Go types, so a resolver that fails to return a required field does not
compile. The generated code is checked in and regenerated with
`go tool gqlgen generate`, which keeps the schema and the resolvers from
drifting.

Resolvers stay thin — they set the tenant context, call a service, and map
the result. Business logic lives in `internal/services` so it is reachable
from the worker and the evaluation scripts, neither of which speaks GraphQL.

## Consequences

N+1 queries are the standing risk. A `products` query that resolves images
per product issues one query per product unless the resolver batches. This
is not yet solved with DataLoader; the current catalogue sizes do not
demand it, and the fix is well understood when they do. It is a known debt,
not an oversight.

Query cost is unbounded by default. A deeply nested query is more expensive
than any REST endpoint could be, and the current defence is the per-IP and
per-tenant rate limiter rather than query-depth analysis.

Audit logging had to be taught to read GraphQL: `POST /query` is one HTTP
route, so the middleware parses the document to distinguish mutations from
queries rather than keying on the method and path. That parsing is in
`internal/middleware/audit.go` and is unit-tested, because an operation it
fails to classify as a mutation is a state change that goes unrecorded.

## Revisiting

The schema is the contract; the transport underneath it is not load-bearing.
Adding a REST facade for a specific integration partner would not disturb
the service layer, since resolvers already delegate to it.
