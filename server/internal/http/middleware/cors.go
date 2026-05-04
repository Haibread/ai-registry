package middleware

import (
	"net/http"
	"slices"
)

// CORS returns a middleware that enforces Cross-Origin Resource Sharing policy.
// If allowedOrigins is empty, CORS headers are not set (defaults to deny).
// Pass []string{"*"} only for fully public APIs; the registry uses an explicit list.
//
// The API is bearer-only — auth travels in the Authorization header, which
// CORS does NOT treat as credentials. We therefore never set
// Access-Control-Allow-Credentials: true. This also avoids the invalid
// browser combo "Allow-Origin: *" + "Allow-Credentials: true", which would
// otherwise be silently misconfigurable via the allowlist.
//
// When a wildcard ("*") is configured, we emit "Allow-Origin: *" rather than
// echoing the request origin, so caches can't fingerprint per-origin responses
// and no credentialed request can ever succeed cross-origin.
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
						w.Header().Set("Vary", "Origin")
					}
				}
				// Preflight
				if r.Method == http.MethodOptions {
					if allowed {
						w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
						w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Request-ID")
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
