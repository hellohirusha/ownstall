# Ownstall

> Multi-tenant commerce platform — every creator gets their own storefront under one roof.
> Built with Go, GraphQL, React, TypeScript, and Postgres.

## Status

| Goal                                    | Status |
| --------------------------------------- | ------ |
| Auth (JWT) + GraphQL foundation         | ✅     |
| Product catalog + image uploads         | ✅     |
| Cart, checkout, orders, Stripe webhooks | ✅     |
| Notify (email engine)                   | ⏳     |
| Reply, Hire Me, mobile app              | ⏳     |

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
| Cache    | Redis 7                | Provisioned; queue/cache work planned |
| Frontend | React 19 + TypeScript  | CRA, Tailwind CSS, Apollo Client v4   |
| State    | Zustand                | Persistent cart store                 |
| Auth     | JWT (HS256)            | Access + refresh tokens               |
| Payments | Stripe                 | Hosted Checkout + signed webhooks     |
| Images   | Cloudinary             | Upload endpoint + CDN delivery        |
| Email    | Resend                 | Planned                               |
| AI       | Groq (Llama)           | Planned                               |
| Mobile   | Expo                   | Planned                               |

## Project structure

```
ownstall/
├── backend/            Go API (Chi router, gqlgen GraphQL, handlers, services)
│   ├── cmd/api/        Entrypoint
│   ├── graph/          GraphQL schema + resolvers
│   ├── internal/       auth, handlers, middleware, models, services
│   └── pkg/database/   Connection + SQL migrations (run on boot)
├── frontend/           React + TypeScript app (CRA)
│   └── src/            pages (admin, store, auth), lib, hooks, components
├── mobile/             Expo app (scaffold)
├── scripts/            deploy / seed / test helpers
├── docs/               Documentation (in progress)
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

### Test the checkout flow

1. `http://localhost:3000/signup` → create a store
2. `/admin/products/new` → create a product
3. `/store?store=<your-subdomain>` → open your storefront
4. Product → **Add to cart** → `/cart` → checkout
5. Pay with Stripe's test card `4242 4242 4242 4242` (any future expiry, any CVC)
6. You land on `/order/success`; the webhook marks the order **paid**
7. `/admin/orders` → the order appears with status `paid`

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
| `REDIS_URL`                                                            | ⏳           | Provisioned for upcoming queue/cache work                                    |
| `RESEND_API_KEY`, `EMAIL_FROM`, `EMAIL_FROM_NAME`                      | ⏳           | Email engine                                                                 |
| `GROQ_API_KEY`, `GROQ_MODEL`, `AI_MONTHLY_COST_LIMIT_USD`              | ⏳           | AI features (planned)                                                        |
| `SENTRY_DSN`                                                           | ⏳           | Error tracking (optional)                                                    |

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
| `POST /api/signup` / `login` / `refresh` / `logout` | —                | JWT auth                                                     |
| `POST /query`                                       | Optional         | GraphQL (admin queries need `Authorization: Bearer <token>`) |
| `GET /playground`                                   | —                | GraphQL playground (non-production only)                     |
| `POST /api/checkout/session`                        | ✅               | Create Stripe Checkout session + pending order               |
| `POST /webhooks/stripe`                             | Stripe signature | Marks orders paid, handles expiry/failure                    |
| `POST /api/upload/product-image`                    | ✅               | Cloudinary product image upload                              |

GraphQL: products / product / productBySlug / tenant / orders queries,
createProduct + product mutations. Schema: [`backend/graph/schema.graphqls`](backend/graph/schema.graphqls)

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
- **Backend tests** — `cd backend; go test ./...` (CI runs them against Postgres 15).
- **Local DB GUI** — Redis Commander at `http://localhost:8081`; use any Postgres client
  against `postgresql://postgres:postgres@localhost:5432/ownstall_dev`.

## License

See [LICENSE](LICENSE).
