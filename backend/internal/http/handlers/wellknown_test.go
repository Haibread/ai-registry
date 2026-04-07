package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
)

func newWellKnownRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/.well-known/oauth-protected-resource", handlers.OAuthProtectedResource)
	r.Get("/.well-known/agent-card.json", handlers.GlobalAgentCard)
	return r
}

func TestOAuthProtectedResource_ResponseShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-protected-resource", nil)
	rec := httptest.NewRecorder()
	newWellKnownRouter().ServeHTTP(rec, req)

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
	newWellKnownRouter().ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct == "" {
		t.Error("expected Content-Type header to be set")
	}
}

func TestGlobalAgentCard_ResponseShape(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newWellKnownRouter().ServeHTTP(rec, req)

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
