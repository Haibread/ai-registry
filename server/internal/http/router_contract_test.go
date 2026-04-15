package http_test

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"gopkg.in/yaml.v3"

	"github.com/haibread/ai-registry/api"
	"github.com/haibread/ai-registry/internal/auth"
	stdhttp "github.com/haibread/ai-registry/internal/http"
)

// TestOpenAPIContract_MatchesRouter enforces a bijection between the documented
// OpenAPI spec and the chi router at runtime:
//
//   - every (method, path) in openapi.yaml must resolve to a registered route
//   - every registered route must have a matching (method, path) in openapi.yaml
//
// CLAUDE.md says the OpenAPI spec is the source of truth and must stay in
// sync with the implementation at all times. This test is the mechanical
// enforcement of that rule — if a new handler lands without a spec entry
// (or vice versa) CI fails here.
//
// An explicit allow-list carries the small set of endpoints that intentionally
// exist in only one place:
//
//   - routerOnlyAllowList: paths the server exposes but the spec does not
//     document (e.g. /config.json, a browser bootstrap payload that is not
//     part of the public API surface).
//   - specOnlyAllowList: paths the spec documents but no chi route registers.
//     Should normally be empty — populate only for paths served outside the
//     chi mux and leave a comment explaining why.
func TestOpenAPIContract_MatchesRouter(t *testing.T) {
	// Endpoints the router exposes that intentionally are NOT documented in
	// openapi.yaml. Keep this list short and add a comment per entry.
	routerOnlyAllowList := map[string]bool{
		// /config.json is a browser-SPA bootstrap payload (OIDC issuer and
		// client id). It's not part of the versioned public API surface.
		"GET /config.json": true,
	}

	// Endpoints the spec documents that the router does not serve directly.
	// Should almost always be empty.
	specOnlyAllowList := map[string]bool{}

	specRoutes, err := parseOpenAPIRoutes(api.Spec)
	if err != nil {
		t.Fatalf("parsing openapi.yaml: %v", err)
	}

	routerRoutes, err := collectRouterRoutes()
	if err != nil {
		t.Fatalf("collecting router routes: %v", err)
	}

	// spec ⊆ router
	var missingInRouter []string
	for route := range specRoutes {
		if !routerRoutes[route] && !specOnlyAllowList[route] {
			missingInRouter = append(missingInRouter, route)
		}
	}
	sort.Strings(missingInRouter)
	for _, r := range missingInRouter {
		t.Errorf("openapi.yaml documents %q but no chi route matches", r)
	}

	// router ⊆ spec
	var missingInSpec []string
	for route := range routerRoutes {
		if !specRoutes[route] && !routerOnlyAllowList[route] {
			missingInSpec = append(missingInSpec, route)
		}
	}
	sort.Strings(missingInSpec)
	for _, r := range missingInSpec {
		t.Errorf("router serves %q but openapi.yaml does not document it", r)
	}
}

// parseOpenAPIRoutes turns the embedded openapi.yaml bytes into a set of
// "METHOD /path" strings. Only the top-level `paths` mapping is inspected;
// non-method keys inside a path item (parameters, summary, etc.) are ignored.
func parseOpenAPIRoutes(specBytes []byte) (map[string]bool, error) {
	var doc struct {
		Paths map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(specBytes, &doc); err != nil {
		return nil, err
	}

	// OpenAPI 3.1 method keys. Anything else at the path-item level is
	// shared metadata (parameters, summary, description, servers, etc.).
	methodKeys := map[string]string{
		"get":     http.MethodGet,
		"put":     http.MethodPut,
		"post":    http.MethodPost,
		"delete":  http.MethodDelete,
		"options": http.MethodOptions,
		"head":    http.MethodHead,
		"patch":   http.MethodPatch,
		"trace":   http.MethodTrace,
	}

	routes := map[string]bool{}
	for path, ops := range doc.Paths {
		for key, m := range methodKeys {
			if _, ok := ops[key]; ok {
				routes[m+" "+path] = true
			}
		}
	}
	return routes, nil
}

// collectRouterRoutes builds the chi router with minimal (zero-value)
// dependencies and returns every registered route as a "METHOD /path" string.
// Handlers are never invoked, so nil DB / Metrics are safe for the walk.
// Uses NewRouterForTest to get the raw *chi.Mux — chi.Walk cannot descend
// through the otelhttp wrapper applied by NewRouter.
func collectRouterRoutes() (map[string]bool, error) {
	mux := stdhttp.NewRouterForTest(stdhttp.RouterDeps{
		AuthConf: auth.Config{OIDCIssuer: "https://example.invalid"},
	})

	routes := map[string]bool{}
	walker := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// chi reports grouped routes with a trailing "/" for the group's
		// index. Normalise by trimming so the comparison with the spec
		// (which uses "/api/v1/publishers", not "/api/v1/publishers/") is
		// apples to apples.
		route = strings.TrimSuffix(route, "/*")
		if route != "/" {
			route = strings.TrimSuffix(route, "/")
		}
		routes[method+" "+route] = true
		return nil
	}
	if err := chi.Walk(mux, walker); err != nil {
		return nil, err
	}
	return routes, nil
}
