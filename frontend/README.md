# Ownstall — web app

React 19 + TypeScript (Create React App), Tailwind CSS, Apollo Client v4.
Serves all three Ownstall audiences from one bundle.

See the [root README](../README.md) for the platform overview and full local
setup.

## Run it

```bash
npm install
cp .env.example .env    # point REACT_APP_* at your local API
npm start               # http://localhost:3000
```

The API must be running on the URL in `REACT_APP_API_URL` (default
`http://localhost:8080`). CRA bakes `REACT_APP_*` in at **build time** — change
one and you must restart `npm start` or rebuild.

| Script             | What it does                    |
| ------------------ | ------------------------------- |
| `npm start`        | Dev server with fast refresh    |
| `npm run build`    | Production bundle into `build/` |
| `npm test`         | Jest + Testing Library          |
| `npx tsc --noEmit` | Typecheck without emitting      |

## Routes

| Path                                | Audience | Notes                                                                                                   |
| ----------------------------------- | -------- | ------------------------------------------------------------------------------------------------------- |
| `/`                                 | public   | Landing page                                                                                            |
| `/stores`                           | public   | Stall directory — search, category, sort, paging. State lives in the URL, so a result page is shareable |
| `/store?store=<subdomain>`          | public   | A stall's storefront                                                                                    |
| `/cart`, `/order/success`           | public   | Guest or signed-in checkout                                                                             |
| `/about`, `/contact`                | public   | Company pages                                                                                           |
| `/terms`, `/privacy`                | public   | Legal                                                                                                   |
| `/signup`, `/login`                 | seller   | Open a stall / sign in                                                                                  |
| `/admin/*`                          | seller   | Dashboard — products, orders, notify, reply, hire, production, AI                                       |
| `/account/signup`, `/account/login` | buyer    | Shopper accounts                                                                                        |
| `/account`                          | buyer    | Order history across every stall                                                                        |
| `/platform/login`, `/platform`      | operator | Approval queue, moderation, platform stats                                                               |

## Sessions

Three audiences can be signed in at once — a seller shopping on someone else's
stall is a normal thing to do — so [`src/lib/session.ts`](src/lib/session.ts)
keeps one token set per scope under separate `localStorage` keys.

Which scope a request uses is decided by **the route**, not by which tokens
happen to exist: `/platform/*` uses the operator token, `/admin/*` the seller
token, everything else the buyer token (absent for guests). `RequireAuth` gates
routes. [`src/lib/apollo.ts`](src/lib/apollo.ts) refreshes an expired token once
and retries the failed request, and only redirects to a sign-in page if the
visitor actually had a session — a guest hitting a protected field sees the
error instead of being bounced somewhere they never asked to go.

## Theme

The palette lives in [`tailwind.config.js`](tailwind.config.js) — teal `brand`,
amber `accent`, and an `ink` neutral scale that also overrides Tailwind's
default `gray` so existing pages inherit it. `src/index.css` mirrors the same
values as CSS variables. Keep it in step with `mobile/src/theme.ts`.

Semantic colours (`green` for paid/in-stock, `red` for errors, `amber` for
warnings) are deliberately **not** part of the brand scale — a "paid" badge
must not change meaning because the brand colour changed.

The logo is [`src/components/Logo.tsx`](src/components/Logo.tsx); the favicon
and PWA icons in `public/` are drawn from the same 48-unit geometry.
