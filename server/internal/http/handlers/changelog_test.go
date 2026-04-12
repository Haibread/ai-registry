package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
)

func newChangelogRouter() *chi.Mux {
	h := handlers.NewChangelogHandlers(testDB)
	r := chi.NewRouter()
	r.Get("/api/v1/changelog", h.GetChangelog)
	return r
}

func TestChangelogHandler_Empty(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/changelog", nil)
	rec := httptest.NewRecorder()
	newChangelogRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []any `json:"items"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Items == nil {
		t.Error("items should be empty array, not nil")
	}
	if len(body.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(body.Items))
	}
}

func TestChangelogHandler_LimitQueryParam(t *testing.T) {
	resetTables(t)

	// Empty DB; we just verify no error and shape is correct.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/changelog?limit=10", nil)
	rec := httptest.NewRecorder()
	newChangelogRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestChangelogHandler_InvalidLimitIgnored(t *testing.T) {
	resetTables(t)

	// Non-numeric limit is ignored (handler falls back to default)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/changelog?limit=abc", nil)
	rec := httptest.NewRecorder()
	newChangelogRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}
