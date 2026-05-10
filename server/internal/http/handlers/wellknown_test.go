package handlers_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
)

const (
	testPublicBaseURL = "https://registry.example.com"
	testIssuer        = "https://idp.example.com/realms/registry"
)

func newWellKnownRouter(publicBaseURL string) *chi.Mux {
	logger := slog.Default()
	cardH := handlers.NewAgentCardHandlers(nil, logger, publicBaseURL)
	r := chi.NewRouter()
	r.Get("/.well-known/oauth-protected-resource",
		handlers.OAuthProtectedResource(publicBaseURL, testIssuer, logger))
	r.Get("/.well-known/agent-card.json", cardH.GlobalAgentCard)
	return r
}

func TestOAuthProtectedResource_ResponseShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	newWellKnownRouter(testPublicBaseURL).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"resource", "authorization_servers", "bearer_methods_supported"} {
		if _, ok := body[key]; !ok {
			t.Errorf("response missing key %q", key)
		}
	}
}

func TestOAuthProtectedResource_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	newWellKnownRouter(testPublicBaseURL).ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct == "" {
		t.Error("expected Content-Type header to be set")
	}
}

// Mirrors the GlobalAgentCard fail-loud pattern: with PUBLIC_BASE_URL empty
// the protected-resource endpoint must NOT silently advertise localhost — it
// returns 500 so misconfigured deployments surface the error rather than
// publishing a wrong resource identifier.
func TestOAuthProtectedResource_MissingBaseURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	newWellKnownRouter("").ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 when PUBLIC_BASE_URL unset", rec.Code)
	}
}

// The handler returns the configured public base URL verbatim, with no
// fallback. Pin the value so a stray `localhost` fallback would be caught.
func TestOAuthProtectedResource_AdvertisesConfiguredBaseURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	newWellKnownRouter(testPublicBaseURL).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Resource              string   `json:"resource"`
		AuthorizationServers  []string `json:"authorization_servers"`
		ResourceDocumentation string   `json:"resource_documentation"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Resource != testPublicBaseURL {
		t.Errorf("resource = %q, want %q", body.Resource, testPublicBaseURL)
	}
	if len(body.AuthorizationServers) != 1 || body.AuthorizationServers[0] != testIssuer {
		t.Errorf("authorization_servers = %v, want [%q]", body.AuthorizationServers, testIssuer)
	}
	if !strings.HasPrefix(body.ResourceDocumentation, testPublicBaseURL) {
		t.Errorf("resource_documentation = %q, want prefix %q", body.ResourceDocumentation, testPublicBaseURL)
	}
}

func TestGlobalAgentCard_ResponseShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newWellKnownRouter(testPublicBaseURL).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// A2A agent card must have at minimum a name field
	if _, ok := body["name"]; !ok {
		t.Error("agent card missing 'name' field")
	}
}

func TestGlobalAgentCard_MissingBaseURL(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newWellKnownRouter("").ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 when PUBLIC_BASE_URL unset", rec.Code)
	}
}
