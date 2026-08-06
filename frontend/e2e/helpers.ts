import type { APIRequestContext } from "@playwright/test";

export const API_URL = process.env.E2E_API_URL ?? "http://localhost:8080";

// Tenants created by E2E runs share this subdomain prefix so they can be
// found and removed later. Matches the namespace understood by
// backend/scripts/cleanup_loadtest.go.
const E2E_PREFIX = "loadteste2e";

export interface Store {
  token: string;
  tenantId: string;
  subdomain: string;
  email: string;
}

export interface Product {
  id: string;
  slug: string;
  name: string;
  variantId: string;
  price: number;
}

/** Creates a fresh tenant. Every spec gets its own so they can run in parallel. */
export async function createStore(api: APIRequestContext): Promise<Store> {
  const stamp = `${Date.now()}${Math.floor(Math.random() * 1000)}`;
  const subdomain = `${E2E_PREFIX}${stamp}`;
  const email = `${E2E_PREFIX}+${stamp}@ownstall.test`;

  const res = await api.post(`${API_URL}/api/signup`, {
    data: {
      store_name: `E2E Store ${stamp}`,
      subdomain,
      email,
      password: "E2ETestPassword123!",
    },
  });

  if (res.status() !== 201) {
    throw new Error(`signup failed (${res.status()}): ${await res.text()}`);
  }

  const body = await res.json();
  return {
    token: body.access_token,
    tenantId: body.user.tenant_id,
    subdomain,
    email,
  };
}

/** Runs a GraphQL operation as the given store's owner. */
export async function graphql(
  api: APIRequestContext,
  store: Store,
  query: string,
  variables: Record<string, unknown> = {},
) {
  const res = await api.post(`${API_URL}/query`, {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${store.token}`,
    },
    data: { query, variables },
  });

  const body = await res.json();
  if (body.errors) {
    throw new Error(`GraphQL errors: ${JSON.stringify(body.errors)}`);
  }
  return body.data;
}

/** Creates a product and publishes it, so the storefront will show it. */
export async function createPublishedProduct(
  api: APIRequestContext,
  store: Store,
  label = "Sticker",
  price = 4.99,
): Promise<Product> {
  // The name LEADS with random characters on purpose. CreateProduct
  // derives the default variant's SKU from the first eight characters of
  // the slug, and product_variants.sku is globally unique — so two
  // products whose names share a leading eight characters cannot both
  // exist, even in different tenants.
  //
  // A timestamp does not work here: the first eight digits of a
  // millisecond timestamp only change every ~100 seconds, so a whole
  // test run lands on one SKU. Ten random base36 characters give the
  // prefix real entropy. See e2e/README.md.
  const token = Math.random().toString(36).slice(2, 12);
  const name = `${token} ${label}`;

  const created = await graphql(
    api,
    store,
    `mutation CreateProduct($input: CreateProductInput!) {
       createProduct(input: $input) { id slug name variants { id price } }
     }`,
    {
      input: {
        name,
        description: "A die-cut vinyl sticker that exists to be tested.",
        basePrice: price,
        tags: ["e2e"],
      },
    },
  );

  const product = created.createProduct;
  const variant = product.variants?.[0];
  if (!variant) {
    throw new Error("product was created with no variant to buy");
  }

  // Unpublished products do not appear on the storefront.
  await graphql(
    api,
    store,
    `mutation Publish($id: UUID!) { publishProduct(id: $id) { id status } }`,
    { id: product.id },
  );

  return {
    id: product.id,
    slug: product.slug,
    name: product.name,
    variantId: variant.id,
    price: variant.price,
  };
}

/**
 * Logs in as a platform operator, or returns null when no credentials are
 * configured.
 *
 * A stall cannot take money until an operator approves it, so the specs
 * that reach checkout need this. The bootstrap operator is created from
 * PLATFORM_ADMIN_* on API boot; set E2E_ADMIN_EMAIL and E2E_ADMIN_PASSWORD
 * to the same values to unlock those specs.
 */
export async function adminToken(api: APIRequestContext): Promise<string | null> {
  const email = process.env.E2E_ADMIN_EMAIL;
  const password = process.env.E2E_ADMIN_PASSWORD;
  if (!email || !password) return null;

  const res = await api.post(`${API_URL}/api/admin/login`, {
    data: { email, password },
  });
  if (res.status() !== 200) return null;

  return (await res.json()).access_token;
}

/** Approves a stall so it can accept orders. */
export async function approveStore(
  api: APIRequestContext,
  adminAccessToken: string,
  tenantId: string,
) {
  const res = await api.post(`${API_URL}/query`, {
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${adminAccessToken}`,
    },
    data: {
      query: `mutation Approve($tenantId: UUID!, $note: String) {
        approveStore(tenantId: $tenantId, note: $note) { id status canAcceptOrders }
      }`,
      variables: { tenantId, note: "approved by the e2e suite" },
    },
  });

  const body = await res.json();
  if (body.errors) {
    throw new Error(`approveStore failed: ${JSON.stringify(body.errors)}`);
  }
  return body.data.approveStore;
}
