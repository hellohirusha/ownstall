// Ownstall checkout load test.
//
// Two scenarios run against separate thresholds, because they measure
// different things and averaging them would hide both:
//
//   catalogue — the public storefront read path. GraphQL, Postgres, RLS
//               policy evaluation, and nothing else. This is the number
//               that reflects this codebase.
//
//   checkout  — POST /api/checkout/session, which writes a pending order
//               and then calls Stripe's API. Its latency is mostly
//               Stripe's, and Stripe test mode rate-limits well below
//               where our own system would break, so it runs at a much
//               lower rate and is reported separately. Quoting the two
//               together would be quoting Stripe's p95 as ours.
//
// Setup creates ONE tenant and reuses its token across every VU. The
// obvious alternative — signing up per iteration — benchmarks bcrypt
// (cost 12, ~250ms of CPU per signup) and leaves a tenant row per
// iteration in the database.
//
// Usage:
//   k6 run scripts/test/load_checkout.js --env BASE_URL=https://ownstall.up.railway.app
//
// The per-IP rate limiter (RATE_LIMIT_REQUESTS, default 300/min) will
// throttle this well before the database does. Raise it for the run and
// put it back afterwards.

import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";

const BASE_URL = (__ENV.BASE_URL || "http://localhost:8080").replace(/\/$/, "");

// Orders created by this test are tagged so cleanup can find them.
// Keep in sync with backend/scripts/cleanup_loadtest.go.
const LOADTEST_EMAIL_PREFIX = "loadtest+";

const catalogueOK = new Rate("catalogue_success");
const checkoutOK = new Rate("checkout_success");
const checkoutMs = new Trend("checkout_duration_ms");
const ordersCreated = new Counter("orders_created");

export const options = {
  scenarios: {
    catalogue: {
      executor: "ramping-vus",
      exec: "catalogueBrowse",
      startVUs: 0,
      stages: [
        { duration: "30s", target: 5 },
        { duration: "1m", target: 20 },
        { duration: "2m", target: 20 },
        { duration: "30s", target: 0 },
      ],
      tags: { scenario: "catalogue" },
    },
    checkout: {
      executor: "constant-arrival-rate",
      exec: "checkoutSession",
      // Deliberately modest: every iteration creates a real Stripe test
      // session, and Stripe rate-limits test mode.
      rate: 2,
      timeUnit: "1s",
      duration: "3m",
      preAllocatedVUs: 10,
      maxVUs: 20,
      startTime: "30s",
      tags: { scenario: "checkout" },
    },
  },

  thresholds: {
    // Our own read path. This is the honest latency number.
    "http_req_duration{scenario:catalogue}": ["p(95)<800"],
    catalogue_success: ["rate>0.99"],

    // Includes a synchronous Stripe API call, so the bar is looser and
    // the number is not comparable to the one above.
    "http_req_duration{scenario:checkout}": ["p(95)<3000"],
    checkout_success: ["rate>0.95"],

    // A 429 here means the rate limiter was not raised for the run.
    "http_req_failed{scenario:catalogue}": ["rate<0.01"],
  },
};

function graphql(query, variables, token, tags) {
  return http.post(
    `${BASE_URL}/query`,
    JSON.stringify({ query, variables }),
    {
      headers: {
        "Content-Type": "application/json",
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
      },
      tags,
    },
  );
}

// ── setup: one tenant, one published product, shared by every VU ──

