import { request } from "@playwright/test";
import { API_URL } from "./helpers";

// Playwright's webServer starts the React dev server. The Go API is a
// separate process that it does not manage, and when it is not running
// every spec fails with an opaque timeout in the middle of a browser
// step. Checking once here turns that into one readable error.
export default async function globalSetup() {
  const ctx = await request.newContext();

  try {
    const res = await ctx.get(`${API_URL}/health`, { timeout: 5_000 });
    if (!res.ok()) {
      throw new Error(`${API_URL}/health returned ${res.status()}`);
    }
  } catch (cause) {
    throw new Error(
      `The Ownstall API is not reachable at ${API_URL}.\n\n` +
        `Start it first:\n` +
        `  cd backend && go run cmd/api/main.go\n\n` +
        `Or point the tests elsewhere with E2E_API_URL.\n\n` +
        `Note that /health is a static 200 — it confirms the process is\n` +
        `routing, not that Postgres is reachable.\n\n` +
        `Cause: ${(cause as Error).message}`,
    );
  } finally {
    await ctx.dispose();
  }
}
