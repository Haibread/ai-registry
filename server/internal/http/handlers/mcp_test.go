package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

// validPackages is a well-formed packages JSON used in MCP version tests.
var validPackages = json.RawMessage(`[{"registryType":"npm","identifier":"@test/pkg","version":"1.0.0","transport":{"type":"stdio"}}]`)

func newMCPRouter() *chi.Mux {
	h := handlers.NewMCPHandlers(testDB, testDB, nil)
	r := chi.NewRouter()
	r.Get("/api/v1/mcp/servers", h.ListServers)
	r.Post("/api/v1/mcp/servers", h.CreateServer)
	r.Get("/api/v1/mcp/servers/{namespace}/{slug}", h.GetServer)
	r.Patch("/api/v1/mcp/servers/{namespace}/{slug}", h.PatchServer)
	r.Delete("/api/v1/mcp/servers/{namespace}/{slug}", h.DeleteServer)
	r.Get("/api/v1/mcp/servers/{namespace}/{slug}/versions", h.ListVersions)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/versions", h.CreateVersion)
	r.Get("/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}", h.GetVersion)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/versions/{version}/publish", h.PublishVersion)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/deprecate", h.DeprecateServer)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/visibility", h.SetVisibility)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/view", h.RecordView)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/copy", h.RecordCopy)
	return r
}

// adminCtx returns a context pre-loaded with admin claims.
func adminCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.KeycloakClaims{
		RealmAccess: auth.RealmAccess{Roles: []string{"admin"}},
	})
}

// seedMCPServer creates a publisher + MCP server and returns the server slug/ns pair.
func seedMCPServer(t *testing.T, ns, slug string) {
	t.Helper()
	pubID := seedPublisher(t, ns, ns)
	_, err := testDB.CreateMCPServer(context.Background(), store.CreateMCPServerParams{
		PublisherID: pubID,
		Slug:        slug,
		Name:        slug,
	})
	if err != nil {
		t.Fatalf("seedMCPServer(%q/%q): %v", ns, slug, err)
	}
}

// seedMCPServerPublic creates a public MCP server.
func seedMCPServerPublic(t *testing.T, ns, slug string) {
	t.Helper()
	seedMCPServer(t, ns, slug)
	srv, err := testDB.GetMCPServer(context.Background(), ns, slug, false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if err := testDB.SetMCPServerVisibility(context.Background(), srv.ID, "public"); err != nil {
		t.Fatalf("SetMCPServerVisibility: %v", err)
	}
}

// createMCPVersion creates a version for a server and returns the version string.
func createMCPVersion(t *testing.T, ns, slug, ver string) {
	t.Helper()
	srv, err := testDB.GetMCPServer(context.Background(), ns, slug, false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	_, err = testDB.CreateMCPServerVersion(context.Background(), store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         ver,
		Runtime:         "stdio",
		Packages:        validPackages,
		ProtocolVersion: "2025-01-01",
	})
	if err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}
}

// ─── ListServers ────────────────────────────────────────────────────────────

func TestMCPHandler_ListServers_EmptyList(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []any `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Items == nil {
		t.Error("items should be empty array, not null")
	}
	if len(body.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(body.Items))
	}
}

func TestMCPHandler_ListServers_WithServers(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "pub-list", "server-a")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(body.Items))
	}
}

func TestMCPHandler_ListServers_PublicOnlyFilter(t *testing.T) {
	resetTables(t)
	// private (default) server
	seedMCPServer(t, "pub-vis", "private-srv")
	// public server
	seedMCPServerPublic(t, "pub-vis2", "public-srv")

	// Unauthenticated request — public-only
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	var body struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 1 {
		t.Errorf("public-only: expected 1 item, got %d", len(body.Items))
	}
	if body.Items[0].Slug != "public-srv" {
		t.Errorf("expected public-srv, got %q", body.Items[0].Slug)
	}

	// Admin request — sees everything
	req = httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("admin: expected 2 items, got %d", len(body.Items))
	}
}

// ─── GetServer ──────────────────────────────────────────────────────────────

func TestMCPHandler_GetServer_Found(t *testing.T) {
	resetTables(t)
	seedMCPServerPublic(t, "ns-get", "my-server")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/ns-get/my-server", nil)
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Slug string `json:"slug"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Slug != "my-server" {
		t.Errorf("slug = %q, want my-server", body.Slug)
	}
}

