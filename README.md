# Ownstall

> A marketplace of independent stalls — every seller gets their own storefront,
> and shoppers get one place to find all of them.
> Built with Go, GraphQL, React, TypeScript, and Postgres.

## How it works

Ownstall has three kinds of user, and they are deliberately kept apart —
separate credentials, separate token scopes, separate surfaces:

| Role                | Signs in at       | Can do                                                                 |
| ------------------- | ----------------- | ---------------------------------------------------------------------- |
| **Seller** (tenant) | `/login`          | Open a stall, add products, manage orders, support, bookings            |
| **Buyer**           | `/account/login`  | Browse every approved stall, buy as guest or signed in, see order history |
| **Platform admin**  | `/platform/login` | Approve/reject stalls, suspend, restrict, view platform-wide stats      |

The lifecycle of a stall:

```
seller signs up ──► pending ──► admin approves ──► listed in /stores, can take orders
                       │                                    │
                       └──► rejected (resubmit)             ├──► restricted (listed, capability removed)
                                                            └──► suspended (delisted, no orders)
```

A stall is invisible to shoppers — not in search, no storefront, no checkout —
until it is approved. Buyers never need an account: guest checkout is a
first-class path, and a guest's past orders are claimed automatically if they
later sign up with the same email.

## Status

| Goal                                                  | Status |
| ----------------------------------------------------- | ------ |
| Auth (JWT) + GraphQL foundation                       | ✅     |
| Product catalog + image uploads                       | ✅     |
| Cart, checkout, orders, Stripe webhooks               | ✅     |
| Notify — templates, campaigns, open/bounce tracking   | ✅     |
| Reply — support inbox, SLA metrics, canned responses  | ✅     |
| Hire Me — creator profiles, bookings, Stripe Connect  | ✅     |
| Manufacturing — production queue + status simulator   | ✅     |
| AI — copy generation, recommendations, auto-reply     | ✅     |
| Observability — logs, metrics, tracing, Sentry        | ✅     |
| Security — rate limiting, security headers, audit log | ✅     |
| Mobile (Expo) — 6 screens, push, offline cache        | ✅     |
| Buyer accounts + guest checkout                       | ✅     |
| Stall directory with search + filters                 | ✅     |
| Platform admin — approval queue, suspend, restrict    | ✅     |
| Terms acceptance + privacy policy                     | ✅     |
| Load tests, E2E tests, store builds                   | ⏳     |

**Live demo**

- API: `https://ownstall.up.railway.app/health`
- Web: `https://ownstall.vercel.app`

## Architecture

```
React + TS (Vercel) ──► Go / Chi + gqlgen (Railway) ──► Postgres 15 (Railway)
        │                        │
        │                        ├─► Stripe (checkout + webhooks)
        └─► Apollo Client        └─► Cloudinary (product images)
```

Multi-tenancy is enforced at the database layer with Postgres **row-level security** —
every tenant's rows are isolated by a session-scoped `app.current_tenant_id` setting.

## Stack

| Layer    | Technology             | Notes                                 |
| -------- | ---------------------- | ------------------------------------- |
| Backend  | Go 1.25 + Chi + gqlgen | GraphQL-first, typed schema           |
| Database | Postgres 15            | RLS for multi-tenant isolation        |
| Queue    | Redis 7                | Job queue for email, AI and production |
| Frontend | React 19 + TypeScript  | CRA, Tailwind CSS, Apollo Client v4   |
| State    | Zustand                | Persistent cart store                 |
| Auth     | JWT (HS256)            | Access + refresh tokens               |
| Payments | Stripe                 | Hosted Checkout, webhooks, Connect    |
| Images   | Cloudinary             | Upload endpoint + CDN delivery        |
| Email    | Resend                 | Templates, campaigns, inbound + tracking |
| AI       | Groq (Llama 3.3 70B)   | Metered client, monthly cost breaker  |
| Mobile   | Expo (React Native)    | Apollo, push notifications, offline cache |
| Telemetry| OTel + Prometheus + Sentry | Structured logs, metrics, traces  |

## Project structure

