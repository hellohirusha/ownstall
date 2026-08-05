import * as Sentry from "@sentry/react";

// initSentry is a no-op without a DSN, so local development and
// preview builds never report to the production project.
export function initSentry() {
  const dsn = process.env.REACT_APP_SENTRY_DSN;
  if (!dsn) return;

  Sentry.init({
    dsn,
    environment: process.env.NODE_ENV,
    release: process.env.REACT_APP_VERSION,
    // The free tier allows 5k events a month; a single loop of broken
    // renders can exhaust that in minutes.
    tracesSampleRate: 0.1,

    beforeSend(event) {
      // Auth tokens live in localStorage and end up in request
      // headers — strip them before anything leaves the browser.
      if (event.request?.headers) {
        delete event.request.headers["Authorization"];
        delete event.request.headers["Cookie"];
      }
      if (event.request) {
        delete event.request.cookies;
        // Query strings carry order and campaign identifiers
        delete event.request.query_string;
      }
      return event;
    },

    // Noise that is not actionable: browser extension failures, users
    // going offline mid-request, and stale chunks after a deploy.
    ignoreErrors: [
      "ResizeObserver loop limit exceeded",
      "ResizeObserver loop completed with undelivered notifications",
      "Network request failed",
      "Failed to fetch",
      "ChunkLoadError",
      /^Script error/,
    ],
    denyUrls: [/extensions\//i, /^chrome:\/\//i, /^moz-extension:\/\//i],
  });
}