func TestMCPHandler_GetServer_NotFound(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "ns-nf", "ns-nf")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/ns-nf/nope", nil)
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestMCPHandler_GetServer_PrivateHiddenFromPublic(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-priv", "hidden") // private by default

	// Unauthenticated
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/ns-priv/hidden", nil)
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unauthenticated: status = %d, want 404", rec.Code)
	}

	// Admin can see it
	req = httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/ns-priv/hidden", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("admin: status = %d, want 200", rec.Code)
	}
}

// ─── CreateServer ───────────────────────────────────────────────────────────

func TestMCPHandler_CreateServer_Valid(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "create-ns", "Create NS")

	payload := `{"namespace":"create-ns","slug":"new-srv","name":"New Server"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.ID == "" {
		t.Error("expected non-empty id")
	}
	if body.Slug != "new-srv" {
		t.Errorf("slug = %q, want new-srv", body.Slug)
	}
}

func TestMCPHandler_CreateServer_MissingFields(t *testing.T) {
	resetTables(t)

	tests := []struct {
		name    string
		payload string
	}{
		{"missing namespace", `{"slug":"s","name":"N"}`},
		{"missing slug", `{"namespace":"ns","name":"N"}`},
		{"missing name", `{"namespace":"ns","slug":"s"}`},
		{"invalid JSON", `{bad}`},
	}

	r := newMCPRouter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers",
				bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
		})
	}
}

func TestMCPHandler_CreateServer_DuplicateSlug(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "dup-ns", "Dup NS")

	payload := `{"namespace":"dup-ns","slug":"dup-srv","name":"Dup"}`
	post := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers",
			bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newMCPRouter().ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post(); code != http.StatusCreated {
		t.Fatalf("first create: %d", code)
	}
	if code := post(); code != http.StatusConflict {
		t.Errorf("duplicate: %d, want 409", code)
	}
}

// ─── ListVersions ───────────────────────────────────────────────────────────

func TestMCPHandler_ListVersions_Empty(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-lv", "srv-lv")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/ns-lv/srv-lv/versions", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []any `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 0 {
		t.Errorf("expected 0 versions, got %d", len(body.Items))
	}
}

func TestMCPHandler_ListVersions_WithVersions(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-lvv", "srv-lvv")
	createMCPVersion(t, "ns-lvv", "srv-lvv", "1.0.0")
	createMCPVersion(t, "ns-lvv", "srv-lvv", "2.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/ns-lvv/srv-lvv/versions", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []struct {
			Version string `json:"version"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("expected 2 versions, got %d", len(body.Items))
	}
}

// ─── GetVersion ─────────────────────────────────────────────────────────────

func TestMCPHandler_GetVersion_Found(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-gv", "srv-gv")
	createMCPVersion(t, "ns-gv", "srv-gv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/ns-gv/srv-gv/versions/1.0.0", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Version string `json:"version"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", body.Version)
	}
}

func TestMCPHandler_GetVersion_NotFound(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-gvnf", "srv-gvnf")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/ns-gvnf/srv-gvnf/versions/99.0.0", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ─── CreateVersion ──────────────────────────────────────────────────────────

func TestMCPHandler_CreateVersion_Valid(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-cv", "srv-cv")

	body := map[string]any{
		"version":          "1.0.0",
		"runtime":          "stdio",
		"protocol_version": "2025-01-01",
		"packages":         json.RawMessage(`[{"registryType":"npm","identifier":"@test/pkg","version":"1.0.0","transport":{"type":"stdio"}}]`),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-cv/srv-cv/versions",
		bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Version string `json:"version"`
	}
	json.NewDecoder(rec.Body).Decode(&resp) //nolint:errcheck
	if resp.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", resp.Version)
	}
}

func TestMCPHandler_CreateVersion_MissingFields(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-cvmf", "srv-cvmf")

	tests := []struct {
		name    string
		payload string
	}{
		{"missing version", `{"runtime":"stdio","protocol_version":"2025-01-01","packages":[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]}`},
		{"missing runtime", `{"version":"1.0.0","protocol_version":"2025-01-01","packages":[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]}`},
		{"missing protocol_version", `{"version":"1.0.0","runtime":"stdio","packages":[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]}`},
	}

	r := newMCPRouter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-cvmf/srv-cvmf/versions",
				bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(adminCtx())
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status = %d, want 422", tt.name, rec.Code)
			}
		})
	}
}

