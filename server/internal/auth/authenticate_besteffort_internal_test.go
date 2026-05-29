package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/store"
)

func localIssuerForTest(t *testing.T) *LocalIssuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	li, err := NewLocalIssuer(string(pemBytes), time.Hour)
	if err != nil {
		t.Fatalf("NewLocalIssuer: %v", err)
	}
	return li
}

// TestAuthenticate_ResolutionFailureIsBestEffort pins the regression that broke
// e2e: when a valid token cannot be resolved into a users row, Authenticate
// must NOT 401 — it proceeds with claims only so legacy claim-based guards keep
// working (the not-yet-cut-over routes). Only `disabled` is a hard stop.
func TestAuthenticate_ResolutionFailureIsBestEffort(t *testing.T) {
	li := localIssuerForTest(t)
	// Empty store: the token's subject resolves to no users row.
	v := NewValidator(nil, "issuer", "", "").WithLocalIssuer(li).WithPrincipalStore(newFakeStore())

	token, err := li.Mint("ghost-user", "ghost@example.com")
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}

	var called bool
	var hadPrincipal bool
	h := v.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, hadPrincipal = PrincipalFromContext(r.Context())
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204 (best-effort proceed)", rec.Code)
	}
	if !called {
		t.Error("handler should have been reached despite unresolvable principal")
	}
	if hadPrincipal {
		t.Error("no Principal should be in context when resolution failed")
	}
}

// TestAuthenticate_DisabledIsHardStop confirms a disabled account is refused on
// every request even though resolution otherwise succeeds.
func TestAuthenticate_DisabledIsHardStop(t *testing.T) {
	li := localIssuerForTest(t)
	fs := newFakeStore()
	fs.add(&store.User{ID: "u1", Email: "u1@example.com", Disabled: true})
	v := NewValidator(nil, "issuer", "", "").WithLocalIssuer(li).WithPrincipalStore(fs)

	token, _ := li.Mint("u1", "u1@example.com")
	h := v.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403 for a disabled account", rec.Code)
	}
}
