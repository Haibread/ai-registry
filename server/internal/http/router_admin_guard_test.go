package http_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	stdhttp "github.com/haibread/ai-registry/internal/http"
)

// hasRequireAdmin reports whether a chi middleware chain contains
// auth.RequireAdmin by comparing function pointers. Identity-compare is the
// only reliable check because middleware.Handler wrappers are plain functions
// with no name at runtime; chi stores them as values in ChainHandler.
func hasRequireAdmin(middlewares []func(http.Handler) http.Handler) bool {
	want := reflect.ValueOf(auth.RequireAdmin).Pointer()
	for _, mw := range middlewares {
		if reflect.ValueOf(mw).Pointer() == want {
			return true
		}
	}
	return false
}

// TestAllWriteRoutesRequireAdmin enforces CLAUDE.md's non-negotiable rule
//
//	"All writes go through admins. Creation, update, publishing, and deletion
//	 of any registry entry is restricted to admin principals (via UI or API).
//	 Non-admins get 403 on any write endpoint."
//
// into a mechanical contract. It walks every registered chi route, filters
// down to mutating HTTP verbs (POST / PUT / PATCH / DELETE), subtracts the
// explicit allow-list of writes that are intentionally public, and fires an
// unauthenticated request at each remaining route. Every one of them must
// return 401 (no claims in context) — if any returns 200/2xx/3xx/404/500
// it means somebody added a write route without `.With(auth.RequireAdmin)`
// and the audit-guard bypass is invisible at unit-test granularity.
//
// The middleware itself already has direct unit tests in
// internal/auth/middleware_test.go (TestRequireAdmin_NoToken,
// TestRequireAdmin_NonAdminClaims, TestRequireAdmin_AdminClaims). The gap
// this test closes is the *wiring*: knowing the middleware works is useless
// if a new route skips it. This test fails when a route is wired wrong, not
// when the middleware is wrong.
//
// Path-parameter placeholders ({namespace}, {slug}, {version}, {id}) are
// substituted with dummy values because the handler is never reached —
// RequireAdmin short-circuits with 401 before the route target runs.
func TestAllWriteRoutesRequireAdmin(t *testing.T) {
	// Writes that are intentionally public. Keep this list small and add a
	// comment per entry explaining why the route is exempt from admin gating.
	//
	// The matching is exact on "METHOD PATH" using the chi route pattern
	// (with param placeholders intact), not the substituted test URL.
	publicWrites := map[string]string{
		// Telemetry endpoints — anonymous users click "view" and "copy"
		// affordances in the public UI; these increment counters and must
		// not require authentication.
		"POST /api/v1/mcp/servers/{namespace}/{slug}/view": "public view counter",
		"POST /api/v1/mcp/servers/{namespace}/{slug}/copy": "public copy counter",
		"POST /api/v1/agents/{namespace}/{slug}/view":      "public view counter",
		"POST /api/v1/agents/{namespace}/{slug}/copy":      "public copy counter",

		// Community-submitted issue reports — unauthenticated users can file
		// reports. List and Patch on /reports remain admin-only.
		"POST /api/v1/reports": "public report submission",
	}

	mux := stdhttp.NewRouterForTest(stdhttp.RouterDeps{
		Logger:   discardLogger(),
		AuthConf: auth.Config{OIDCIssuer: "https://example.invalid"},
	})

	// Collect every write route the router serves.
	type route struct {
		method, pattern string
	}
	var writeRoutes []route
	walker := func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		switch method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		default:
			return nil
		}
		// chi reports grouped routes with a trailing slash for the group's
		// index ("/api/v1/publishers/"); normalise so the allow-list keys
		// match what a human would write in router.go.
		pattern = strings.TrimSuffix(pattern, "/*")
		if pattern != "/" {
			pattern = strings.TrimSuffix(pattern, "/")
		}
		writeRoutes = append(writeRoutes, route{method: method, pattern: pattern})
		return nil
	}
	if err := chi.Walk(mux, walker); err != nil {
		t.Fatalf("walk router: %v", err)
	}
	if len(writeRoutes) == 0 {
		t.Fatal("no write routes discovered — router wiring changed; update this test")
	}

	// Sanity check: every entry in the allow-list must resolve to an actual
	// route. Otherwise typos in the allow-list silently weaken the contract
	// (an entry that matches nothing doesn't exempt anything).
	haveRoute := map[string]bool{}
	for _, r := range writeRoutes {
		haveRoute[r.method+" "+r.pattern] = true
	}
	for key := range publicWrites {
		if !haveRoute[key] {
			t.Errorf("publicWrites allow-list references %q but router has no such route; "+
				"remove the stale entry or fix the pattern", key)
		}
	}

	// Substitution table: path params the handler would otherwise parse from
	// the URL. Values are arbitrary — the request never reaches a handler.
	subs := map[string]string{
		"{namespace}": "dummy-ns",
		"{slug}":      "dummy-slug",
		"{version}":   "1.0.0",
		"{id}":        "01HAGT0000000000000000000",
	}

	// Deterministic order for predictable failure output.
	sort.Slice(writeRoutes, func(i, j int) bool {
		if writeRoutes[i].pattern != writeRoutes[j].pattern {
			return writeRoutes[i].pattern < writeRoutes[j].pattern
		}
		return writeRoutes[i].method < writeRoutes[j].method
	})

	for _, r := range writeRoutes {
		key := r.method + " " + r.pattern
		if _, exempt := publicWrites[key]; exempt {
			continue
		}

		url := r.pattern
		for placeholder, value := range subs {
			url = strings.ReplaceAll(url, placeholder, value)
		}

		// Bodyless request with no Authorization header. RequireJSONBody
		// allows bodyless writes through (Content-Length: 0 path), so the
		// request reaches the auth stack. Authenticate passes through (no
		// token), then RequireAdmin emits 401 because no claims were
		// attached to context.
		req := httptest.NewRequest(r.method, url, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401 (missing .With(auth.RequireAdmin)?)\nbody: %s",
				key, rec.Code, rec.Body.String())
		}
	}
}