export function setup() {
  const stamp = Date.now();
  const subdomain = `loadtest${stamp}`;

  const signup = http.post(
    `${BASE_URL}/api/signup`,
    JSON.stringify({
      store_name: `Load Test Store ${stamp}`,
      subdomain,
      email: `${LOADTEST_EMAIL_PREFIX}owner${stamp}@ownstall.test`,
      password: "LoadTestPassword123!",
    }),
    { headers: { "Content-Type": "application/json" } },
  );

  if (signup.status !== 201) {
    throw new Error(
      `setup: signup returned ${signup.status}: ${signup.body}. ` +
        `If this is 429, raise RATE_LIMIT_REQUESTS before running.`,
    );
  }

  const auth = JSON.parse(signup.body);
  const token = auth.access_token;
  const tenantId = auth.user.tenant_id;

  const created = graphql(
    `mutation CreateProduct($input: CreateProductInput!) {
       createProduct(input: $input) { id slug variants { id } }
     }`,
    {
      input: {
        name: `Load Test Sticker ${stamp}`,
        description: "A product that exists only to be read under load.",
        basePrice: 4.99,
        tags: ["loadtest"],
      },
    },
    token,
  );

  if (created.status !== 200) {
    throw new Error(`setup: createProduct returned ${created.status}: ${created.body}`);
  }

  const body = JSON.parse(created.body);
  if (body.errors) {
    throw new Error(`setup: createProduct errors: ${JSON.stringify(body.errors)}`);
  }

  const product = body.data.createProduct;
  const variantId = product.variants && product.variants[0] && product.variants[0].id;
  if (!variantId) {
    throw new Error("setup: product was created with no variant to buy");
  }

  // Publish it, or the storefront query returns nothing.
  const published = graphql(
    `mutation Publish($id: UUID!) { publishProduct(id: $id) { id status } }`,
    { id: product.id },
    token,
  );
  if (published.status !== 200) {
    throw new Error(`setup: publishProduct returned ${published.status}: ${published.body}`);
  }

  console.log(`setup: tenant=${tenantId} subdomain=${subdomain} variant=${variantId}`);

  return { token, tenantId, subdomain, variantId, slug: product.slug };
}

// ── scenario: public storefront reads ─────────────────────────

export function catalogueBrowse(data) {
  const tags = { scenario: "catalogue" };

  // Anonymous, exactly as a shopper hits it.
  const list = graphql(
    `query Storefront($tenantId: UUID!) {
       products(tenantId: $tenantId, status: "active") {
         id name basePrice slug
       }
     }`,
    { tenantId: data.tenantId },
    null,
    tags,
  );

  const listOK = check(list, {
    "catalogue: status 200": (r) => r.status === 200,
    "catalogue: no graphql errors": (r) => !JSON.parse(r.body).errors,
    "catalogue: returned the product": (r) =>
      (JSON.parse(r.body).data?.products || []).length > 0,
  });
  catalogueOK.add(listOK);

  if (listOK) {
    const detail = graphql(
      `query Detail($tenantId: UUID!, $slug: String!) {
         productBySlug(tenantId: $tenantId, slug: $slug) {
           id name description basePrice
           variants { id title price stockQuantity }
         }
       }`,
      { tenantId: data.tenantId, slug: data.slug },
      null,
      tags,
    );
    catalogueOK.add(
      check(detail, {
        "detail: status 200": (r) => r.status === 200,
        "detail: no graphql errors": (r) => !JSON.parse(r.body).errors,
      }),
    );
  }

  sleep(1);
}

// ── scenario: checkout session creation (calls Stripe) ────────

export function checkoutSession(data) {
  const started = Date.now();

  const res = http.post(
    `${BASE_URL}/api/checkout/session`,
    JSON.stringify({
      items: [{ variant_id: data.variantId, quantity: 1 }],
      // Tagged so cleanup can find every order this test created.
      customer_email: `${LOADTEST_EMAIL_PREFIX}${__VU}_${__ITER}@ownstall.test`,
    }),
    {
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${data.token}`,
      },
      tags: { scenario: "checkout" },
    },
  );

  checkoutMs.add(Date.now() - started);

  const ok = check(res, {
    "checkout: status 200": (r) => r.status === 200,
    "checkout: returned a stripe url": (r) => {
      try {
        return typeof JSON.parse(r.body).checkout_url === "string";
      } catch {
        return false;
      }
    },
  });

  checkoutOK.add(ok);
  if (ok) ordersCreated.add(1);

  if (res.status === 429) {
    console.warn("checkout: 429 — the rate limiter is capping this run");
  }
}

export function teardown(data) {
  console.log(
    `\nLoad test finished. It left a tenant and its orders behind.\n` +
      `Clean up with:\n` +
      `  cd backend && go run scripts/cleanup_loadtest.go --subdomain ${data.subdomain}\n`,
  );
}
