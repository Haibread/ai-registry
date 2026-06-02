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
// session manager, and returns a helper that opens a session for a Server-Admin
// or a plain (no-grants) user and returns the session cookie. Auth is the
// session-cookie model — there are no bearer tokens.
func buildSecureRouter(t *testing.T) (http.Handler, func(isAdmin bool) *http.Cookie) {
	t.Helper()
	sm := auth.NewSessionManager(testDB, auth.SessionConfig{
		CookieName: "ai_registry_session",
		TTL:        time.Hour,
	})
	router := stdhttp.NewRouterForTest(stdhttp.RouterDeps{
		Logger:            slog.New(slog.NewTextHandler(io.Discard, nil)),
		DB:                testDB,
		Sessions:          sm,
		LocalLoginEnabled: true,
	})
	ctx := context.Background()
	cookieFor := func(isAdmin bool) *http.Cookie {
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
		token, err := sm.Issue(ctx, auth.IssueParams{UserID: u.ID, AuthMethod: "local"})
		if err != nil {
			t.Fatalf("issue session: %v", err)
		}
		return sm.Cookie(token)
	}
	return router, cookieFor
}

func fireSecure(router http.Handler, method, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		req.AddCookie(cookie)
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
	router, cookieFor := buildSecureRouter(t)

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
			// Case 1: No session → 401.
			if rec := fireSecure(router, route.method, route.path, nil); rec.Code != http.StatusUnauthorized {
				t.Errorf("no session: got %d, want 401\nbody: %s", rec.Code, rec.Body.String())
			}
			// Case 2: Non-admin session → 403.
			if rec := fireSecure(router, route.method, route.path, cookieFor(false)); rec.Code != http.StatusForbidden {
				t.Errorf("non-admin session: got %d, want 403\nbody: %s", rec.Code, rec.Body.String())
			}
			// Case 3: Server-Admin session → neither 401 nor 403.
			rec := fireSecure(router, route.method, route.path, cookieFor(true))
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
				t.Errorf("admin session: got %d, want neither 401 nor 403\nbody: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

// TestRouter_MetricsUnauthenticated verifies that /metrics is scrapeable
// without a session. Prometheus scrapes it in-cluster via the ClusterIP
// Service and cannot present the registry session cookie, so gating it behind
// RequireAdmin (as it once was) silently broke metric collection on k8s. The
// endpoint is not routed by the shipped Ingress, so it is not public.
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
