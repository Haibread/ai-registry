package middleware

import (
	"net/http"
	"slices"
)

// CORS returns a middleware that enforces Cross-Origin Resource Sharing policy.
// If allowedOrigins is empty, CORS headers are not set (defaults to deny).
// Pass []string{"*"} only for fully public APIs; the registry uses an explicit list.
//
// Auth is a Secure; HttpOnly session cookie, and the SPA fetches with
// credentials: 'include'. For a cross-origin SPA deployment to work, the browser
// requires Access-Control-Allow-Credentials: true together with an exact
// (non-wildcard) Allow-Origin echo. We therefore set Allow-Credentials only for
// an explicitly allow-listed origin.
//
// A wildcard ("*") is incompatible with credentials by spec, so when "*" is
// configured we emit "Allow-Origin: *" WITHOUT Allow-Credentials: caches can't
// fingerprint per-origin responses and no credentialed (cookie) request can
// succeed cross-origin — wildcard is for an unauthenticated public mirror only.
// Cross-site state-changing requests are additionally blocked by
// EnforceSameOrigin regardless of CORS.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	wildcard := slices.Contains(allowedOrigins, "*")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && len(allowedOrigins) > 0 {
				allowed := wildcard || slices.Contains(allowedOrigins, origin)
				if allowed {
					if wildcard {
						w.Header().Set("Access-Control-Allow-Origin", "*")
					} else {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Add("Vary", "Origin")
						// Cookie auth is cross-origin-credentialed; safe only with
						// an exact origin echo (never with the wildcard above).
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
				}
				// Preflight
				if r.Method == http.MethodOptions {
					if allowed {
						w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
						w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Request-ID")
						w.Header().Set("Access-Control-Max-Age", "86400")
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
