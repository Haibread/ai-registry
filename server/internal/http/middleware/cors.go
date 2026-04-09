package middleware

import (
	"net/http"
	"slices"
)

// CORS returns a middleware that enforces Cross-Origin Resource Sharing policy.
// If allowedOrigins is empty, CORS headers are not set (defaults to deny).
// Pass []string{"*"} only for fully public APIs; the registry uses an explicit list.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && len(allowedOrigins) > 0 {
				allowed := slices.Contains(allowedOrigins, "*") || slices.Contains(allowedOrigins, origin)
				if allowed {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
					w.Header().Set("Vary", "Origin")
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
