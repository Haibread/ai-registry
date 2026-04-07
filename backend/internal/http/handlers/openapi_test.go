package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
)

func newOpenAPIRouter() *chi.Mux {
	r := chi.NewRouter()
	r.Get("/openapi.yaml", handlers.OpenAPISpec)
	return r
}

func TestOpenAPISpec_Returns200(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	newOpenAPIRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestOpenAPISpec_ContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	newOpenAPIRouter().ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "yaml") {
		t.Errorf("Content-Type = %q, want to contain 'yaml'", ct)
	}
}

func TestOpenAPISpec_NonEmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
	rec := httptest.NewRecorder()
	newOpenAPIRouter().ServeHTTP(rec, req)

	if rec.Body.Len() == 0 {
		t.Error("expected non-empty response body")
	}
}
