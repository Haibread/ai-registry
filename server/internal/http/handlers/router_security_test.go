package handlers_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/auth"
	stdhttp "github.com/haibread/ai-registry/internal/http"
	"github.com/haibread/ai-registry/internal/store"
)

// buildSecureRouter builds the real production router backed by testDB plus a
// token authority, and returns a helper that mints a bearer access token for a
// Server-Admin or a plain (no-grants) user. Auth is the bearer-token model —
// there is no cookie.
func buildSecureRouter(t *testing.T) (http.Handler, func(isAdmin bool) string) {
	t.Helper()
	ta := testAuthority(t)
	router := stdhttp.NewRouterForTest(stdhttp.RouterDeps{
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:                testDB,
		Tokens:            ta,
		Refresh:           auth.NewRefreshManager(testDB, time.Hour),
		LocalLoginEnabled: true,
	})
	ctx := context.Background()
	tokenFor := func(isAdmin bool) string {
		prefix := "viewer-"
		if isAdmin {
			prefix = "admin-"
		}
		u, err := testDB.CreateUser(ctx, store.CreateUserParams{
			Email:         prefix + store.NewULID() + "@test.example",
			IsServerAdmin: isAdmin,
		})
		if err != nil {
			t.Fatalf("create user: %v", err)
		}
		token, _, err := ta.Mint(auth.MintParams{
			UserID: u.ID, Email: u.Email, SrvAdmin: isAdmin, AuthMethod: "local",
		})
		if err != nil {
			t.Fatalf("mint token: %v", err)
		}
		return token
	}
	return router, tokenFor
}

func fireSecure(router http.Handler, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// TestRouter_AdminRoutes_AuthEnforcement verifies the 401 / 403 / authorized
// matrix on the middleware-gated admin + write routes of the production router:
// no session → 401, a non-admin session → 403, a Server-Admin session →
// neither. These routes resolve their publisher from the path (or are
// Server-Admin-only), so the matrix is independent of the request body. The
// body-authorized create routes (POST /mcp/servers, /agents) have their own
// per-handler role tests.
func TestRouter_AdminRoutes_AuthEnforcement(t *testing.T) {
	resetTables(t)
	router, tokenFor := buildSecureRouter(t)

	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/publishers"},
		{http.MethodPost, "/api/v1/mcp/servers/ns/slug/deprecate"},
		{http.MethodPost, "/api/v1/mcp/servers/ns/slug/visibility"},
		{http.MethodPost, "/api/v1/mcp/servers/ns/slug/versions"},
		{http.MethodPost, "/api/v1/mcp/servers/ns/slug/versions/1.0.0/publish"},
		{http.MethodPost, "/api/v1/agents/ns/slug/deprecate"},
		{http.MethodPost, "/api/v1/agents/ns/slug/visibility"},
		{http.MethodPost, "/api/v1/agents/ns/slug/versions"},
		{http.MethodPost, "/api/v1/agents/ns/slug/versions/1.0.0/publish"},
		{http.MethodGet, "/api/v1/stats"},
		{http.MethodGet, "/api/v1/audit"},
	}

	for _, route := range routes {
		route := route
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// Case 1: No token → 401.
			if rec := fireSecure(router, route.method, route.path, ""); rec.Code != http.StatusUnauthorized {
				t.Errorf("no token: got %d, want 401\nbody: %s", rec.Code, rec.Body.String())
			}
			// Case 2: Non-admin token → 403.
			if rec := fireSecure(router, route.method, route.path, tokenFor(false)); rec.Code != http.StatusForbidden {
				t.Errorf("non-admin token: got %d, want 403\nbody: %s", rec.Code, rec.Body.String())
			}
			// Case 3: Server-Admin token → neither 401 nor 403.
			rec := fireSecure(router, route.method, route.path, tokenFor(true))
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
				t.Errorf("admin token: got %d, want neither 401 nor 403\nbody: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestRouter_MetricsUnauthenticated verifies that /metrics is scrapeable
// without auth. Prometheus scrapes it in-cluster via the ClusterIP Service and
// cannot present a bearer token, so gating it behind RequireAdmin (as it once
// was) silently broke metric collection on k8s. The endpoint is not routed by
// the shipped Ingress, so it is not public.
func TestRouter_MetricsUnauthenticated(t *testing.T) {
	resetTables(t)
	router, _ := buildSecureRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics without auth: got %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want a Prometheus text exposition type", ct)
	}
}
