import { defineConfig, devices } from "@playwright/test";

// The API is a separate Go process. Playwright starts the React dev
// server but not the backend — see e2e/global-setup.ts, which fails
// fast with a readable message rather than letting every spec time out.
const FRONTEND_URL = process.env.E2E_FRONTEND_URL ?? "http://localhost:3000";

export default defineConfig({
  testDir: "./e2e",
  globalSetup: "./e2e/global-setup.ts",

  // Each spec creates its own tenant, so specs do not contend.
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,

  // CRA's dev server is slow to compile on first hit, and the checkout
  // step waits on a redirect to Stripe.
  timeout: 60_000,
  expect: { timeout: 15_000 },

  reporter: process.env.CI ? [["github"], ["html"]] : [["list"], ["html", { open: "never" }]],

  use: {
    baseURL: FRONTEND_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
    video: "on-first-retry",
  },

  // Chromium only. A second browser project doubles the runtime and has
  // never caught a bug in this app that chromium did not.
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],

  webServer: {
    command: "npm start",
    url: FRONTEND_URL,
    reuseExistingServer: !process.env.CI,
    timeout: 180_000,
    // CRA opens a browser tab on boot otherwise.
    env: { BROWSER: "none" },
  },
});
