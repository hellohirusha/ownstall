# ADR-001: Row-level security for multi-tenant isolation

**Status**: Accepted
**Date**: 2026-07-29

## Context

Every merchant on Ownstall gets a storefront, an order book, a support inbox
and a creator profile. All of it shares one Postgres instance. A bug that
leaks one tenant's orders into another tenant's admin page is the kind of
failure that ends the product, so isolation had to be enforced somewhere
that a forgotten `WHERE` clause could not defeat.

Three options were on the table: a database per tenant, a schema per tenant,
or row-level security within a single schema.

## Decision

Row-level security, with the tenant supplied per-transaction through a
session variable.

Every tenant-scoped table enables RLS and carries a policy:

```sql
ALTER TABLE products ENABLE ROW LEVEL SECURITY;

CREATE POLICY product_tenant_isolation ON products
    USING (tenant_id = current_setting('app.current_tenant_id', true)::uuid);
```

Child tables that have no `tenant_id` of their own inherit through their
parent rather than denormalising the column:

```sql
CREATE POLICY product_image_isolation ON product_images
    USING (product_id IN (
        SELECT id FROM products
        WHERE tenant_id = current_setting('app.current_tenant_id', true)::uuid
    ));
```

Request handlers set the variable before touching tenant data:

```go
_, err := db.Exec(ctx,
    "SELECT set_config('app.current_tenant_id', $1, true)", tenantID)
```

The `true` third argument makes it transaction-local, so the setting cannot
leak to the next request that borrows the same pooled connection.

## Rationale

**A database per tenant** was ruled out on cost before anything else.
Managed Postgres starts around $25/month per instance, which makes a
thousand tenants a five-figure monthly bill before a single order is placed.
It also multiplies connection pools, backups and migration runs by the
tenant count.

**A schema per tenant** has a genuinely attractive property: the blast
radius of a bad migration is one tenant. The cost is that every migration
becomes a loop over N schemas, with partial-failure states where some
tenants are on the new schema and some are not. That demands a migration
runner considerably more sophisticated than the one this project has, which
walks a directory of `.sql` files on boot.

**Row-level security** puts the check in the one place that sees every
query. A resolver that forgets to filter by tenant returns zero rows rather
than someone else's data — the failure mode is a visible bug rather than a
silent leak. Migrations run once.

## Consequences

Policy evaluation adds roughly 2–3ms per query. At the scale this system is
built for that is comfortably acceptable, and it buys an isolation guarantee
that does not depend on every future query being written correctly.

The session variable is now load-bearing. Any code path that queries tenant
data without setting it gets an empty result set, which is confusing the
first time it happens and is the single most likely source of "why is this
list empty" bugs. That is the price of the guarantee, and it is paid in
development rather than in production.

Connection pooling and RLS interact: a transaction-local setting is correct,
a session-local one would not be. This is not obvious and is worth
preserving in any refactor of the database layer.

## Revisiting

Moving to schema-per-tenant later is possible but not cheap — it means a
migration runner that iterates schemas and a data migration per tenant. The
more likely evolution is keeping RLS and sharding by tenant across several
Postgres instances, which the current design does not obstruct: the tenant
id is already the partition key in everything but name.