func TestMCPHandler_CreateVersion_BadPackages(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-cvbp", "srv-cvbp")

	payload := `{"version":"1.0.0","runtime":"stdio","protocol_version":"2025-01-01","packages":[{"registryType":"","identifier":"","version":"","transport":{"type":""}}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-cvbp/srv-cvbp/versions",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

// TestMCPHandler_CreateVersion_WithTools verifies that a valid `tools[]`
// payload is persisted and returned through the create handler. The tool
// list shows up in the response so the admin UI can confirm the save.
func TestMCPHandler_CreateVersion_WithTools(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-cvwt", "srv-cvwt")

	body := map[string]any{
		"version":          "1.0.0",
		"runtime":          "stdio",
		"protocol_version": "2025-01-01",
		"packages":         json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]`),
		"tools": json.RawMessage(`[
			{"name":"read_file","description":"reads"},
			{"name":"write_file","input_schema":{"type":"object"}}
		]`),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-cvwt/srv-cvwt/versions",
		bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Tools) != 2 {
		t.Fatalf("tools len = %d, want 2", len(resp.Tools))
	}
	if resp.Tools[0]["name"] != "read_file" || resp.Tools[1]["name"] != "write_file" {
		t.Errorf("tool names = [%v %v], want [read_file write_file]",
			resp.Tools[0]["name"], resp.Tools[1]["name"])
	}
}

