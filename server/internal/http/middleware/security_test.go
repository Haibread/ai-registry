package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/haibread/ai-registry/internal/http/middleware"
)

func TestRequireJSONBody_AllowsBodylessPOST(t *testing.T) {
	// Endpoints like /view and /copy POST with no body to record an event.
	// The middleware must not reject them for lack of a Content-Type header.
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns/slug/view", nil)
	req.ContentLength = 0
	rec := httptest.NewRecorder()

	middleware.RequireJSONBody(next).ServeHTTP(rec, req)

	if !called {
		t.Fatalf("expected next handler to be invoked for bodyless POST, got %d", rec.Code)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestRequireJSONBody_RejectsPOSTWithWrongContentType(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", strings.NewReader(`hello`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	middleware.RequireJSONBody(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", rec.Code)
	}
}

func TestRequireJSONBody_AllowsPOSTWithJSONContentType(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/anything", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	middleware.RequireJSONBody(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be invoked")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestRequireJSONBody_PassesThroughGET(t *testing.T) {
	var called bool
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
	rec := httptest.NewRecorder()

	middleware.RequireJSONBody(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be invoked for GET")
	}
}
