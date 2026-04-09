package middleware

import (
	"net/http"
	"strings"

	"github.com/haibread/ai-registry/internal/problem"
)

// SecurityHeaders sets standard defensive HTTP response headers on every
// response. Apply near the top of the middleware chain.
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// MaxBodySize returns middleware that caps the request body to maxBytes.
// Requests whose Content-Length already exceeds the limit are rejected before
// the body is read. Reading beyond the limit causes the decoder to receive an
// *http.MaxBytesError, which decodeJSON (in handlers/health.go) converts to a
// 413 response.
func MaxBodySize(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireJSONBody rejects POST, PUT, and PATCH requests whose Content-Type
// header does not start with "application/json". GET, DELETE, and other safe
// methods pass through unchanged.
func RequireJSONBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			ct := r.Header.Get("Content-Type")
			if !strings.HasPrefix(ct, "application/json") {
				problem.Write(w, http.StatusUnsupportedMediaType, "unsupported-media-type",
					"Content-Type must be application/json", r.URL.Path)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
