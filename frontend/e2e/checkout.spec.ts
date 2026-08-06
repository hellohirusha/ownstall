import { test, expect, type APIRequestContext, type Page } from "@playwright/test";
import {
  API_URL,
  adminToken,
  approveStore,
  createPublishedProduct,
  createStore,
  graphql,
  type Product,
  type Store,
} from "./helpers";

// A stall opens in "pending" and stays invisible until a platform
// operator approves it — its products are not publicly listed and it
// cannot take money. That gate shapes this file: everything a shopper
// does needs an approved stall, so those specs require operator
// credentials and skip without them. See e2e/README.md.

async function addFirstProductToCart(page: Page, store: Store, product: Product) {
  await page.goto(`/store?store=${store.subdomain}`);
  await page.getByRole("link", { name: new RegExp(product.name, "i") }).first().click();
  await page.getByRole("button", { name: /Add to cart/i }).click();
  await expect(page.getByRole("button", { name: /Added to cart/i })).toBeVisible();
}

// ── The gate itself, which needs no operator ──────────────────

test.describe("Unapproved stall", () => {
  let store: Store;
  let product: Product;

  test.beforeEach(async ({ request }) => {
    store = await createStore(request);
    product = await createPublishedProduct(request, store);
  });

  test("its products are not publicly listed", async ({ request }) => {
    // The product is active. The stall is not approved. Anonymous
    // traffic must see nothing.
    const res = await request.post(`${API_URL}/query`, {
      data: {
        query: `query Storefront($tenantId: UUID!) {
          products(tenantId: $tenantId, status: "active") { id name }
        }`,
        variables: { tenantId: store.tenantId },
      },
    });

    const body = await res.json();
    expect(body.errors).toBeUndefined();
    expect(body.data.products).toHaveLength(0);

    // The owner still sees their own draft catalogue.
    const owner = await graphql(request, store, `query { products { id status } }`);
    expect(owner.products).toHaveLength(1);
    expect(owner.products[0].status).toBe("active");
  });

  test("it cannot create a checkout session", async ({ request }) => {
    const res = await request.post(`${API_URL}/api/checkout/session`, {
      headers: { Authorization: `Bearer ${store.token}` },
      data: {
        items: [{ variant_id: product.variantId, quantity: 1 }],
        customer_email: "buyer@ownstall.test",
      },
    });

    expect(res.status()).toBe(400);
    expect(await res.text()).toContain("not currently accepting orders");

    // The attempt left no order behind.
    const data = await graphql(request, store, `query { orders { id } }`);
    expect(data.orders).toHaveLength(0);
  });
});

// ── Everything a shopper does, on an approved stall ───────────

