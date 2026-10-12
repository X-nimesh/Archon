// Package secure sets response headers that cost nothing and close off classes
// of attack.
package secure

import (
	"net/http"
	"strings"
)

// Headers applies defensive response headers.
//
// This is a JSON API consumed by a server-side fetch, so the browser never
// renders its responses directly. The CSP is therefore intentionally strict
// rather than permissive: if any HTML is ever served from this origin, it has
// no business executing anything.
func Headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()

		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// 'unsafe-inline' is required by the CSS-in-JS style tags Next.js
		// emits, but this API serves JSON only, so none is needed here.
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")

		// Auth responses contain credentials-adjacent material; keep them out
		// of shared and browser caches.
		if strings.HasPrefix(r.URL.Path, "/auth/") {
			h.Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			h.Set("Pragma", "no-cache")
		}

		next.ServeHTTP(w, r)
	})
}
