package handlers_test

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

func newV0MCPRouter() *chi.Mux {
	h := handlers.NewV0MCPHandlers(testDB, testDB)
	r := chi.NewRouter()
	r.Get("/v0/servers", h.ListServers)
	r.Get("/v0/servers/{id}", h.GetServer)
	r.Route("/v0/servers/{namespace}/{slug}", func(r chi.Router) {
		r.Get("/", h.GetServerByName)
		r.Patch("/status", h.PatchServerStatus)
		r.Get("/versions", h.ListServerVersions)
		r.Get("/versions/{version}", h.GetServerVersion)
		r.Patch("/versions/{version}/status", h.PatchVersionStatus)
	})
	r.Post("/v0/publish", h.Publish)
	return r
}

// seedPublicMCPWithVersion creates a public MCP server with a published version
// and returns the server ID.
func seedPublicMCPWithVersion(t *testing.T, ns, slug, ver string) string {
	t.Helper()
	pubID := seedPublisher(t, ns, ns)

	srv, err := testDB.CreateMCPServer(context.Background(), store.CreateMCPServerParams{
		PublisherID: pubID,
		Slug:        slug,
		Name:        slug,
		Description: "a test server",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	if err := testDB.SetMCPServerVisibility(context.Background(), srv.ID, "public"); err != nil {
		t.Fatalf("SetMCPServerVisibility: %v", err)
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
	if err := testDB.PublishMCPServerVersion(context.Background(), srv.ID, ver); err != nil {
		t.Fatalf("PublishMCPServerVersion: %v", err)
	}

	return srv.ID
}

// ─── V0 ListServers ──────────────────────────────────────────────────────────

func TestV0MCPHandler_ListServers_OnlyPublic(t *testing.T) {
	resetTables(t)

	// Seed one public server (with published version) and one private
	seedPublicMCPWithVersion(t, "v0-pub-ns", "public-srv", "1.0.0")

	// Private server — separate publisher
	pubID2 := seedPublisher(t, "v0-priv-ns", "v0-priv-ns")
	_, err := testDB.CreateMCPServer(context.Background(), store.CreateMCPServerParams{
		PublisherID: pubID2,
		Slug:        "private-srv",
		Name:        "private-srv",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer private: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Servers []struct {
			Name string `json:"name"`
		} `json:"servers"`
		Metadata struct {
			Count int `json:"count"`
		} `json:"metadata"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Metadata.Count != 1 {
		t.Errorf("count = %d, want 1 (only public)", body.Metadata.Count)
	}
	if len(body.Servers) != 1 {
		t.Fatalf("servers len = %d, want 1", len(body.Servers))
	}
	if body.Servers[0].Name != "v0-pub-ns/public-srv" {
		t.Errorf("name = %q, want v0-pub-ns/public-srv", body.Servers[0].Name)
	}
}

func TestV0MCPHandler_ListServers_EmptyList(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Servers  []any `json:"servers"`
		Metadata struct {
			Count int `json:"count"`
		} `json:"metadata"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Servers == nil {
		t.Error("servers should be empty array, not null")
	}
	if len(body.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(body.Servers))
	}
}

func TestV0MCPHandler_ListServers_SearchParam(t *testing.T) {
	resetTables(t)
	seedPublicMCPWithVersion(t, "v0-srch-ns", "searchable", "1.0.0")
	seedPublicMCPWithVersion(t, "v0-srch-ns2", "other-server", "1.0.0")

	// "search" param (spec name)
	req := httptest.NewRequest(http.MethodGet, "/v0/servers?search=searchable", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Servers []any `json:"servers"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Servers) == 0 {
		t.Error("search=searchable: expected at least one result")
	}
}

// ─── V0 GetServer (by ULID) ──────────────────────────────────────────────────

func TestV0MCPHandler_GetServer_FoundPublic(t *testing.T) {
	resetTables(t)
	id := seedPublicMCPWithVersion(t, "v0-get-ns", "v0-get-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/"+id, nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Server struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"server"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Server.ID != id {
		t.Errorf("id = %q, want %q", body.Server.ID, id)
	}
	if body.Server.Name != "v0-get-ns/v0-get-srv" {
		t.Errorf("name = %q, want v0-get-ns/v0-get-srv", body.Server.Name)
	}
}

func TestV0MCPHandler_GetServer_StatusIsActive(t *testing.T) {
	resetTables(t)
	id := seedPublicMCPWithVersion(t, "v0-stat-ns", "stat-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/"+id, nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	var body map[string]any
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	server := body["server"].(map[string]any)
	meta := server["_meta"].(map[string]any)
	official := meta["io.modelcontextprotocol.registry/official"].(map[string]any)
	if official["status"] != "active" {
		t.Errorf("status = %q, want \"active\"", official["status"])
	}
}

func TestV0MCPHandler_GetServer_PrivateReturns404(t *testing.T) {
	resetTables(t)
	// Create a private server (default)
	pubID := seedPublisher(t, "v0-priv-get-ns", "v0-priv-get-ns")
	srv, err := testDB.CreateMCPServer(context.Background(), store.CreateMCPServerParams{
		PublisherID: pubID,
		Slug:        "priv-srv",
		Name:        "priv-srv",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/"+srv.ID, nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestV0MCPHandler_GetServer_UnknownID(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/01JABCDEFGHIJKLMNOPQRSTUV0", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestV0MCPHandler_GetServer_ErrorBodyHasErrorField(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/01JABCDEFGHIJKLMNOPQRSTUV0", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("error body is not valid JSON: %v", err)
	}
	if _, ok := body["error"]; !ok {
		t.Errorf("404 error body missing 'error' field; got keys: %v", mapKeys(body))
	}
}

// ─── V0 GetServerByName ──────────────────────────────────────────────────────

func TestV0MCPHandler_GetServerByName_Found(t *testing.T) {
	resetTables(t)
	seedPublicMCPWithVersion(t, "v0-nm-ns", "nm-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/v0-nm-ns/nm-srv", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Server struct {
			Name string `json:"name"`
		} `json:"server"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Server.Name != "v0-nm-ns/nm-srv" {
		t.Errorf("name = %q, want v0-nm-ns/nm-srv", body.Server.Name)
	}
}

func TestV0MCPHandler_GetServerByName_NotFound(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/no-ns/no-srv", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ─── V0 ListServerVersions ───────────────────────────────────────────────────

func TestV0MCPHandler_ListServerVersions(t *testing.T) {
	resetTables(t)
	seedPublicMCPWithVersion(t, "v0-vs-ns", "vs-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/v0-vs-ns/vs-srv/versions", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Versions []struct {
			Version string `json:"version"`
			Status  string `json:"status"`
		} `json:"versions"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Versions) != 1 {
		t.Fatalf("versions len = %d, want 1", len(body.Versions))
	}
	if body.Versions[0].Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", body.Versions[0].Version)
	}
	if body.Versions[0].Status != "active" {
		t.Errorf("status = %q, want active", body.Versions[0].Status)
	}
}

func TestV0MCPHandler_ListServerVersions_ServerNotFound(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/no-ns/no-srv/versions", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ─── V0 GetServerVersion ─────────────────────────────────────────────────────

func TestV0MCPHandler_GetServerVersion_Found(t *testing.T) {
	resetTables(t)
	seedPublicMCPWithVersion(t, "v0-gv-ns", "gv-srv", "2.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/v0-gv-ns/gv-srv/versions/2.0.0", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Server struct {
			Version string `json:"version"`
		} `json:"server"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Server.Version != "2.0.0" {
		t.Errorf("version = %q, want 2.0.0", body.Server.Version)
	}
}

func TestV0MCPHandler_GetServerVersion_NotFound(t *testing.T) {
	resetTables(t)
	seedPublicMCPWithVersion(t, "v0-gv2-ns", "gv2-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/v0-gv2-ns/gv2-srv/versions/9.9.9", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ─── V0 PatchServerStatus ────────────────────────────────────────────────────

func TestV0MCPHandler_PatchServerStatus_Deprecate(t *testing.T) {
	resetTables(t)
	seedPublicMCPWithVersion(t, "v0-ps-ns", "ps-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{"status": "deprecated"})
	req := httptest.NewRequest(http.MethodPatch, "/v0/servers/v0-ps-ns/ps-srv/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
}

func TestV0MCPHandler_PatchServerStatus_InvalidStatus(t *testing.T) {
	resetTables(t)
	seedPublicMCPWithVersion(t, "v0-ps2-ns", "ps2-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{"status": "invalid"})
	req := httptest.NewRequest(http.MethodPatch, "/v0/servers/v0-ps2-ns/ps2-srv/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

// ─── V0 PatchVersionStatus ───────────────────────────────────────────────────

func TestV0MCPHandler_PatchVersionStatus_Deprecate(t *testing.T) {
	resetTables(t)
	seedPublicMCPWithVersion(t, "v0-pvs-ns", "pvs-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{"status": "deprecated"})
	req := httptest.NewRequest(http.MethodPatch,
		"/v0/servers/v0-pvs-ns/pvs-srv/versions/1.0.0/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204; body: %s", rec.Code, rec.Body.String())
	}
}

func TestV0MCPHandler_PatchVersionStatus_VersionNotFound(t *testing.T) {
	resetTables(t)
	seedPublicMCPWithVersion(t, "v0-pvs2-ns", "pvs2-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{"status": "deprecated"})
	req := httptest.NewRequest(http.MethodPatch,
		"/v0/servers/v0-pvs2-ns/pvs2-srv/versions/9.9.9/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ─── V0 Publish ─────────────────────────────────────────────────────────────

// validPublishBody builds a flat publish payload (spec format — no "server" wrapper).
func validPublishBody(ns, slug, ver string) map[string]any {
	return map[string]any{
		"name":            ns + "/" + slug,
		"description":     "A test server",
		"version":         ver,
		"protocolVersion": "2025-01-01",
		"packages":        json.RawMessage(`[{"registryType":"npm","identifier":"@test/pkg","version":"1.0.0","transport":{"type":"stdio"}}]`),
	}
}

func TestV0MCPHandler_Publish_NewServerAndVersion(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "v0pub-ns", "v0pub-ns")

	body, _ := json.Marshal(validPublishBody("v0pub-ns", "new-srv", "1.0.0"))
	req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	// Spec: 200 OK on successful publish.
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	// Spec: response is ServerResponse shape: { server: ServerDetail }
	var resp struct {
		Server struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"server"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode publish response: %v", err)
	}
	if resp.Server.Name != "v0pub-ns/new-srv" {
		t.Errorf("response server.name = %q, want v0pub-ns/new-srv", resp.Server.Name)
	}
}

func TestV0MCPHandler_Publish_DuplicateVersion(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "v0dup-ns", "v0dup-ns")

	r := newV0MCPRouter()
	body, _ := json.Marshal(validPublishBody("v0dup-ns", "dup-srv", "1.0.0"))

	post := func() int {
		req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post(); code != http.StatusOK {
		t.Fatalf("first publish: %d", code)
	}
	if code := post(); code != http.StatusConflict {
		t.Errorf("duplicate publish: %d, want 409", code)
	}
}

func TestV0MCPHandler_Publish_MissingFields(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "v0mf-ns", "v0mf-ns")

	validPkgs := json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]`)

	tests := []struct {
		name string
		body map[string]any
	}{
		{
			"missing name",
			map[string]any{
				"description": "desc", "version": "1.0.0", "protocolVersion": "2025-01-01",
				"packages": validPkgs,
			},
		},
		{
			"missing version",
			map[string]any{
				"name": "v0mf-ns/srv", "description": "desc", "protocolVersion": "2025-01-01",
				"packages": validPkgs,
			},
		},
		{
			"missing protocolVersion",
			map[string]any{
				"name": "v0mf-ns/srv", "description": "desc", "version": "1.0.0",
				"packages": validPkgs,
			},
		},
		{
			"missing description",
			map[string]any{
				"name": "v0mf-ns/srv", "version": "1.0.0", "protocolVersion": "2025-01-01",
				"packages": validPkgs,
			},
		},
	}

	r := newV0MCPRouter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status = %d, want 422", tt.name, rec.Code)
			}
		})
	}
}

func TestV0MCPHandler_Publish_BadNameFormat(t *testing.T) {
	resetTables(t)

	tests := []struct {
		name string
		body map[string]any
	}{
		{
			"no slash",
			map[string]any{
				"name":            "noslash",
				"description":     "desc",
				"version":         "1.0.0",
				"protocolVersion": "2025-01-01",
				"packages":        json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]`),
			},
		},
		{
			"invalid chars in namespace",
			map[string]any{
				"name":            "invalid ns!/srv",
				"description":     "desc",
				"version":         "1.0.0",
				"protocolVersion": "2025-01-01",
				"packages":        json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]`),
			},
		},
	}

	r := newV0MCPRouter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, _ := json.Marshal(tt.body)
			req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status = %d, want 422", tt.name, rec.Code)
			}
		})
	}
}

func TestV0MCPHandler_Publish_BadPackages(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "v0bp-ns", "v0bp-ns")

	tests := []struct {
		name     string
		packages json.RawMessage
	}{
		{
			"empty required fields",
			json.RawMessage(`[{"registryType":"","identifier":"","version":"","transport":{"type":""}}]`),
		},
		{
			"invalid registryType",
			json.RawMessage(`[{"registryType":"maven","identifier":"com.example:pkg","version":"1.0.0","transport":{"type":"stdio"}}]`),
		},
		{
			"version is latest",
			json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"latest","transport":{"type":"stdio"}}]`),
		},
	}

	r := newV0MCPRouter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := map[string]any{
				"name":            "v0bp-ns/srv",
				"description":     "desc",
				"version":         "1.0.0",
				"protocolVersion": "2025-01-01",
				"packages":        tt.packages,
			}
			b, _ := json.Marshal(body)
			req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status = %d, want 422", tt.name, rec.Code)
			}
		})
	}
}

func TestV0MCPHandler_Publish_MissingPublisher(t *testing.T) {
	resetTables(t)
	// Publisher does NOT exist

	body, _ := json.Marshal(validPublishBody("no-such-publisher", "srv", "1.0.0"))
	req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

// TestV0MCPHandler_Publish_TransportTypes exercises deriveRuntime for different
// transport values so that all branches are covered.
func TestV0MCPHandler_Publish_TransportTypes(t *testing.T) {
	transports := []struct {
		name      string
		transport string
	}{
		{"http", "http"},
		{"sse", "sse"},
		{"streamable-http", "streamable-http"},
	}

	for _, tt := range transports {
		t.Run(tt.name, func(t *testing.T) {
			resetTables(t)
			pubNS := "v0tr-" + tt.transport
			seedPublisher(t, pubNS, pubNS)

			pkg := `[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"` + tt.transport + `"}}]`
			body := map[string]any{
				"name":            pubNS + "/srv",
				"description":     "desc",
				"version":         "1.0.0",
				"protocolVersion": "2025-01-01",
				"packages":        json.RawMessage(pkg),
			}
			b, _ := json.Marshal(body)
			req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			newV0MCPRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("transport=%q: status = %d, want 200; body: %s",
					tt.transport, rec.Code, rec.Body.String())
			}
		})
	}
}
