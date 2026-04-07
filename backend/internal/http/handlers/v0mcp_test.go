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

// ─── V0 GetServer ────────────────────────────────────────────────────────────

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

// ─── V0 Publish ─────────────────────────────────────────────────────────────

func validPublishBody(ns, slug, ver string) map[string]any {
	return map[string]any{
		"server": map[string]any{
			"name":            ns + "/" + slug,
			"description":     "A test server",
			"version":         ver,
			"protocolVersion": "2025-01-01",
			"packages":        json.RawMessage(`[{"registryType":"npm","identifier":"@test/pkg","version":"1.0.0","transport":{"type":"stdio"}}]`),
		},
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

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rec.Body).Decode(&resp) //nolint:errcheck
	if resp["message"] == "" {
		t.Error("expected non-empty message")
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

	if code := post(); code != http.StatusCreated {
		t.Fatalf("first publish: %d", code)
	}
	if code := post(); code != http.StatusConflict {
		t.Errorf("duplicate publish: %d, want 409", code)
	}
}

func TestV0MCPHandler_Publish_MissingFields(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "v0mf-ns", "v0mf-ns")

	tests := []struct {
		name string
		body map[string]any
	}{
		{
			"missing name",
			map[string]any{"server": map[string]any{
				"version": "1.0.0", "protocolVersion": "2025-01-01",
				"packages": json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]`),
			}},
		},
		{
			"missing version",
			map[string]any{"server": map[string]any{
				"name": "v0mf-ns/srv", "protocolVersion": "2025-01-01",
				"packages": json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]`),
			}},
		},
		{
			"missing protocolVersion",
			map[string]any{"server": map[string]any{
				"name": "v0mf-ns/srv", "version": "1.0.0",
				"packages": json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]`),
			}},
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

	// Name without slash
	body := map[string]any{
		"server": map[string]any{
			"name":            "noslash",
			"version":         "1.0.0",
			"protocolVersion": "2025-01-01",
			"packages":        json.RawMessage(`[{"registryType":"npm","identifier":"@t/p","version":"1.0.0","transport":{"type":"stdio"}}]`),
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestV0MCPHandler_Publish_BadPackages(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "v0bp-ns", "v0bp-ns")

	// Missing required package fields
	body := map[string]any{
		"server": map[string]any{
			"name":            "v0bp-ns/srv",
			"version":         "1.0.0",
			"protocolVersion": "2025-01-01",
			"packages":        json.RawMessage(`[{"registryType":"","identifier":"","version":"","transport":{"type":""}}]`),
		},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
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
				"server": map[string]any{
					"name":            pubNS + "/srv",
					"version":         "1.0.0",
					"protocolVersion": "2025-01-01",
					"packages":        json.RawMessage(pkg),
				},
			}
			b, _ := json.Marshal(body)
			req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(b))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			newV0MCPRouter().ServeHTTP(rec, req)

			if rec.Code != http.StatusCreated {
				t.Errorf("transport=%q: status = %d, want 201; body: %s",
					tt.transport, rec.Code, rec.Body.String())
			}
		})
	}
}
