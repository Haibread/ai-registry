package handlers_test

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/http/handlers"
)

// ---------------------------------------------------------------------------
// JWT helpers (duplicated from auth package tests — different package)
// ---------------------------------------------------------------------------

func generateTestKeyRouter(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	return priv
}

func bigIntToBase64URLRouter(n *big.Int) string {
	return base64.RawURLEncoding.EncodeToString(n.Bytes())
}

func intToBase64URLRouter(e int) string {
	return base64.RawURLEncoding.EncodeToString(big.NewInt(int64(e)).Bytes())
}

type jwksEntryRouter struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksDocRouter struct {
	Keys []jwksEntryRouter `json:"keys"`
}

func newFakeJWKSServerRouter(t *testing.T, priv *rsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()
	doc := jwksDocRouter{
		Keys: []jwksEntryRouter{{
			Kty: "RSA",
			Kid: kid,
			Use: "sig",
			N:   bigIntToBase64URLRouter(priv.PublicKey.N),
			E:   intToBase64URLRouter(priv.PublicKey.E),
		}},
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(doc)
	}))
}

func signJWTForRouter(t *testing.T, priv *rsa.PrivateKey, kid, issuer string, roles []string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": issuer,
		"sub": "user-123",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
		"realm_access": map[string]interface{}{
			"roles": roles,
		},
	})
	tok.Header["kid"] = kid

	signed, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("signing JWT: %v", err)
	}
	return signed
}

// ---------------------------------------------------------------------------
// buildSecureRouter wires up the full production-like router with test JWKS.
// ---------------------------------------------------------------------------

func buildSecureRouter(t *testing.T) (http.Handler, func([]string) string) {
	t.Helper()
	priv := generateTestKeyRouter(t)
	const kid = "k1"
	const issuer = "http://test-keycloak/realms/test"

	jwksSrv := newFakeJWKSServerRouter(t, priv, kid)
	t.Cleanup(jwksSrv.Close)

	cache := auth.NewJWKSCache(jwksSrv.URL, time.Minute)
	validator := auth.NewValidator(cache, issuer)

	mcpH := handlers.NewMCPHandlers(testDB, testDB)
	agentH := handlers.NewAgentHandlers(testDB, testDB)
	pubH := handlers.NewPublisherHandlers(testDB, testDB)
	auditH := handlers.NewAuditHandlers(testDB)
	statsH := handlers.NewStatsHandlers(testDB)
	v0H := handlers.NewV0MCPHandlers(testDB, testDB)

	r := chi.NewRouter()
	r.Use(validator.Authenticate)

	r.With(auth.RequireAdmin).Get("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r.Route("/v0", func(r chi.Router) {
		r.With(auth.RequireAdmin).Post("/publish", v0H.Publish)
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/publishers", func(r chi.Router) {
			r.With(auth.RequireAdmin).Post("/", pubH.CreatePublisher)
		})
		r.Route("/mcp/servers", func(r chi.Router) {
			r.With(auth.RequireAdmin).Post("/", mcpH.CreateServer)
			r.Route("/{namespace}/{slug}", func(r chi.Router) {
				r.With(auth.RequireAdmin).Post("/deprecate", mcpH.DeprecateServer)
				r.With(auth.RequireAdmin).Post("/visibility", mcpH.SetVisibility)
				r.Route("/versions", func(r chi.Router) {
					r.With(auth.RequireAdmin).Post("/", mcpH.CreateVersion)
					r.With(auth.RequireAdmin).Post("/{version}/publish", mcpH.PublishVersion)
				})
			})
		})
		r.Route("/agents", func(r chi.Router) {
			r.With(auth.RequireAdmin).Post("/", agentH.CreateAgent)
			r.Route("/{namespace}/{slug}", func(r chi.Router) {
				r.With(auth.RequireAdmin).Post("/deprecate", agentH.DeprecateAgent)
				r.With(auth.RequireAdmin).Post("/visibility", agentH.SetVisibility)
				r.Route("/versions", func(r chi.Router) {
					r.With(auth.RequireAdmin).Post("/", agentH.CreateVersion)
					r.With(auth.RequireAdmin).Post("/{version}/publish", agentH.PublishVersion)
				})
			})
		})
		r.With(auth.RequireAdmin).Get("/stats", statsH.GetStats)
		r.With(auth.RequireAdmin).Get("/audit", auditH.ListEvents)
	})

	sign := func(roles []string) string {
		return signJWTForRouter(t, priv, kid, issuer, roles)
	}
	return r, sign
}

// ---------------------------------------------------------------------------
// TestRouter_AdminRoutes_AuthEnforcement verifies 401/403 enforcement on all
// write and admin-only endpoints in the production router.
// ---------------------------------------------------------------------------

func TestRouter_AdminRoutes_AuthEnforcement(t *testing.T) {
	resetTables(t)
	router, sign := buildSecureRouter(t)

	routes := []struct{ method, path string }{
		{http.MethodPost, "/api/v1/publishers"},
		{http.MethodPost, "/api/v1/mcp/servers"},
		{http.MethodPost, "/api/v1/mcp/servers/ns/slug/deprecate"},
		{http.MethodPost, "/api/v1/mcp/servers/ns/slug/visibility"},
		{http.MethodPost, "/api/v1/mcp/servers/ns/slug/versions"},
		{http.MethodPost, "/api/v1/mcp/servers/ns/slug/versions/1.0.0/publish"},
		{http.MethodPost, "/api/v1/agents"},
		{http.MethodPost, "/api/v1/agents/ns/slug/deprecate"},
		{http.MethodPost, "/api/v1/agents/ns/slug/visibility"},
		{http.MethodPost, "/api/v1/agents/ns/slug/versions"},
		{http.MethodPost, "/api/v1/agents/ns/slug/versions/1.0.0/publish"},
		{http.MethodGet, "/api/v1/stats"},
		{http.MethodGet, "/api/v1/audit"},
		{http.MethodGet, "/metrics"},
		{http.MethodPost, "/v0/publish"},
	}

	for _, route := range routes {
		route := route
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			// Case 1: No token → 401
			req := httptest.NewRequest(route.method, route.path, bytes.NewBufferString("{}"))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("no token: got %d, want 401", rec.Code)
			}

			// Case 2: Non-admin token → 403
			req = httptest.NewRequest(route.method, route.path, bytes.NewBufferString("{}"))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+sign([]string{"viewer"}))
			rec = httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != http.StatusForbidden {
				t.Errorf("non-admin token: got %d, want 403", rec.Code)
			}

			// Case 3: Admin token → not 401/403 (may be 404/422 since ns/slug don't exist)
			req = httptest.NewRequest(route.method, route.path, bytes.NewBufferString("{}"))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+sign([]string{"admin"}))
			rec = httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusForbidden {
				t.Errorf("admin token: got %d, want neither 401 nor 403", rec.Code)
			}
		})
	}
}
