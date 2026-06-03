package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/http/middleware"
)

func TestCORS_NoOriginHeader(t *testing.T) {
	handler := middleware.CORS([]string{"http://example.com"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("expected no CORS header when Origin header is absent")
	}
}

func TestCORS_OriginInAllowList(t *testing.T) {
	const origin = "http://example.com"
	handler := middleware.CORS([]string{origin})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", origin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, origin)
	}
	// An exact (non-wildcard) origin echo is paired with Allow-Credentials: true
	// for an allow-listed origin; a wildcard never is.
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want \"true\" for an exact allowlisted origin", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want %q to prevent cross-origin cache poisoning", got, "Origin")
	}
}

func TestCORS_OriginNotInAllowList(t *testing.T) {
	handler := middleware.CORS([]string{"http://allowed.com"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://notallowed.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// next handler is still called, just without CORS headers
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin, got %q", got)
	}
}

func TestCORS_PreflightAllowedOrigin(t *testing.T) {
	const origin = "http://example.com"
	handler := middleware.CORS([]string{origin})(okHandler())

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", origin)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != origin {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, origin)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Error("expected Access-Control-Allow-Methods to be set on preflight")
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got == "" {
		t.Error("expected Access-Control-Allow-Headers to be set on preflight")
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got == "" {
		t.Error("expected Access-Control-Max-Age to be set on preflight")
	}
}

func TestCORS_PreflightDisallowedOrigin(t *testing.T) {
	handler := middleware.CORS([]string{"http://allowed.com"})(okHandler())

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://notallowed.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Origin for disallowed origin, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Methods for disallowed origin, got %q", got)
	}
}

func TestCORS_EmptyAllowedOrigins(t *testing.T) {
	handler := middleware.CORS([]string{})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("expected no CORS headers with empty allowedOrigins, got %q", got)
	}
}

func TestCORS_Wildcard(t *testing.T) {
	handler := middleware.CORS([]string{"*"})(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://any-origin.com")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	// H4: a wildcard allowlist must emit literal "*", not echo the origin.
	// Echoing origin while wildcard is configured paired with Allow-Credentials
	// would be a browser-rejected misconfig; emitting "*" makes the public
	// intent explicit and prevents any credentialed request from succeeding.
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q (wildcard allowlist)", got, "*")
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Access-Control-Allow-Credentials = %q, want unset (invalid with wildcard)", got)
	}
}

// TestCORS_CredentialsOnlyForExactOrigin locks in the invariant:
// Allow-Credentials is emitted ONLY for an exact allowlisted origin echo, NEVER
// alongside the "*" wildcard (which the browser rejects with credentials and
// which we reserve for an unauthenticated public mirror).
func TestCORS_CredentialsOnlyForExactOrigin(t *testing.T) {
	cases := []struct {
		name     string
		allow    []string
		origin   string
		method   string
		wantCred string
	}{
		{"GET exact origin", []string{"http://example.com"}, "http://example.com", http.MethodGet, "true"},
		{"preflight exact origin", []string{"http://example.com"}, "http://example.com", http.MethodOptions, "true"},
		{"GET wildcard", []string{"*"}, "http://any-origin.com", http.MethodGet, ""},
		{"preflight wildcard", []string{"*"}, "http://any-origin.com", http.MethodOptions, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := middleware.CORS(tc.allow)(okHandler())
			req := httptest.NewRequest(tc.method, "/", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != tc.wantCred {
				t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, tc.wantCred)
			}
		})
	}
}

// okHandler returns a simple 200 OK handler for use in tests.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
