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

func TestSwaggerUI_ContentSecurityPolicy(t *testing.T) {
	r := chi.NewRouter()
	r.Get("/docs", handlers.SwaggerUI)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/docs", nil))

	csp := rec.Header().Values("Content-Security-Policy")
	if len(csp) != 1 {
		t.Fatalf("Content-Security-Policy headers = %q, want exactly one", csp)
	}
	directives := map[string]string{}
	for _, d := range strings.Split(csp[0], ";") {
		name, sources, _ := strings.Cut(strings.TrimSpace(d), " ")
		directives[name] = sources
	}

	const bundle = "https://cdn.jsdelivr.net/npm/@scalar/api-reference"
	if got := directives["script-src"]; got != bundle {
		t.Errorf("script-src = %q, want only %q", got, bundle)
	}
	if !strings.Contains(rec.Body.String(), `<script src="`+bundle+`">`) {
		t.Errorf("page does not load the bundle script-src allows")
	}
	if got := directives["default-src"]; got != "'none'" {
		t.Errorf("default-src = %q, want 'none'", got)
	}
}
