package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
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
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "" {
		t.Errorf("expected no Access-Control-Allow-Headers for disallowed origin, got %q", got)
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
	// A wildcard allowlist emits a literal "*" rather than echoing the origin.
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q (wildcard allowlist)", got, "*")
	}
}

// TestCORS_NeverCredentialed locks in that Allow-Credentials is never sent:
// auth is a bearer header, so no request needs ambient credentials.
func TestCORS_NeverCredentialed(t *testing.T) {
	cases := []struct {
		name   string
		allow  []string
		origin string
		method string
	}{
		{"GET exact origin", []string{"http://example.com"}, "http://example.com", http.MethodGet},
		{"preflight exact origin", []string{"http://example.com"}, "http://example.com", http.MethodOptions},
		{"GET wildcard", []string{"*"}, "http://any-origin.com", http.MethodGet},
		{"preflight wildcard", []string{"*"}, "http://any-origin.com", http.MethodOptions},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := middleware.CORS(tc.allow)(okHandler())
			req := httptest.NewRequest(tc.method, "/", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "" {
				t.Fatal("expected Access-Control-Allow-Origin to be set")
			}
			if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "" {
				t.Errorf("Access-Control-Allow-Credentials = %q, want unset", got)
			}
		})
	}
}

// TestCORS_PreflightAllowsRequestHeaders covers every request header a
// cross-origin client sends: the bearer token, the JSON body type, and the
// caller-supplied correlation id.
func TestCORS_PreflightAllowsRequestHeaders(t *testing.T) {
	cases := []struct {
		name   string
		allow  []string
		origin string
	}{
		{"exact origin", []string{"http://example.com"}, "http://example.com"},
		{"wildcard", []string{"*"}, "http://any-origin.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := middleware.CORS(tc.allow)(okHandler())
			req := httptest.NewRequest(http.MethodOptions, "/", nil)
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)
			req.Header.Set("Access-Control-Request-Headers", "authorization,content-type")
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			allowed := headerTokens(rec.Header().Get("Access-Control-Allow-Headers"))
			for _, want := range []string{"authorization", "content-type", "x-request-id"} {
				if !slices.Contains(allowed, want) {
					t.Errorf("Access-Control-Allow-Headers = %q, missing %q",
						rec.Header().Get("Access-Control-Allow-Headers"), want)
				}
			}
		})
	}
}

func TestCORS_ExposeHeaders(t *testing.T) {
	cases := []struct {
		name   string
		allow  []string
		origin string
		method string
		want   []string
	}{
		{"GET exact origin", []string{"http://example.com"}, "http://example.com", http.MethodGet, []string{"retry-after", "x-request-id"}},
		{"GET wildcard", []string{"*"}, "http://any-origin.com", http.MethodGet, []string{"retry-after", "x-request-id"}},
		{"GET disallowed origin", []string{"http://allowed.com"}, "http://notallowed.com", http.MethodGet, nil},
		{"GET empty allow-list", []string{}, "http://example.com", http.MethodGet, nil},
		{"preflight exact origin", []string{"http://example.com"}, "http://example.com", http.MethodOptions, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := middleware.CORS(tc.allow)(okHandler())
			req := httptest.NewRequest(tc.method, "/", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			got := headerTokens(rec.Header().Get("Access-Control-Expose-Headers"))
			slices.Sort(got)
			if !slices.Equal(got, tc.want) {
				t.Errorf("Access-Control-Expose-Headers = %v, want %v", got, tc.want)
			}
		})
	}
}

func headerTokens(v string) []string {
	var out []string
	for _, tok := range strings.Split(v, ",") {
		if tok = strings.ToLower(strings.TrimSpace(tok)); tok != "" {
			out = append(out, tok)
		}
	}
	return out
}

// okHandler returns a simple 200 OK handler for use in tests.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
