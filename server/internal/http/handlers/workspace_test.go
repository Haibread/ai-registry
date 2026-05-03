package handlers_test

// Tests for the workspace handlers introduced in ADR 0001.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

func newWorkspaceRouter() *chi.Mux {
	h := handlers.NewWorkspaceHandlers(testDB, testDB)
	r := chi.NewRouter()
	r.Route("/api/v1/publishers/{publisher_slug}/workspaces", func(r chi.Router) {
		r.Get("/", h.ListWorkspaces)
		r.Post("/", h.CreateWorkspace)
		r.Get("/{workspace_slug}", h.GetWorkspace)
		r.Patch("/{workspace_slug}", h.PatchWorkspace)
		r.Delete("/{workspace_slug}", h.DeleteWorkspace)
		r.Get("/{workspace_slug}/servers", h.ListWorkspaceServers)
		r.Get("/{workspace_slug}/agents", h.ListWorkspaceAgents)
	})
	return r
}

func TestWorkspaceHandler_CreateAndGet(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")

	payload := `{"slug":"claude-team","name":"Claude Team","description":"team owns claude","contact":"team@acme.example"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/acme/workspaces",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newWorkspaceRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	var created struct {
		ID          string `json:"id"`
		PublisherID string `json:"publisher_id"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.Slug != "claude-team" {
		t.Errorf("slug = %q", created.Slug)
	}
	if created.Description != "team owns claude" {
		t.Errorf("description = %q", created.Description)
	}

	// GET via hierarchical path.
	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/publishers/acme/workspaces/claude-team", nil)
	rec = httptest.NewRecorder()
	newWorkspaceRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: status = %d", rec.Code)
	}
}

