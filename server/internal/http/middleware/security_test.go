package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/haibread/ai-registry/internal/http/middleware"
)

func TestRequireJSONBody_AllowsBodylessPOST(t *testing.T) {
	// Endpoints like /view and /copy POST with no body to record an event.
	// The middleware must not reject them for lack of a Content-Type header.
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns/slug/view", nil)
	req.ContentLength = 0
	rec := httptest.NewRecorder()

	middleware.RequireJSONBody(next).ServeHTTP(rec, req)

	if !called {
		t.Fatalf("expected next handler to be invoked for bodyless POST, got %d", rec.Code)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestRequireJSONBody_RejectsPOSTWithWrongContentType(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", strings.NewReader(`hello`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	middleware.RequireJSONBody(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", rec.Code)
	}
}

func TestRequireJSONBody_AllowsPOSTWithJSONContentType(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	middleware.RequireJSONBody(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be invoked")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireJSONBody_PassesThroughGET(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
	rec := httptest.NewRecorder()

	middleware.RequireJSONBody(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be invoked for GET")
	}
}

func TestSecurityHeaders_AlwaysSetCSPAndDefaults(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	middleware.SecurityHeaders(false)(okHandler()).ServeHTTP(rec, req)

	for k, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "no-referrer",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if got := rec.Header().Get("Content-Security-Policy"); got == "" {
		t.Error("expected a Content-Security-Policy header")
	}
	if got := rec.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS = %q, want unset when enableHSTS=false", got)
	}
}

func TestSecurityHeaders_HSTSOnlyWhenEnabled(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	middleware.SecurityHeaders(true)(okHandler()).ServeHTTP(rec, req)

	if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
		t.Error("expected HSTS header when enableHSTS=true")
	}
}

func TestEnforceSameOrigin(t *testing.T) {
	cases := []struct {
		name      string
		method    string
		host      string
		secFetch  string // Sec-Fetch-Site header ("" = unset)
		origin    string // Origin header ("" = unset)
		allow     []string
		wantBlock bool
	}{
		{"safe GET cross-site passes", http.MethodGet, "reg.test", "cross-site", "http://evil.test", nil, false},
		{"POST same-origin via fetch-metadata", http.MethodPost, "reg.test", "same-origin", "", nil, false},
		{"POST same-site via fetch-metadata", http.MethodPost, "reg.test", "same-site", "", nil, false},
		{"POST user-initiated (none) passes", http.MethodPost, "reg.test", "none", "", nil, false},
		{"POST cross-site via fetch-metadata blocked", http.MethodPost, "reg.test", "cross-site", "http://evil.test", nil, true},
		{"DELETE cross-site via fetch-metadata blocked", http.MethodDelete, "reg.test", "cross-site", "", nil, true},
		{"POST no metadata, no origin (non-browser) passes", http.MethodPost, "reg.test", "", "", nil, false},
		{"POST no metadata, same-origin origin passes", http.MethodPost, "reg.test", "", "https://reg.test", nil, false},
		{"POST no metadata, cross origin blocked", http.MethodPost, "reg.test", "", "https://evil.test", nil, true},
		{"POST no metadata, allowlisted cross origin passes", http.MethodPost, "reg.test", "", "https://spa.test", []string{"https://spa.test"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusOK)
			})
			req := httptest.NewRequest(tc.method, "/api/v1/anything", nil)
			req.Host = tc.host
			if tc.secFetch != "" {
				req.Header.Set("Sec-Fetch-Site", tc.secFetch)
			}
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			rec := httptest.NewRecorder()
			middleware.EnforceSameOrigin(tc.allow)(next).ServeHTTP(rec, req)

			if tc.wantBlock {
				if called {
					t.Fatal("expected request to be blocked, but next handler ran")
				}
				if rec.Code != http.StatusForbidden {
					t.Errorf("status = %d, want 403", rec.Code)
				}
			} else if !called {
				t.Fatalf("expected request to pass, but it was blocked with %d", rec.Code)
			}
		})
	}
}
