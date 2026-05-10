package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/http/handlers"
)

// /config.json is the SPA's runtime bootstrap — issuer, client ID, and the
// token-storage strategy advertised to oidc-client-ts. The audit flagged
// this handler as untested. These tests pin the wire shape and the
// auth-storage coercion (the only logic the handler has).

type configResponse struct {
	OIDCIssuer   string `json:"oidc_issuer"`
	OIDCClientID string `json:"oidc_client_id"`
	AuthStorage  string `json:"auth_storage"`
}

func get(t *testing.T, h http.HandlerFunc) configResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/config.json", nil)
	rec := httptest.NewRecorder()
	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct == "" {
		t.Error("Content-Type header is empty")
	}

	var got configResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return got
}

func TestConfigJSON_RoundTripsConfiguredValues(t *testing.T) {
	h := handlers.ConfigJSON(
		"https://idp.example.com/realms/registry",
		"ai-registry-web",
		"session",
	)
	got := get(t, h)
	if got.OIDCIssuer != "https://idp.example.com/realms/registry" {
		t.Errorf("OIDCIssuer = %q", got.OIDCIssuer)
	}
	if got.OIDCClientID != "ai-registry-web" {
		t.Errorf("OIDCClientID = %q", got.OIDCClientID)
	}
	if got.AuthStorage != "session" {
		t.Errorf("AuthStorage = %q, want session", got.AuthStorage)
	}
}

func TestConfigJSON_AcceptsLocalStorage(t *testing.T) {
	// "local" is the E2E escape hatch — the only other accepted value.
	h := handlers.ConfigJSON("issuer", "client", "local")
	got := get(t, h)
	if got.AuthStorage != "local" {
		t.Errorf("AuthStorage = %q, want local", got.AuthStorage)
	}
}

func TestConfigJSON_CoercesUnknownStorageToSession(t *testing.T) {
	// An operator who sets AUTH_STORAGE to anything other than the two
	// recognised values must get the safer default. Letting unknown values
	// flow through to oidc-client-ts would either crash the SPA at runtime
	// or, worse, silently accept a typo'd "lokal" value.
	for _, in := range []string{"", "lokal", "permanent", "  session  "} {
		t.Run(in, func(t *testing.T) {
			h := handlers.ConfigJSON("issuer", "client", in)
			if got := get(t, h).AuthStorage; got != "session" {
				t.Errorf("AuthStorage = %q for input %q, want session", got, in)
			}
		})
	}
}

func TestConfigJSON_EmptyIssuerStillResponds(t *testing.T) {
	// The dev compose stack sometimes runs with OIDC_ISSUER unset for early
	// boot — the handler must still produce a valid JSON body so the SPA
	// fail-closed branch can surface a clear error to the user instead of
	// crashing during bootstrap.
	h := handlers.ConfigJSON("", "", "session")
	got := get(t, h)
	if got.OIDCIssuer != "" || got.OIDCClientID != "" {
		t.Errorf("expected empty strings, got %+v", got)
	}
	if got.AuthStorage != "session" {
		t.Errorf("AuthStorage = %q, want session", got.AuthStorage)
	}
}
