package middleware

import (
	"net/http"
	"strings"
)

// maxAuthHeaderBytes is generous for a JWT but rejects headers sized
// to probe for buffer handling bugs.
const maxAuthHeaderBytes = 4096

// SecurityHeaders sets defensive response headers.
//
// No Content-Security-Policy here. This service returns JSON, where a
// CSP does nothing, and the one place it would apply — the GraphQL
// playground in development — loads its assets from a CDN that a
// strict policy blocks. The storefront's CSP belongs with the
// storefront, in Vercel's header config.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy",
			"accelerometer=(), camera=(), geolocation=(), microphone=(), payment=()")
		// An API's responses should never be cached by a shared proxy:
		// they are per-tenant and authenticated.
		h.Set("Cache-Control", "no-store")

		// HSTS only over TLS. Sent on a plaintext response it is
		// ignored by browsers, and it would make local development
		// over http painful the moment one leaked through.
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		next.ServeHTTP(w, r)
	})
}

// RequestSanitizer rejects malformed requests before they reach a
// handler.
func RequestSanitizer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(r.Header.Get("Authorization")) > maxAuthHeaderBytes {
			http.Error(w, `{"error":"request header too large"}`,
				http.StatusRequestHeaderFieldsTooLarge)
			return
		}

		// A null byte in a path is never legitimate and is a classic
		// way to confuse downstream string handling.
		if strings.ContainsRune(r.URL.Path, '\x00') ||
			strings.ContainsRune(r.URL.RawQuery, '\x00') {
			http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
			return
		}

		next.ServeHTTP(w, r)
	})
}