// TestMCPHandler_CreateVersion_InvalidTools rejects a malformed tools
// payload (missing required `name`) with a 422 validation-error before
// touching the database.
func TestMCPHandler_CreateVersion_InvalidTools(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-cvit", "srv-cvit")

	payload := `{
		"version":"1.0.0",
		"runtime":"stdio",
		"protocol_version":"2025-01-01",
		"packages":[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}],
		"tools":[{"description":"missing name"}]
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-cvit/srv-cvit/versions",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
}

// TestMCPHandler_GetServer_IncludesToolsOnLatestVersion verifies that when a
// server has a published version with a tools[] array, the detail endpoint
// surfaces those tools under latest_version.tools. Regression guard for
// forgetting to wire the field into the serverToResponse projection.
func TestMCPHandler_GetServer_IncludesToolsOnLatestVersion(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-gsv", "srv-gsv")
	// Publish a version with tools declared.
	body := map[string]any{
		"version":          "1.0.0",
		"runtime":          "stdio",
		"protocol_version": "2025-01-01",
		"packages":         json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]`),
		"tools":            json.RawMessage(`[{"name":"a"},{"name":"b"},{"name":"c"}]`),
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-gsv/srv-gsv/versions",
		bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: status = %d; body: %s", rec.Code, rec.Body.String())
	}
	// Publish.
	req = httptest.NewRequest(http.MethodPost,
		"/api/v1/mcp/servers/ns-gsv/srv-gsv/versions/1.0.0/publish", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("publish: status = %d; body: %s", rec.Code, rec.Body.String())
	}

	// Fetch the server detail.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers/ns-gsv/srv-gsv", nil)
	req = req.WithContext(adminCtx())
	rec = httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		LatestVersion struct {
			Tools []map[string]any `json:"tools"`
		} `json:"latest_version"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.LatestVersion.Tools) != 3 {
		t.Errorf("latest_version.tools len = %d, want 3 (%+v)", len(resp.LatestVersion.Tools), resp.LatestVersion.Tools)
	}
}

// ─── PublishVersion ─────────────────────────────────────────────────────────

func TestMCPHandler_PublishVersion_Success(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-pv", "srv-pv")
	createMCPVersion(t, "ns-pv", "srv-pv", "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-pv/srv-pv/versions/1.0.0/publish", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body["status"] != "published" {
		t.Errorf("status = %q, want published", body["status"])
	}
}

func TestMCPHandler_PublishVersion_NotFound(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-pvnf", "srv-pvnf")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-pvnf/srv-pvnf/versions/99.0.0/publish", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestMCPHandler_PublishVersion_AlreadyPublished(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-pvap", "srv-pvap")
	createMCPVersion(t, "ns-pvap", "srv-pvap", "1.0.0")

	r := newMCPRouter()

	publish := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-pvap/srv-pvap/versions/1.0.0/publish", nil)
		req = req.WithContext(adminCtx())
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := publish(); code != http.StatusOK {
		t.Fatalf("first publish: %d", code)
	}
	if code := publish(); code != http.StatusConflict {
		t.Errorf("second publish: %d, want 409", code)
	}
}

// ─── DeprecateServer ────────────────────────────────────────────────────────

func TestMCPHandler_DeprecateServer_Success(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-dep", "srv-dep")
	createMCPVersion(t, "ns-dep", "srv-dep", "1.0.0")

	r := newMCPRouter()

	// Publish first
	pubReq := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-dep/srv-dep/versions/1.0.0/publish", nil)
	pubReq = pubReq.WithContext(adminCtx())
	pubRec := httptest.NewRecorder()
	r.ServeHTTP(pubRec, pubReq)
	if pubRec.Code != http.StatusOK {
		t.Fatalf("publish: status = %d", pubRec.Code)
	}

	// Now deprecate
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-dep/srv-dep/deprecate", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body["status"] != "deprecated" {
		t.Errorf("status = %q, want deprecated", body["status"])
	}
}

func TestMCPHandler_DeprecateServer_NotPublished(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "ns-depnp", "srv-depnp") // draft, not published

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/ns-depnp/srv-depnp/deprecate", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

// ─── Status / Visibility filter tests ──────────────────────────────────────

func TestMCPHandler_List_FilterByStatus(t *testing.T) {
	resetTables(t)

	// Seed two servers that will be promoted to published, one left draft.
	seedMCPServer(t, "fst-ns1", "fst-draft")
	seedMCPServerPublished(t, "fst-ns2", "fst-pub-1")
	seedMCPServerPublished(t, "fst-ns3", "fst-pub-2")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers?status=published", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []struct {
			Slug   string `json:"slug"`
			Status string `json:"status"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("status=published: got %d items, want 2", len(body.Items))
	}
	for _, item := range body.Items {
		if item.Status != "published" {
			t.Errorf("expected status=published, got %q for slug %q", item.Status, item.Slug)
		}
	}
}

func TestMCPHandler_List_FilterByVisibility(t *testing.T) {
	resetTables(t)

	// Seed one private (default) and one public server.
	seedMCPServer(t, "fvis-ns1", "fvis-private")
	seedMCPServerPublic(t, "fvis-ns2", "fvis-public")

	// Admin request with visibility=private filter.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers?visibility=private", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []struct {
			Slug       string `json:"slug"`
			Visibility string `json:"visibility"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 1 {
		t.Errorf("visibility=private: got %d items, want 1", len(body.Items))
	}
	if len(body.Items) > 0 && body.Items[0].Slug != "fvis-private" {
		t.Errorf("visibility=private: slug=%q, want fvis-private", body.Items[0].Slug)
	}
}

func TestMCPHandler_List_InvalidFilterIgnored(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "inv-ns1", "inv-srv-1")
	seedMCPServer(t, "inv-ns2", "inv-srv-2")

	// Invalid status and visibility values should be silently ignored,
	// returning all entries visible to an admin.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers?status=badvalue&visibility=garbage", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []any `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("invalid filters ignored: got %d items, want 2", len(body.Items))
	}
}

