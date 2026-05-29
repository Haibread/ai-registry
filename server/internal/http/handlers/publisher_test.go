package handlers_test

// Regression tests for the publisher handler.
// Root cause of the bug: /api/v1/publishers routes were never implemented,
// so the admin publishers page always returned an empty list.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

func newPublisherRouter() *chi.Mux {
	h := handlers.NewPublisherHandlers(testDB, testDB)
	r := chi.NewRouter()
	r.Get("/api/v1/publishers", h.ListPublishers)
	r.Post("/api/v1/publishers", h.CreatePublisher)
	r.Get("/api/v1/publishers/{slug}", h.GetPublisher)
	r.Patch("/api/v1/publishers/{slug}", h.PatchPublisher)
	r.Delete("/api/v1/publishers/{slug}", h.DeletePublisher)
	return r
}

func TestPublisherHandler_ListEmpty(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/publishers", nil)
	rec := httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Must return an empty slice, not null — the web UI does `data?.items ?? []`.
	if body.Items == nil {
		t.Error("items should be an empty array, not null")
	}
	if len(body.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(body.Items))
	}
}

func TestPublisherHandler_CreateAndList(t *testing.T) {
	resetTables(t)

	// Create.
	payload := `{"slug":"acme","name":"Acme Corp","contact":"ops@acme.example"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID      string `json:"id"`
		Slug    string `json:"slug"`
		Name    string `json:"name"`
		Contact string `json:"contact"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty id")
	}
	if created.Slug != "acme" {
		t.Errorf("slug = %q, want %q", created.Slug, "acme")
	}

	// List should now show 1 publisher.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/publishers", nil)
	rec = httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)

	var list struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Items) != 1 {
		t.Errorf("list len = %d, want 1", len(list.Items))
	}
	if list.Items[0].Slug != "acme" {
		t.Errorf("slug = %q, want acme", list.Items[0].Slug)
	}
}

func TestPublisherHandler_Create_ValidationErrors(t *testing.T) {
	resetTables(t)
	r := newPublisherRouter()

	tests := []struct {
		name    string
		payload string
		wantStatus int
	}{
		{"missing slug", `{"name":"No Slug"}`, http.StatusUnprocessableEntity},
		{"missing name", `{"slug":"no-name"}`, http.StatusUnprocessableEntity},
		{"invalid json", `{bad json}`, http.StatusUnprocessableEntity},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers",
				bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestPublisherHandler_Create_ConflictOnDuplicateSlug(t *testing.T) {
	resetTables(t)
	r := newPublisherRouter()

	payload := `{"slug":"dup","name":"Dup"}`
	post := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers",
			bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post(); code != http.StatusCreated {
		t.Fatalf("first create: status = %d", code)
	}
	if code := post(); code != http.StatusConflict {
		t.Errorf("duplicate create: status = %d, want 409", code)
	}
}

func TestPublisherHandler_GetBySlug(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "getme", "Get Me")

	r := newPublisherRouter()

	// Found.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/publishers/getme", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var pub struct {
		Slug string `json:"slug"`
	}
	json.NewDecoder(rec.Body).Decode(&pub) //nolint:errcheck
	if pub.Slug != "getme" {
		t.Errorf("slug = %q, want getme", pub.Slug)
	}

	// Not found.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/publishers/nope", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestPublisherHandler_List_Pagination(t *testing.T) {
	resetTables(t)
	r := newPublisherRouter()

	// Seed 5 publishers via the store directly.
	for _, s := range []string{"z1", "z2", "z3", "z4", "z5"} {
		_, err := testDB.CreatePublisher(t.Context(), store.CreatePublisherParams{Slug: s, Name: s})
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// First page of 3.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/publishers?limit=3", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var page1 struct {
		Items      []struct{ ID string `json:"id"` } `json:"items"`
		NextCursor string                             `json:"next_cursor"`
	}
	json.NewDecoder(rec.Body).Decode(&page1) //nolint:errcheck
	if len(page1.Items) != 3 {
		t.Errorf("page1 len = %d, want 3", len(page1.Items))
	}
	if page1.NextCursor == "" {
		t.Error("expected non-empty next_cursor for first page")
	}

	// Second page.
	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/publishers?limit=3&cursor="+page1.NextCursor, nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	var page2 struct {
		Items []struct{ ID string `json:"id"` } `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&page2) //nolint:errcheck
	if len(page2.Items) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2.Items))
	}

	// No overlap.
	seen := map[string]bool{}
	for _, p := range page1.Items {
		seen[p.ID] = true
	}
	for _, p := range page2.Items {
		if seen[p.ID] {
			t.Errorf("item %s appeared on both pages", p.ID)
		}
	}
}

func TestPublisherHandler_Patch_Success(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "patch-pub", "Original Name")

	payload := `{"name":"Updated Name","contact":"new@example.com"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/publishers/patch-pub",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var pub struct {
		Name    string `json:"name"`
		Contact string `json:"contact"`
		Slug    string `json:"slug"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&pub); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pub.Name != "Updated Name" {
		t.Errorf("name = %q, want %q", pub.Name, "Updated Name")
	}
	if pub.Contact != "new@example.com" {
		t.Errorf("contact = %q, want %q", pub.Contact, "new@example.com")
	}
	if pub.Slug != "patch-pub" {
		t.Errorf("slug changed: got %q, want %q", pub.Slug, "patch-pub")
	}
}

func TestPublisherHandler_Patch_NotFound(t *testing.T) {
	resetTables(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/publishers/missing",
		bytes.NewBufferString(`{"name":"X"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestPublisherHandler_Patch_NameRequired(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "val-pub", "Some Pub")

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/publishers/val-pub",
		bytes.NewBufferString(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestPublisherHandler_Delete_Success(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "del-pub", "Delete Me")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/publishers/del-pub", nil)
	rec := httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	// Should now 404.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/publishers/del-pub", nil)
	rec = httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("after delete: status = %d, want 404", rec.Code)
	}
}

func TestPublisherHandler_Delete_NotFound(t *testing.T) {
	resetTables(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/publishers/missing", nil)
	rec := httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestPublisherHandler_Delete_ConflictWithActiveEntries(t *testing.T) {
	resetTables(t)
	// Create publisher with an MCP server.
	pubID := seedPublisher(t, "busy-pub", "Busy")
	if _, err := testDB.CreateMCPServer(t.Context(), store.CreateMCPServerParams{
		PublisherID: pubID,
		Slug:        "server",
		Name:        "Server",
	}); err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/publishers/busy-pub", nil)
	rec := httptest.NewRecorder()
	newPublisherRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}