```
ownstall/
├── backend/            Go API (Chi router, gqlgen GraphQL, handlers, services)
│   ├── cmd/api/        Entrypoint
│   ├── graph/          GraphQL schema + resolvers
│   ├── internal/       auth, handlers, middleware, models, services
│   └── pkg/database/   Connection + SQL migrations (run on boot)
├── frontend/           React + TypeScript app (CRA)
│   └── src/pages/      Landing, company (about/contact), legal (terms/privacy),
│                       auth (seller), account (buyer), platform (operator),
│                       admin (seller dashboard), store (finder + storefront)
├── mobile/             Expo app (screens, Apollo, push, offline cache)
├── scripts/            deploy / seed / test helpers
├── docs/               Architecture decisions, runbooks, observability
└── docker-compose.yml  Local Postgres + Redis (+ Redis GUI on :8081)
```

## Prerequisites

- Go 1.25+
- Node.js 20+
- Docker Desktop (for local Postgres/Redis)
- A Stripe account (test mode) and the [Stripe CLI](https://github.com/stripe/stripe-cli/releases) for webhook testing
- A Cloudinary account (image uploads)

## Quick start (Windows / PowerShell)

```powershell
# Clone
git clone https://github.com/hellohirusha/ownstall.git
cd ownstall

# Start local Postgres + Redis
docker compose up -d

# Backend — terminal 1
cd backend
Copy-Item .env.example .env    # then fill in your keys (see table below)
go run cmd/api/main.go         # migrations run automatically on boot
# → http://localhost:8080/health          {"status":"ok","version":"0.1.0"}
# → http://localhost:8080/playground      GraphQL playground (non-production)

# Frontend — terminal 2
cd frontend
Copy-Item .env.example .env
npm install
npm start
# → http://localhost:3000

# Stripe webhooks — terminal 3 (keep running while testing checkout)
stripe listen --forward-to localhost:8080/webhooks/stripe
# Copy the printed whsec_... into backend/.env as STRIPE_WEBHOOK_SECRET,
# then restart the backend (terminal 1).
```

macOS/Linux users: the same commands work in any shell; replace `Copy-Item` with `cp`.

### Walk the full marketplace flow

Set `PLATFORM_ADMIN_EMAIL` and `PLATFORM_ADMIN_PASSWORD` in `backend/.env`
before the first boot — the operator account is created only when the
`platform_admins` table is empty.

**As a seller**

1. `http://localhost:3000/signup` → open a stall (the terms checkbox is required)
2. `/admin/products/new` → add a product
3. The dashboard shows an amber banner: the stall is **pending** and invisible

**As the platform admin**

4. `/platform/login` → sign in with the bootstrap credentials
5. The **Pending review** tab lists the new stall → **Approve**

**As a buyer**

6. `/stores` → search for the stall; it appears now that it is approved
7. Open it, add a product to the cart, go to `/cart`
8. Check out as a **guest**, or sign in at `/account/login` first
9. Pay with Stripe's test card `4242 4242 4242 4242` (any future expiry, any CVC)
10. You land on `/order/success`; the webhook marks the order **paid**
11. Signed-in buyers see it under `/account`; the seller sees it at `/admin/orders`

**Back as the admin** — suspend the stall and confirm it vanishes from `/stores`
and can no longer take orders.

## Environment variables

### Backend (`backend/.env`)

| Variable                                                               | Required now | Purpose                                                                      |
| ---------------------------------------------------------------------- | ------------ | ---------------------------------------------------------------------------- |
| `DATABASE_URL`                                                         | ✅           | Postgres connection string                                                   |
| `PORT`                                                                 | ✅           | API port (default 8080)                                                      |
| `ENVIRONMENT`                                                          | ✅           | `development` / `production`                                                 |
| `FRONTEND_URL`                                                         | ✅           | Used for Stripe success/cancel redirects + CORS                              |
| `JWT_SECRET`, `JWT_ACCESS_EXPIRY_MINUTES`, `JWT_REFRESH_EXPIRY_DAYS`   | ✅           | Token signing + lifetimes                                                    |
| `STRIPE_SECRET_KEY`, `STRIPE_PUBLISHABLE_KEY`                          | ✅           | Stripe API keys (test mode: `sk_test_…`)                                     |
| `STRIPE_WEBHOOK_SECRET`                                                | ✅           | Local: from `stripe listen`. Production: from the Dashboard webhook endpoint |
| `CLOUDINARY_CLOUD_NAME`, `CLOUDINARY_API_KEY`, `CLOUDINARY_API_SECRET` | ✅           | Product image uploads                                                        |
| `REDIS_URL`                                                            | ✅           | Job queue (email, AI, production). Absent → rate limiting is a pass-through  |
| `RESEND_API_KEY`, `EMAIL_FROM`, `EMAIL_FROM_NAME`                      | ✅           | Transactional email, campaigns, inbound parsing                              |
| `GROQ_API_KEY`, `GROQ_MODEL`, `AI_MONTHLY_COST_LIMIT_USD`              | —            | AI features. Absent → they degrade to no-ops rather than failing the boot    |
| `RATE_LIMIT_REQUESTS`, `RATE_LIMIT_WINDOW_SECONDS`                     | —            | Per-IP and per-tenant caps (default 300 per minute)                          |
| `METRICS_TOKEN`                                                        | —            | Required to read `/metrics`; unset leaves the endpoint closed                |
| `SENTRY_DSN`                                                           | —            | Error tracking (optional)                                                    |
| `PLATFORM_ADMIN_EMAIL`, `PLATFORM_ADMIN_PASSWORD`, `PLATFORM_ADMIN_NAME` | —          | Creates the first operator account, and only while `platform_admins` is empty. Password must be 12+ chars. Never resets an existing operator |

### Frontend (`frontend/.env`)

| Variable                                                      | Purpose                                                  |
| ------------------------------------------------------------- | -------------------------------------------------------- |
| `REACT_APP_API_URL`                                           | REST base URL (`http://localhost:8080` locally)          |
| `REACT_APP_GRAPHQL_URL`                                       | GraphQL endpoint (`http://localhost:8080/query` locally) |
| `REACT_APP_STRIPE_PUBLISHABLE_KEY`                            | Stripe publishable key (`pk_test_…`)                     |
| `REACT_APP_SENTRY_DSN`, `REACT_APP_NAME`, `REACT_APP_VERSION` | Optional metadata                                        |

> CRA bakes `REACT_APP_*` values in at **build time** — change them, then rebuild/redeploy.

## API overview

| Endpoint                                            | Auth             | Purpose                                                      |
| --------------------------------------------------- | ---------------- | ------------------------------------------------------------ |
| `GET /health`                                       | —                | Liveness check                                               |
| `GET /metrics`                                      | `X-Metrics-Token`| Prometheus exposition; closed unless `METRICS_TOKEN` is set  |
| `POST /api/signup` / `login` / `refresh` / `logout` | —                | Seller JWT auth                                              |
| `POST /api/buyer/signup` / `login` / `refresh` / `logout` | —          | Buyer JWT auth (`buyer` scope)                               |
| `GET /api/buyer/me`                                 | Buyer            | Signed-in shopper's profile                                  |
| `POST /api/admin/login` / `refresh` / `logout`      | —                | Operator JWT auth (`admin` scope). No signup — accounts are provisioned |
| `POST /query`                                       | Optional         | GraphQL (admin queries need `Authorization: Bearer <token>`) |
| `GET /playground`                                   | —                | GraphQL playground (non-production only)                     |
| `POST /api/checkout/session`                        | Optional         | Create Stripe Checkout session + pending order. Public so guests can buy; the stall is derived from the cart's variants, never from the caller's token |
| `POST /api/upload/product-image`                    | ✅               | Cloudinary product image upload                              |
| `POST /webhooks/stripe`                             | Stripe signature | Marks orders paid, handles expiry/failure                    |
| `POST /webhooks/resend`                             | Resend signature | Delivery, bounce and complaint events                        |
| `GET /webhooks/email/open`                          | —                | Tracking pixel                                               |
| `POST /webhooks/email/inbound`                      | Resend signature | Inbound email → support ticket                               |

Webhook paths are exempt from rate limiting — providers retry on a 429, and
dropping a payment webhook is worse than serving a burst. They are
authenticated by signature instead.

### GraphQL surface

| Area          | Queries                                                          | Mutations                                                                          |
| ------------- | ---------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| Auth          | `me`                                                             | `login`, `signup`, `updateProfile`                                                 |
| Stores        | `tenant`, `products`, `product`, `productBySlug`, `orders`, `order` | `createProduct`, `updateProduct`, `deleteProduct`, `publishProduct`, `addProductImage` |
| Notify        | `emailTemplates`, `emailCampaigns`                               | `createCampaign`, `scheduleCampaign`                                               |
| Reply         | `tickets`, `ticket`, `cannedResponses`, `supportMetrics`         | `createTicket`, `replyToTicket`, `updateTicketStatus`                              |
| Manufacturing | `productionQueue`, `productionStats`                             | `advanceProductionStatus`, `simulateProduction`                                    |
| Hire Me       | `myCreatorProfile`, `creatorProfile`, `booking`                  | `updateCreatorProfile`, `createCreatorService`, `createBooking`, `acceptBooking`, `declineBooking`, `deliverBooking`, `completeBooking`, `sendBookingMessage`, `generateStripeOnboardingLink` |
| AI            | `aiStats`, `productsMissingCopy`, `recommendedProducts`          | `generateProductCopy`, `indexAllProducts`, `processNewTickets`                     |
| Mobile        | —                                                                | `registerDeviceToken`                                                              |
| Marketplace   | `stores`, `storeCategories`, `store`, `myStore`, `myOrders`      | `updateStoreProfile`, `submitStoreForReview`, `acceptTerms`                        |
| Platform admin| `platformStats`, `adminStores`, `storeModerationHistory`         | `approveStore`, `rejectStore`, `suspendStore`, `restoreStore`, `setStoreRestrictions` |

Schema: [`backend/graph/schema.graphqls`](backend/graph/schema.graphqls) and
[`backend/graph/marketplace.graphqls`](backend/graph/marketplace.graphqls)

**Token scopes.** Access tokens carry a `scope` claim (`tenant`, `buyer` or
`admin`) and the middleware refuses a token minted for a different audience —
a buyer token on a seller endpoint is a `403`, not a `401`. Tokens issued
before scopes existed decode as `tenant`, which is what they were.

## Documentation

- [Architecture Decision Records](docs/decisions/) — why the system is shaped
  this way: [RLS multi-tenancy](docs/decisions/001-multi-tenant-isolation.md),
  [GraphQL](docs/decisions/002-graphql-over-rest.md),
  [the job queue](docs/decisions/003-redis-lists-over-pubsub.md),
  [the AI provider](docs/decisions/004-groq-as-ai-provider.md),
  [hosting](docs/decisions/005-railway-over-gcp.md)
- [Observability](docs/observability.md) — logs, metrics, traces, error reporting
- [Incident runbook](docs/runbooks/INCIDENTS.md) — one playbook per alert rule
- [Grafana dashboard](docs/grafana-dashboard.json) — importable panel definitions

## Testing

```bash
cd backend && go test ./...
```

CI additionally runs with `-race`, which needs cgo — on Windows without a C
toolchain that flag fails with `-race requires cgo`, so run it plain locally
and let CI cover the race detector.

The suite is deliberately concentrated on the logic where a silent regression
would be expensive, and it runs without a database or network:

| Package              | Coverage | What it pins down                                                      |
| -------------------- | -------- | ---------------------------------------------------------------------- |
| `internal/auth`      | 96%      | Token round-trip, lifetimes, expiry, wrong secret, `alg=none` downgrade |
| `pkg/ai`             | 62%      | JSON extraction from fenced/prose replies, reasoning-trace stripping, cost pricing, retry policy, cost breaker |
| `internal/middleware`| 53%      | Refresh-token-as-access rejection, security headers, HSTS only over TLS, NUL-byte rejection, mutation auditing |
| `internal/handlers`  | 5%       | Stripe webhook signature: forged, tampered and replayed events are rejected |
| `pkg/telemetry`      | 8%       | Client IP resolution behind a proxy (the rate-limit key)                |

The two low percentages are honest: those packages are mostly database and
Stripe I/O, and only their security boundary is unit-tested. The handler
tests deliberately run with a nil database — anything that reaches Postgres
panics rather than passing quietly.

Not yet covered by unit tests: resolver-level integration against Postgres.

### End-to-end

```bash
cd frontend && npm run test:e2e
```

Playwright drives the real storefront against a running API. It covers the
approval gate (an unapproved stall is neither listed nor able to take money),
the browse → cart → checkout path, and cross-tenant order isolation.

The specs stop at the redirect to Stripe's hosted page — going further means
driving a third-party DOM, and the order only becomes `paid` once Stripe's
webhook arrives. Seven of the nine specs need platform operator credentials
and skip without them. Setup, and the reason product names in the specs lead
with random characters, are in [frontend/e2e/README.md](frontend/e2e/README.md).

## Deployment

### Backend → Railway (via GitHub Actions)

CI ([`.github/workflows/backend.yml`](.github/workflows/backend.yml)) tests every push
touching `backend/**` on `develop`/`main`, and **deploys to Railway only from `main`**.

1. **Provision** — Railway project with a Postgres service and the backend service
   (builds from `backend/Dockerfile`). Add `RAILWAY_TOKEN` to GitHub repo secrets.
2. **Configure** — Railway → backend service → _Variables_: set every "Required now"
   backend variable above. `DATABASE_URL` references the Railway Postgres.
   `ENVIRONMENT=production`. `FRONTEND_URL=<your Vercel URL>`.
3. **Production Stripe webhook** — Stripe Dashboard → _Developers → Webhooks → Add endpoint_:
   - URL: `https://ownstall.up.railway.app/webhooks/stripe`
   - Events: `checkout.session.completed`, `checkout.session.expired`, `payment_intent.payment_failed`
   - Copy the endpoint's `whsec_…` → Railway variable `STRIPE_WEBHOOK_SECRET`.
     (The Stripe **CLI** secret only works locally.)
4. **Deploy** — merge `develop` → `main` (or run the workflow manually). Migrations
   run automatically on boot.
5. **Verify** — `https://ownstall.up.railway.app/health` returns `{"status":"ok",...}`.

### Frontend → Vercel

```powershell
npm install -g vercel
cd frontend
vercel login          # one-time browser auth
vercel                # link/create the project (framework: Create React App)
vercel --prod
```

1. Vercel dashboard → project → _Settings → Environment Variables_ (Production):
   `REACT_APP_API_URL=https://ownstall.up.railway.app`,
   `REACT_APP_GRAPHQL_URL=https://ownstall.up.railway.app/query`,
   `REACT_APP_STRIPE_PUBLISHABLE_KEY=pk_test_…`
2. Redeploy (`vercel --prod`) so the build picks the variables up.
3. Add a SPA rewrite so deep links like `/admin/products` don't 404 —
   `frontend/vercel.json`:
   ```json
   { "rewrites": [{ "source": "/(.*)", "destination": "/index.html" }] }
   ```
4. Copy the production URL → set it as `FRONTEND_URL` on Railway (Stripe redirects
   and CORS depend on it).

### Post-deploy smoke test

1. Sign up on the live site → create + activate a product
2. Storefront → add to cart → pay with `4242 4242 4242 4242`
3. `/order/success` loads, `/admin/orders` shows the order as **paid**
4. Stripe Dashboard → Webhooks shows `200`s on the endpoint

## Development notes

- **Go has no hot reload** — restart `go run cmd/api/main.go` after backend changes.
- **GraphQL codegen** — after editing `schema.graphqls`:
  `cd backend; go tool gqlgen generate`
  (resolver implementations belong in `schema.resolvers.go`; gqlgen copies them through).
- **Backend tests** — `cd backend; go test ./...` (CI runs them with `-race` against
  Postgres 15). See [Testing](#testing) for what is and is not covered.
- **Local DB GUI** — Redis Commander at `http://localhost:8081`; use any Postgres client
  against `postgresql://postgres:postgres@localhost:5432/ownstall_dev`.

## License

See [LICENSE](LICENSE).
