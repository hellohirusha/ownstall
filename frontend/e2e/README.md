# End-to-end tests

Playwright, chromium only. The specs drive the real storefront against a
real API and a real database.

```bash
cd frontend
npm run test:e2e            # run
npm run test:e2e:ui         # watch mode
npm run test:e2e:report     # open the last HTML report
```

## What has to be running

Playwright starts the React dev server itself and reuses one already on
`:3000`. It does **not** start the Go API — `global-setup.ts` checks
`/health` first and fails with a readable message rather than letting
every spec time out in a browser step.

```bash
cd backend && go run cmd/api/main.go
```

Point the specs elsewhere with `E2E_API_URL` and `E2E_FRONTEND_URL`.

## Unlocking the shopper specs

A stall opens in `pending`. Until a platform operator approves it, its
products are not publicly listed and it cannot create a checkout
session — so almost everything a shopper does needs an approved stall.

Seven of the nine specs therefore need operator credentials and **skip**
without them. To run the full suite, set the bootstrap operator on the
API:

```
# backend/.env — the operator is created on boot when the table is empty
PLATFORM_ADMIN_EMAIL=admin@ownstall.test
PLATFORM_ADMIN_PASSWORD=<at least 12 characters>
PLATFORM_ADMIN_NAME=E2E Operator
```

Restart the API, then mirror the same values into the test environment:

```
# frontend/.env.local
E2E_ADMIN_EMAIL=admin@ownstall.test
E2E_ADMIN_PASSWORD=<the same password>
```

The specs log in at `/api/admin/login` and call `approveStore` before
each shopper scenario.

## What is deliberately not covered

**Completing a Stripe payment.** The specs stop at the redirect to
`checkout.stripe.com`. Reaching it proves the API priced the cart, wrote
a pending order and got a session URL back. Going further means driving
Stripe's own DOM, which this project does not control, and the order
only becomes `paid` when Stripe's webhook arrives — which needs
`stripe listen` running alongside. Both are worth doing by hand before a
release; neither belongs in a suite that has to pass unattended.

## Test data

Every spec creates its own tenant, so specs run in parallel without
contending. Tenants are named `loadteste2e<random>`, inside the
`loadtest` namespace that the cleanup script recognises:

```bash
cd backend
go run scripts/cleanup_loadtest.go --all              # dry run
go run scripts/cleanup_loadtest.go --all --confirm    # delete
```

Run that periodically — a full suite leaves nine tenants behind.

## One sharp edge

Product names in these specs **lead with ten random characters**, and
that is load-bearing rather than cosmetic.

`CreateProduct` derives the default variant's SKU from the first eight
characters of the product slug, and `product_variants.sku` carries a
global `UNIQUE` constraint. Two products whose names share a leading
eight characters cannot both exist — in the same tenant or in different
ones. A fixed name like `E2E Test Sticker` makes the second spec fail
with a duplicate key error.

A timestamp does not solve it either: the first eight digits of a
millisecond timestamp only change every ~100 seconds, so a whole run
lands on one SKU.
