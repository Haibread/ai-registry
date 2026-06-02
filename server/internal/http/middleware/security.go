package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/haibread/ai-registry/internal/problem"
)

// apiContentSecurityPolicy is a deny-by-default CSP suitable for the JSON/YAML
// API surface this server emits: nothing should ever be loaded or framed from
// an API response. The /docs (Scalar) HTML handler overrides this header with a
// policy that permits its CDN bundle.
const apiContentSecurityPolicy = "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'"

// hstsValue is sent only when the deployment serves over HTTPS (Secure cookies).
// Two years, including subdomains; preload is intentionally omitted so operators
// opt into the preload list deliberately.
const hstsValue = "max-age=63072000; includeSubDomains"

// SecurityHeaders returns middleware that sets standard defensive HTTP response
// headers on every response. Apply near the top of the middleware chain.
// enableHSTS gates the Strict-Transport-Security header: pass true only when the
// server is reached over HTTPS (mirror the Secure-cookie setting), since HSTS on
// a plain-HTTP deployment would be meaningless and could lock out a local
// http:// dev setup once a browser has cached it.
func SecurityHeaders(enableHSTS bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Content-Security-Policy", apiContentSecurityPolicy)
			if enableHSTS {
				w.Header().Set("Strict-Transport-Security", hstsValue)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// unsafeMethod reports whether the HTTP method can change server state and so
// must carry a trustworthy same-origin signal before it is allowed through.
func unsafeMethod(m string) bool {
	switch m {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

// EnforceSameOrigin is the registry's CSRF defense. For state-changing methods
// it requires the request to originate from a trusted same-site context. It
// prefers the Fetch Metadata `Sec-Fetch-Site` header (sent by all current
// browsers) and falls back to an `Origin` check for clients that don't send it.
// Combined with the session cookie's SameSite attribute and the JSON
// content-type requirement, this blocks cross-site forged writes even when
// SameSite is relaxed to "none" for a cross-origin SPA deployment.
//
// allowedOrigins is the cross-origin allowlist (the same list CORS uses); a
// request whose Origin matches the target host is always treated as same-origin,
// so an empty allowlist still permits the normal single-origin deployment.
// Safe methods (GET/HEAD/OPTIONS) pass through untouched.
func EnforceSameOrigin(allowedOrigins []string) func(http.Handler) http.Handler {
	allow := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allow[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !unsafeMethod(r.Method) {
				next.ServeHTTP(w, r)
				return
			}

			// Modern browsers label every request with its relationship to the
			// target origin. "same-origin"/"same-site" are trusted; "none" is a
			// user-initiated action (typed URL, bookmark); "cross-site" is the
			// CSRF vector we reject.
			if site := r.Header.Get("Sec-Fetch-Site"); site != "" {
				switch site {
				case "same-origin", "same-site", "none":
					next.ServeHTTP(w, r)
				default:
					rejectCrossOrigin(w, r)
				}
				return
			}

			// No Fetch Metadata (older browser or a non-browser client). An
			// absent Origin means a non-browser client — which carries no
			// ambient cookies to forge — or a same-origin navigation; allow it.
			origin := r.Header.Get("Origin")
			if origin == "" || originMatchesHost(origin, r.Host) {
				next.ServeHTTP(w, r)
				return
			}
			if _, ok := allow[origin]; ok {
				next.ServeHTTP(w, r)
				return
			}
			rejectCrossOrigin(w, r)
		})
	}
}

// originMatchesHost reports whether the scheme://host[:port] Origin refers to
// the same host:port the request targeted (r.Host), i.e. a same-origin request.
func originMatchesHost(origin, host string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	return u.Host == host
}

func rejectCrossOrigin(w http.ResponseWriter, r *http.Request) {
	problem.Write(w, http.StatusForbidden, "cross-origin-forbidden",
		"cross-origin state-changing request rejected", r.URL.Path)
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
// methods pass through unchanged. Bodyless requests (Content-Length: 0) are
// also allowed — endpoints like /view and /copy trigger side effects from the
// URL alone and have no payload to validate.
func RequireJSONBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			if r.ContentLength == 0 {
				break
			}
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