func TestWorkspaceHandler_Create_NotFoundOnUnknownPublisher(t *testing.T) {
	resetTables(t)
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/publishers/missing/workspaces",
		bytes.NewBufferString(`{"slug":"x","name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newWorkspaceRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestWorkspaceHandler_Create_ValidationErrors(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	r := newWorkspaceRouter()

	tests := []struct {
		name       string
		payload    string
		wantStatus int
	}{
		{"missing slug", `{"name":"No Slug"}`, http.StatusUnprocessableEntity},
		{"missing name", `{"slug":"no-name"}`, http.StatusUnprocessableEntity},
		{"invalid slug", `{"slug":"Bad Slug!","name":"Bad"}`, http.StatusUnprocessableEntity},
		{"bad json", `{nope}`, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/acme/workspaces",
				bytes.NewBufferString(tc.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestWorkspaceHandler_ConflictOnDuplicateSlug(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	r := newWorkspaceRouter()

	create := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/acme/workspaces",
			bytes.NewBufferString(`{"slug":"default","name":"Default"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}
	if rec := create(); rec.Code != http.StatusCreated {
		t.Fatalf("first create: %d", rec.Code)
	}
	if rec := create(); rec.Code != http.StatusConflict {
		t.Errorf("second create: status = %d, want 409", rec.Code)
	}
}

func TestWorkspaceHandler_List(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	seedPublisher(t, "globex", "Globex")
	r := newWorkspaceRouter()

	for _, slug := range []string{"alpha", "beta", "gamma"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/acme/workspaces",
			bytes.NewBufferString(`{"slug":"`+slug+`","name":"`+slug+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed %s: %d", slug, rec.Code)
		}
	}
	// One workspace under globex with the same slug as one under acme — must
	// not appear in acme's list.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/globex/workspaces",
		bytes.NewBufferString(`{"slug":"alpha","name":"alpha"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed globex/alpha: %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/publishers/acme/workspaces", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var body struct {
		Items []struct {
			Slug        string `json:"slug"`
			PublisherID string `json:"publisher_id"`
		} `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 3 {
		t.Errorf("len = %d, want 3", len(body.Items))
	}
}

func TestWorkspaceHandler_Patch(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	r := newWorkspaceRouter()

	// Create.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/acme/workspaces",
		bytes.NewBufferString(`{"slug":"team","name":"Old"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}

	// Patch name + description.
	req = httptest.NewRequest(http.MethodPatch,
		"/api/v1/publishers/acme/workspaces/team",
		bytes.NewBufferString(`{"name":"New","description":"new desc"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d, body: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "New" {
		t.Errorf("name = %q", got.Name)
	}
	if got.Description != "new desc" {
		t.Errorf("description = %q", got.Description)
	}

	// Empty name rejected.
	req = httptest.NewRequest(http.MethodPatch,
		"/api/v1/publishers/acme/workspaces/team",
		bytes.NewBufferString(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty name: status = %d, want 422", rec.Code)
	}
}

func TestWorkspaceHandler_DeleteEmpty(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	r := newWorkspaceRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/acme/workspaces",
		bytes.NewBufferString(`{"slug":"team","name":"Team"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodDelete,
		"/api/v1/publishers/acme/workspaces/team", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete: status = %d, want 204", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/publishers/acme/workspaces/team", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("get-after-delete: status = %d, want 404", rec.Code)
	}
}

func TestWorkspaceHandler_ListWorkspaceServers(t *testing.T) {
	resetTables(t)
	pubID := seedPublisher(t, "acme", "Acme")
	r := newWorkspaceRouter()

	// Create two workspaces under acme via the handler.
	for _, slug := range []string{"team-a", "team-b"} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/acme/workspaces",
			bytes.NewBufferString(`{"slug":"`+slug+`","name":"`+slug+`"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Fatalf("seed %s: %d", slug, rec.Code)
		}
	}

	// Fetch each workspace ID so we can attach servers.
	wsA, _ := testDB.ResolveWorkspace(context.Background(), "acme", "team-a")
	wsB, _ := testDB.ResolveWorkspace(context.Background(), "acme", "team-b")

	// Two servers in team-a, one in team-b.
	for _, wsID := range []string{wsA.ID, wsA.ID, wsB.ID} {
		_, err := testDB.CreateMCPServer(context.Background(), store.CreateMCPServerParams{
			PublisherID: pubID,
			WorkspaceID: wsID,
			Slug:        store.NewULID(), // unique
			Name:        "n",
		})
		if err != nil {
			t.Fatalf("create server: %v", err)
		}
	}

	// team-a list should return 2.
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/publishers/acme/workspaces/team-a/servers", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Items []any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Errorf("team-a servers = %d, want 2", len(body.Items))
	}

	// team-b list should return 1.
	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/publishers/acme/workspaces/team-b/servers", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 {
		t.Errorf("team-b servers = %d, want 1", len(body.Items))
	}

	// Unknown workspace → 404.
	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/publishers/acme/workspaces/missing/servers", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown workspace: status = %d, want 404", rec.Code)
	}
}

func TestWorkspaceHandler_ListWorkspaceAgents(t *testing.T) {
	resetTables(t)
	pubID := seedPublisher(t, "acme", "Acme")
	r := newWorkspaceRouter()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/acme/workspaces",
		bytes.NewBufferString(`{"slug":"team","name":"Team"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed workspace: %d", rec.Code)
	}
	ws, _ := testDB.ResolveWorkspace(context.Background(), "acme", "team")

	// One agent in team.
	if _, err := testDB.CreateAgent(context.Background(), store.CreateAgentParams{
		PublisherID: pubID, WorkspaceID: ws.ID, Slug: "planner", Name: "Planner",
	}); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/publishers/acme/workspaces/team/agents", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Items []any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 {
		t.Errorf("team agents = %d, want 1", len(body.Items))
	}
}

func TestWorkspaceHandler_DeleteConflictWhenNonEmpty(t *testing.T) {
	resetTables(t)
	pubID := seedPublisher(t, "acme", "Acme")
	r := newWorkspaceRouter()

	// Create a workspace via the handler.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/publishers/acme/workspaces",
		bytes.NewBufferString(`{"slug":"team","name":"Team"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	var ws struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&ws); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Insert an mcp_server pointing at the workspace.
	srvID := store.NewULID()
	_, err := testDB.Pool.Exec(context.Background(), `
		INSERT INTO mcp_servers (id, publisher_id, workspace_id, slug, name)
		VALUES ($1, $2, $3, 'weather', 'Weather')`,
		srvID, pubID, ws.ID)
	if err != nil {
		t.Fatalf("insert server: %v", err)
	}

	req = httptest.NewRequest(http.MethodDelete,
		"/api/v1/publishers/acme/workspaces/team", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("delete: status = %d, want 409", rec.Code)
	}
}
