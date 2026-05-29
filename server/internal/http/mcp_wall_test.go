package http_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/auth"
	stdhttp "github.com/haibread/ai-registry/internal/http"
)

// TestMCPWall_RejectsLocalToken is the end-to-end assertion of ADR 0006's
// non-negotiable MCP wall: a valid registry-issued local token must be refused
// on the OAuth-only MCP surface (/v0). It builds the real router with a local
// issuer, mints a genuine local token, and fires it at a /v0 write — the wall
// must 403 it before the route's own admin guard even runs.
func TestMCPWall_RejectsLocalToken(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: mustPKCS8(t, key),
	})
	li, err := auth.NewLocalIssuer(string(pemBytes), time.Hour)
	if err != nil {
		t.Fatalf("NewLocalIssuer: %v", err)
	}

	mux := stdhttp.NewRouterForTest(stdhttp.RouterDeps{
		Logger:      discardLogger(),
		AuthConf:    auth.Config{OIDCIssuer: "https://example.invalid"},
		LocalIssuer: li,
	})

	token, err := li.Mint("user-1", "admin@example.com")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v0/publish", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("local token on /v0/publish: status = %d, want 403 (MCP wall)", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Local registry tokens") {
		t.Errorf("403 body should explain the MCP wall, got: %s", rec.Body.String())
	}
}

func mustPKCS8(t *testing.T, key *rsa.PrivateKey) []byte {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	return der
}