// seedMCPServerPublished creates an MCP server and sets its status to published via SQL.
func seedMCPServerPublished(t *testing.T, ns, slug string) {
	t.Helper()
	seedMCPServer(t, ns, slug)
	srv, err := testDB.GetMCPServer(context.Background(), ns, slug, false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if _, err := testDB.Pool.Exec(context.Background(), "UPDATE mcp_servers SET status=$1 WHERE id=$2", "published", srv.ID); err != nil {
		t.Fatalf("seedMCPServerPublished: %v", err)
	}
}

func TestMCPHandler_ListServers_TotalCount(t *testing.T) {
	resetTables(t)
	// Seed 3 servers under the same publisher.
	pubID := seedPublisher(t, "tc-pub", "TC Pub")
	for i := range 3 {
		_, err := testDB.CreateMCPServer(context.Background(), store.CreateMCPServerParams{
			PublisherID: pubID,
			Slug:        fmt.Sprintf("tc-srv-%d", i),
			Name:        fmt.Sprintf("TC Server %d", i),
		})
		if err != nil {
			t.Fatalf("creating server %d: %v", i, err)
		}
	}

	// Ask for page of 2 — items should have 2, but total_count should be 3.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers?limit=2", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items      []any `json:"items"`
		TotalCount int   `json:"total_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Errorf("items length: got %d, want 2", len(body.Items))
	}
	if body.TotalCount != 3 {
		t.Errorf("total_count: got %d, want 3", body.TotalCount)
	}
}


func TestMCPHandler_PatchServer_Success(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "patch-ns", "patch-srv")

	payload := `{"name":"New Name","description":"New desc","license":"MIT"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/mcp/servers/patch-ns/patch-srv",
		bytes.NewBufferString(payload))
	req = req.WithContext(adminCtx())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var srv struct {
		Name    string `json:"name"`
		License string `json:"license"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&srv); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if srv.Name != "New Name" {
		t.Errorf("name = %q, want %q", srv.Name, "New Name")
	}
	if srv.License != "MIT" {
		t.Errorf("license = %q, want %q", srv.License, "MIT")
	}
}

func TestMCPHandler_PatchServer_NotFound(t *testing.T) {
	resetTables(t)
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/mcp/servers/ns/missing",
		bytes.NewBufferString(`{"name":"X"}`))
	req = req.WithContext(adminCtx())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestMCPHandler_PatchServer_NameRequired(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "val-ns", "val-srv")

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/mcp/servers/val-ns/val-srv",
		bytes.NewBufferString(`{"name":""}`))
	req = req.WithContext(adminCtx())
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestMCPHandler_DeleteServer_Success(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "del-ns", "del-srv")

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/mcp/servers/del-ns/del-srv", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	// No longer in public listing.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/mcp/servers", nil)
	rec = httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	var body struct {
		Items []any `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 0 {
		t.Errorf("expected empty list after delete, got %d items", len(body.Items))
	}
}

func TestMCPHandler_DeleteServer_NotFound(t *testing.T) {
	resetTables(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/mcp/servers/ns/missing", nil)
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ─── RecordView / RecordCopy ───────────────────────────────────────────────

func TestMCPHandler_RecordView_OK(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "rv-ns", "rv-srv")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/rv-ns/rv-srv/view", nil)
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	// Increment must have hit the DB.
	got, err := testDB.GetMCPServer(context.Background(), "rv-ns", "rv-srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if got.ViewCount != 1 {
		t.Errorf("view_count = %d, want 1", got.ViewCount)
	}
	if got.CopyCount != 0 {
		t.Errorf("copy_count = %d, want 0 (view must not touch copy)", got.CopyCount)
	}
}

func TestMCPHandler_RecordView_NotFound(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "rv-nf", "rv-nf")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/rv-nf/missing/view", nil)
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestMCPHandler_RecordCopy_OK(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "rc-ns", "rc-srv")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/rc-ns/rc-srv/copy", nil)
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}

	got, err := testDB.GetMCPServer(context.Background(), "rc-ns", "rc-srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if got.CopyCount != 1 {
		t.Errorf("copy_count = %d, want 1", got.CopyCount)
	}
	if got.ViewCount != 0 {
		t.Errorf("view_count = %d, want 0 (copy must not touch view)", got.ViewCount)
	}
}

func TestMCPHandler_RecordCopy_NotFound(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/servers/nope/nope/copy", nil)
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