test.describe("Approved stall", () => {
  let admin: string | null = null;

  test.beforeAll(async ({ playwright }) => {
    const ctx = await playwright.request.newContext();
    admin = await adminToken(ctx);
    await ctx.dispose();
  });

  test.beforeEach(() => {
    test.skip(
      !admin,
      "set E2E_ADMIN_EMAIL and E2E_ADMIN_PASSWORD (matching the API's PLATFORM_ADMIN_*) to run these",
    );
  });

  async function openStall(request: APIRequestContext) {
    const store = await createStore(request);
    await approveStore(request, admin!, store.tenantId);
    const product = await createPublishedProduct(request, store);
    return { store, product };
  }

  test("the storefront lists the published product", async ({ page, request }) => {
    const { store, product } = await openStall(request);

    await page.goto(`/store?store=${store.subdomain}`);

    await expect(page.getByRole("heading", { name: /Welcome to/i })).toBeVisible();
    await expect(
      page.getByRole("link", { name: new RegExp(product.name, "i") }),
    ).toBeVisible();
  });

  test("a product page adds to the cart", async ({ page, request }) => {
    const { store, product } = await openStall(request);

    await page.goto(`/store?store=${store.subdomain}`);
    await page.getByRole("link", { name: new RegExp(product.name, "i") }).first().click();

    await expect(page).toHaveURL(
      new RegExp(`/store/${store.subdomain}/products/${product.slug}`),
    );
    await expect(page.getByRole("heading", { name: product.name })).toBeVisible();

    const addToCart = page.getByRole("button", { name: /Add to cart/i });
    await expect(addToCart).toBeEnabled();
    await addToCart.click();

    // The button confirms in place rather than navigating.
    await expect(page.getByRole("button", { name: /Added to cart/i })).toBeVisible();

    await page.goto("/cart");
    await expect(page.getByRole("heading", { name: /Cart \(1 item\)/i })).toBeVisible();
    await expect(page.getByText(product.name)).toBeVisible();
  });

  test("the pay button stays disabled until an email is entered", async ({ page, request }) => {
    const { store, product } = await openStall(request);

    await addFirstProductToCart(page, store, product);
    await page.goto("/cart");

    const pay = page.getByRole("button", { name: /Pay \$.* with Stripe/i });
    await expect(pay).toBeDisabled();

    await page.getByPlaceholder("you@example.com").fill("e2e-customer@ownstall.test");
    await expect(pay).toBeEnabled();
  });

  test("an empty cart offers no way to pay", async ({ page, request }) => {
    const { store } = await openStall(request);

    // Visit the storefront first so /cart can resolve the tenant —
    // useTenant remembers the subdomain in localStorage.
    await page.goto(`/store?store=${store.subdomain}`);
    await page.goto("/cart");

    await expect(page.getByRole("button", { name: /Pay \$.* with Stripe/i })).toHaveCount(0);
  });

  test("checkout hands off to Stripe", async ({ page, request }) => {
    const { store, product } = await openStall(request);

    await addFirstProductToCart(page, store, product);
    await page.goto("/cart");
    await page.getByPlaceholder("you@example.com").fill("e2e-customer@ownstall.test");
    await page.getByRole("button", { name: /Pay \$.* with Stripe/i }).click();

    // Reaching Stripe's hosted page proves the API priced the cart,
    // wrote a pending order, and got a session URL back. Completing the
    // payment would mean driving Stripe's own DOM, and the order only
    // becomes "paid" once their webhook lands — neither belongs in a
    // suite that has to pass unattended.
    await expect(page).toHaveURL(/checkout\.stripe\.com/, { timeout: 30_000 });
  });

  test("checkout writes a pending order priced from the variant", async ({ page, request }) => {
    const { store, product } = await openStall(request);

    await addFirstProductToCart(page, store, product);
    await page.goto("/cart");
    await page.getByPlaceholder("you@example.com").fill("e2e-customer@ownstall.test");
    await page.getByRole("button", { name: /Pay \$.* with Stripe/i }).click();
    await expect(page).toHaveURL(/checkout\.stripe\.com/, { timeout: 30_000 });

    const data = await graphql(
      request,
      store,
      `query { orders { id status customerEmail total } }`,
    );

    expect(data.orders).toHaveLength(1);
    expect(data.orders[0].status).toBe("pending");
    expect(data.orders[0].customerEmail).toBe("e2e-customer@ownstall.test");
    expect(data.orders[0].total).toBeCloseTo(product.price, 2);
  });

  // Isolation is enforced by Postgres row-level security rather than by
  // the resolvers, so it is worth proving from outside.
  test("one stall cannot see another's orders", async ({ request }) => {
    const { store: storeA, product: productA } = await openStall(request);
    const storeB = await createStore(request);

    const checkout = await request.post(`${API_URL}/api/checkout/session`, {
      headers: { Authorization: `Bearer ${storeA.token}` },
      data: {
        items: [{ variant_id: productA.variantId, quantity: 1 }],
        customer_email: "buyer@ownstall.test",
      },
    });
    expect(checkout.status()).toBe(200);

    const seenByA = await graphql(request, storeA, `query { orders { id total } }`);
    expect(seenByA.orders).toHaveLength(1);

    const seenByB = await graphql(request, storeB, `query { orders { id total } }`);
    expect(seenByB.orders).toHaveLength(0);
  });
});
