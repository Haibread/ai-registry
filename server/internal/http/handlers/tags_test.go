package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

// newTagRouter mounts the tag handlers without the production auth
// middleware — the 401/403 matrix for the write routes is covered by
// TestRouter_AdminRoutes_AuthEnforcement against the real router.
func newTagRouter() *chi.Mux {
	h := handlers.NewTagHandlers(testDB, testDB)
	r := chi.NewRouter()
	r.Get("/api/v1/tags", h.List)
	r.Post("/api/v1/tags", h.Create)
	r.Patch("/api/v1/tags/{slug}", h.Update)
	r.Delete("/api/v1/tags/{slug}", h.Delete)
	return r
}

func fireTag(t *testing.T, router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Buffer
	if body == "" {
		reader = bytes.NewBufferString("{}")
	} else {
		reader = bytes.NewBufferString(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestTagHandler_ListEmpty(t *testing.T) {
	resetTables(t)
	router := newTagRouter()

	rec := fireTag(t, router, http.MethodGet, "/api/v1/tags", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /tags: got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if body.Items == nil || len(body.Items) != 0 {
		t.Errorf("want items: [], got %s", rec.Body.String())
	}
}

func TestTagHandler_CreateListUpdateDelete(t *testing.T) {
	resetTables(t)
	router := newTagRouter()

	// Create.
	rec := fireTag(t, router, http.MethodPost, "/api/v1/tags",
		`{"slug":"early-access","name":"Early Access","description":"Pre-GA","color":"yellow"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /tags: got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Slug   string `json:"slug"`
		Name   string `json:"name"`
		Color  string `json:"color"`
		Active bool   `json:"active"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding created tag: %v", err)
	}
	if created.Slug != "early-access" || created.Name != "Early Access" || created.Color != "yellow" || !created.Active {
		t.Errorf("unexpected created tag: %+v", created)
	}

	// Duplicate slug → 409.
	if rec := fireTag(t, router, http.MethodPost, "/api/v1/tags",
		`{"slug":"early-access","name":"Again"}`); rec.Code != http.StatusConflict {
		t.Errorf("duplicate create: got %d, want 409", rec.Code)
	}

	// Listing includes it; default color applies on a minimal create.
	rec = fireTag(t, router, http.MethodPost, "/api/v1/tags", `{"slug":"free","name":"Free"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("minimal create: got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"color":"gray"`) {
		t.Errorf("minimal create should default color to gray: %s", rec.Body.String())
	}

	// Patch display fields + deactivate.
	rec = fireTag(t, router, http.MethodPatch, "/api/v1/tags/early-access",
		`{"name":"Beta","color":"orange","active":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /tags/early-access: got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"active":false`) || !strings.Contains(rec.Body.String(), `"name":"Beta"`) {
		t.Errorf("unexpected patch result: %s", rec.Body.String())
	}

	// Deactivated tags stay listed for display resolution.
	rec = fireTag(t, router, http.MethodGet, "/api/v1/tags", "")
	var listing struct {
		Items []struct {
			Slug   string `json:"slug"`
			Active bool   `json:"active"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listing); err != nil {
		t.Fatalf("decoding listing: %v", err)
	}
	if len(listing.Items) != 2 {
		t.Fatalf("want 2 tags in listing, got %d", len(listing.Items))
	}

	// Patch of a missing tag → 404.
	if rec := fireTag(t, router, http.MethodPatch, "/api/v1/tags/nope", `{"name":"X"}`); rec.Code != http.StatusNotFound {
		t.Errorf("patch missing: got %d, want 404", rec.Code)
	}

	// Delete unused → 204; again → 404.
	if rec := fireTag(t, router, http.MethodDelete, "/api/v1/tags/free", ""); rec.Code != http.StatusNoContent {
		t.Errorf("delete unused: got %d, want 204", rec.Code)
	}
	if rec := fireTag(t, router, http.MethodDelete, "/api/v1/tags/free", ""); rec.Code != http.StatusNotFound {
		t.Errorf("delete missing: got %d, want 404", rec.Code)
	}
}

func TestTagHandler_CreateValidation(t *testing.T) {
	resetTables(t)
	router := newTagRouter()

	tests := []struct {
		name string
		body string
	}{
		{"missing name", `{"slug":"x"}`},
		{"missing slug", `{"name":"X"}`},
		{"bad slug", `{"slug":"Not A Slug","name":"X"}`},
		{"bad color", `{"slug":"x","name":"X","color":"magenta"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if rec := fireTag(t, router, http.MethodPost, "/api/v1/tags", tt.body); rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("got %d, want 422\nbody: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestTagHandler_DeleteInUseConflicts(t *testing.T) {
	resetTables(t)
	router := newTagRouter()

	if rec := fireTag(t, router, http.MethodPost, "/api/v1/tags",
		`{"slug":"free","name":"Free","color":"green"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create tag: got %d", rec.Code)
	}

	seedMCPServer(t, "acme", "srv")
	srv, err := testDB.GetMCPServer(context.Background(), "acme", "srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if _, err := testDB.CreateMCPServerVersion(context.Background(), store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "1.0.0", Runtime: "stdio",
		Packages: validPackages, ProtocolVersion: "2025-01-01",
		Tags: []string{"free"},
	}); err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}

	rec := fireTag(t, router, http.MethodDelete, "/api/v1/tags/free", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("delete in-use: got %d, want 409\nbody: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "deactivate") {
		t.Errorf("conflict body should point at deactivation: %s", rec.Body.String())
	}
}

func TestMCPHandler_CreateVersion_TagValidation(t *testing.T) {
	resetTables(t)
	tagRouter := newTagRouter()
	mcpRouter := newMCPRouter()

	if rec := fireTag(t, tagRouter, http.MethodPost, "/api/v1/tags",
		`{"slug":"free","name":"Free"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create tag: got %d", rec.Code)
	}
	if rec := fireTag(t, tagRouter, http.MethodPatch, "/api/v1/tags/free", `{}`); rec.Code != http.StatusOK {
		t.Fatalf("noop patch: got %d", rec.Code)
	}
	seedMCPServer(t, "acme", "srv")

	// Unknown tag → 422 naming the offender.
	rec := fireTag(t, mcpRouter, http.MethodPost, "/api/v1/mcp/servers/acme/srv/versions",
		`{"version":"1.0.0","runtime":"stdio","protocol_version":"2025-01-01","tags":["free","nope"]}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("unknown tag: got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "nope") {
		t.Errorf("422 body should name the unknown slug: %s", rec.Body.String())
	}

	// Valid tags stick (duplicates collapse, output sorted).
	rec = fireTag(t, mcpRouter, http.MethodPost, "/api/v1/mcp/servers/acme/srv/versions",
		`{"version":"1.0.0","runtime":"stdio","protocol_version":"2025-01-01","tags":["free","free"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid tags: got %d\nbody: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Tags []string `json:"tags"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decoding version: %v", err)
	}
	if len(created.Tags) != 1 || created.Tags[0] != "free" {
		t.Errorf("created version tags = %v, want [free]", created.Tags)
	}

	// A deactivated tag can no longer be ticked.
	if rec := fireTag(t, tagRouter, http.MethodPatch, "/api/v1/tags/free", `{"active":false}`); rec.Code != http.StatusOK {
		t.Fatalf("deactivate: got %d", rec.Code)
	}
	rec = fireTag(t, mcpRouter, http.MethodPost, "/api/v1/mcp/servers/acme/srv/versions",
		`{"version":"2.0.0","runtime":"stdio","protocol_version":"2025-01-01","tags":["free"]}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("deactivated tag tick: got %d, want 422\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestTagHandler_ManagedTagsAreReadOnly(t *testing.T) {
	resetTables(t)
	router := newTagRouter()

	if err := testDB.ReconcileManagedInstanceTags(context.Background(), []store.ManagedTagSpec{
		{Slug: "free", Name: "Free", Color: "green", Active: true},
	}); err != nil {
		t.Fatalf("ReconcileManagedInstanceTags: %v", err)
	}

	// The listing surfaces the managed flag so the UI can disable actions.
	rec := fireTag(t, router, http.MethodGet, "/api/v1/tags", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"managed":true`) {
		t.Errorf("listing should expose managed:true: %d %s", rec.Code, rec.Body.String())
	}

	// PATCH and DELETE both answer 409 pointing at the configuration.
	rec = fireTag(t, router, http.MethodPatch, "/api/v1/tags/free", `{"name":"Hacked"}`)
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "configuration") {
		t.Errorf("patch managed: got %d %s, want 409 mentioning configuration", rec.Code, rec.Body.String())
	}
	rec = fireTag(t, router, http.MethodDelete, "/api/v1/tags/free", "")
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "configuration") {
		t.Errorf("delete managed: got %d %s, want 409 mentioning configuration", rec.Code, rec.Body.String())
	}

	// Managed tags remain tickable on versions like any active tag.
	mcpRouter := newMCPRouter()
	seedMCPServer(t, "acme", "srv")
	rec = fireTag(t, mcpRouter, http.MethodPost, "/api/v1/mcp/servers/acme/srv/versions",
		`{"version":"1.0.0","runtime":"stdio","protocol_version":"2025-01-01","tags":["free"]}`)
	if rec.Code != http.StatusCreated {
		t.Errorf("ticking a managed tag should work: got %d %s", rec.Code, rec.Body.String())
	}
}