// TestPublicWriteRoutesBypassAdmin is the negative counterpart of the above:
// the routes in the allow-list MUST NOT carry auth.RequireAdmin in their
// middleware chain. If someone accidentally wires `.With(auth.RequireAdmin)`
// onto a view/copy endpoint, anonymous telemetry breaks silently — clients
// in the public UI start getting 401s on click. This test catches that
// direction by identity-comparing the middleware chain chi reports for each
// route against auth.RequireAdmin, so it never fires a real request and
// never touches a nil DB.
func TestPublicWriteRoutesBypassAdmin(t *testing.T) {
	// Full catalogue — must mirror the publicWrites allow-list in the
	// sibling test. A route missing from the router surfaces via the
	// sanity-check below.
	publicWriteKeys := []string{
		"POST /api/v1/mcp/servers/{namespace}/{slug}/view",
		"POST /api/v1/mcp/servers/{namespace}/{slug}/copy",
		"POST /api/v1/agents/{namespace}/{slug}/view",
		"POST /api/v1/agents/{namespace}/{slug}/copy",
		"POST /api/v1/reports",
	}

	mux := stdhttp.NewRouterForTest(stdhttp.RouterDeps{
		Logger:   discardLogger(),
		AuthConf: auth.Config{OIDCIssuer: "https://example.invalid"},
	})

	gated := map[string]bool{}
	seen := map[string]bool{}
	walker := func(method, pattern string, _ http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		pattern = strings.TrimSuffix(pattern, "/*")
		if pattern != "/" {
			pattern = strings.TrimSuffix(pattern, "/")
		}
		key := method + " " + pattern
		seen[key] = true
		gated[key] = hasRequireAdmin(middlewares)
		return nil
	}
	if err := chi.Walk(mux, walker); err != nil {
		t.Fatalf("walk router: %v", err)
	}

	for _, key := range publicWriteKeys {
		if !seen[key] {
			t.Errorf("public-write route %q is not registered on the router", key)
			continue
		}
		if gated[key] {
			t.Errorf("public-write route %q has auth.RequireAdmin in its middleware chain — "+
				"anonymous telemetry/reports would break", key)
		}
	}
}
