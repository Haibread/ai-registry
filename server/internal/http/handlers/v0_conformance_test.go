// Package handlers_test — MCP Registry wire-format conformance tests.
//
// This file verifies compatibility with the official MCP Server Registry API
// specification:
// https://github.com/modelcontextprotocol/registry/blob/main/docs/reference/api/openapi.yaml
//
// # Structure
//
// Tests are grouped by spec endpoint. Each test is named after the exact spec
// requirement it verifies. Known gaps between our implementation and the spec
// are documented with t.Skip + a "CONFORMANCE GAP:" prefix so they stand out
// in test output and can be grepped:
//
//	go test ./... -v 2>&1 | grep "CONFORMANCE GAP"
//
// A skipped test is NOT a passing test — it is a tracked debt item. Removing
// the t.Skip and making the test pass is the correct way to close a gap.
//
// # Path prefix note
//
// The spec defines paths under /v0.1/. Our implementation uses /v0/ as the
// prefix (a deliberate versioning choice). Conformance is tested against the
// /v0/ paths; the prefix difference is noted but not treated as a gap since
// callers are expected to use our documented paths.
package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/store"
)

// ── Helpers ───────────────────────────────────────────────────────────────────

// conformancePublishBody returns a minimal valid publish payload in the flat
// wire format (no wrapper): body IS the ServerDetail per spec.
func conformancePublishBody(ns, slug, ver string) []byte {
	b, _ := json.Marshal(map[string]any{
		"name":            ns + "/" + slug,
		"description":     "A conformance test server",
		"version":         ver,
		"protocolVersion": "2025-01-01",
		"packages": []map[string]any{{
			"registryType": "npm",
			"identifier":   "@" + ns + "/" + slug,
			"version":      ver,
			"transport":    map[string]string{"type": "stdio"},
		}},
	})
	return b
}

// decodeJSON is a test helper that decodes response body into a generic map.
func decodeJSON(t *testing.T, body *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.NewDecoder(body).Decode(&m); err != nil {
		t.Fatalf("failed to decode JSON response: %v\nbody: %s", err, body.String())
	}
	return m
}

// seedConformanceServer creates a public, published MCP server and returns its ID.
func seedConformanceServer(t *testing.T, ns, slug, ver string) string {
	t.Helper()
	return seedPublicMCPWithVersion(t, ns, slug, ver)
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /v0/servers  (spec: GET /v0.1/servers)
// ─────────────────────────────────────────────────────────────────────────────

// Spec: response MUST have a top-level "servers" key that is an array (never null).
func TestV0Conformance_ListServers_TopLevelServersKey(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := decodeJSON(t, rec.Body)

	rawServers, ok := body["servers"]
	if !ok {
		t.Fatal("response missing required 'servers' key")
	}
	if rawServers == nil {
		t.Fatal("'servers' must not be null; spec requires an array")
	}
	if _, ok := rawServers.([]any); !ok {
		t.Fatalf("'servers' must be an array, got %T", rawServers)
	}
}

// Spec: response MUST have a "metadata" object.
func TestV0Conformance_ListServers_MetadataPresent(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	meta, ok := body["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("response missing 'metadata' object; got %T", body["metadata"])
	}

	// Spec: metadata.count is the number of items in the current page.
	count, ok := meta["count"]
	if !ok {
		t.Error("metadata missing required 'count' field")
	}
	if _, ok := count.(float64); !ok {
		t.Errorf("metadata.count must be a number, got %T", count)
	}
}

// Spec: metadata.count MUST equal len(servers) in the current page.
func TestV0Conformance_ListServers_MetadataCountMatchesServersLen(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-cnt-ns1", "srv1", "1.0.0")
	seedConformanceServer(t, "conf-cnt-ns2", "srv2", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	servers := body["servers"].([]any)
	meta := body["metadata"].(map[string]any)
	count := int(meta["count"].(float64))

	if count != len(servers) {
		t.Errorf("metadata.count = %d, but len(servers) = %d; they must be equal", count, len(servers))
	}
}

// Spec: each server entry MUST have a "name" field in the format "namespace/slug".
func TestV0Conformance_ListServers_EntryNameFormat(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-name-ns", "my-server", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	servers := body["servers"].([]any)
	if len(servers) == 0 {
		t.Fatal("expected at least one server in response")
	}

	entry := servers[0].(map[string]any)
	// Each list item is a ServerResponse: { server: {...}, _meta: {...} }
	serverObj, ok := entry["server"].(map[string]any)
	if !ok {
		t.Fatalf("server list entry missing 'server' object; got %T", entry["server"])
	}
	name, ok := serverObj["name"].(string)
	if !ok || name == "" {
		t.Fatal("server entry missing 'name' string field")
	}
	if !strings.Contains(name, "/") {
		t.Errorf("server name %q must be in 'namespace/slug' format (spec pattern: ^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$)", name)
	}
}

// Spec: each server entry MUST have a "_meta" object with the official registry
// metadata under "io.modelcontextprotocol.registry/official".
func TestV0Conformance_ListServers_EntryMetaShape(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-meta-ns", "meta-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	servers := body["servers"].([]any)
	entry := servers[0].(map[string]any)

	// Each list item is a ServerResponse: { server: {...}, _meta: {...} }
	meta, ok := entry["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("server entry missing '_meta' object; got %T", entry["_meta"])
	}

	official, ok := meta["io.modelcontextprotocol.registry/official"].(map[string]any)
	if !ok {
		t.Fatalf("'_meta' missing 'io.modelcontextprotocol.registry/official' key; got keys: %v", mapKeys(meta))
	}

	// Spec: status must be present.
	if _, ok := official["status"]; !ok {
		t.Error("_meta official missing required 'status' field")
	}

	// Spec: publishedAt must be present for published servers.
	if _, ok := official["publishedAt"]; !ok {
		t.Error("_meta official missing 'publishedAt' for a published server")
	}
}

// Spec: _meta.official.status for a published server MUST be "active".
func TestV0Conformance_ListServers_StatusEnumIsActive(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-status-ns", "status-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	servers := body["servers"].([]any)
	entry := servers[0].(map[string]any)
	// _meta is at the ServerResponse level (not inside server object)
	meta := entry["_meta"].(map[string]any)
	official := meta["io.modelcontextprotocol.registry/official"].(map[string]any)

	status := official["status"].(string)
	if status != "active" {
		t.Errorf("status = %q, spec requires \"active\" for a published server", status)
	}
}

// Spec: GET /v0.1/servers supports a "search" query parameter for substring filtering.
func TestV0Conformance_ListServers_SearchParamName(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-srch-ns", "searchable", "1.0.0")
	seedConformanceServer(t, "conf-srch-ns2", "other", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers?search=searchable", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	servers := body["servers"].([]any)
	// Full-text search on "searchable" may return 1 or more depending on indexing;
	// we assert that the param is accepted and does not cause a 4xx/5xx.
	if rec.Code != http.StatusOK {
		t.Errorf("search param caused non-200 response: status = %d", rec.Code)
	}
	_ = servers
}

// Spec: GET /v0.1/servers supports "updated_since" (RFC 3339 date-time) filter.
// Only servers with updated_at > updated_since are returned.
func TestV0Conformance_ListServers_UpdatedSinceParam(t *testing.T) {
	resetTables(t)

	// Create server A before the filter timestamp.
	seedConformanceServer(t, "conf-us-ns1", "server-a", "1.0.0")

	// Let the DB clock advance by sleeping briefly, then record time T.
	time.Sleep(10 * time.Millisecond)
	filterTime := time.Now().UTC()
	time.Sleep(10 * time.Millisecond)

	// Create server B after the filter timestamp.
	seedConformanceServer(t, "conf-us-ns2", "server-b", "1.0.0")

	// Query with updated_since=T — only server B should appear.
	req := httptest.NewRequest(http.MethodGet,
		"/v0/servers?updated_since="+filterTime.Format(time.RFC3339Nano), nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("updated_since request: status = %d, want 200", rec.Code)
	}
	body := decodeJSON(t, rec.Body)
	servers := body["servers"].([]any)

	if len(servers) != 1 {
		t.Fatalf("updated_since filter: got %d servers, want 1 (server-b only)", len(servers))
	}
	entry := servers[0].(map[string]any)
	serverObj, ok := entry["server"].(map[string]any)
	if !ok {
		t.Fatalf("list entry missing 'server' object; got %T", entry["server"])
	}
	name, _ := serverObj["name"].(string)
	if !strings.Contains(name, "server-b") {
		t.Errorf("updated_since returned wrong server: got %q, want conf-us-ns2/server-b", name)
	}
}

// Spec: GET /v0.1/servers supports "include_deleted=true" to include servers
// with status=deleted. By default, deleted servers are excluded.
func TestV0Conformance_ListServers_IncludeDeletedParam(t *testing.T) {
	resetTables(t)

	const ns, slug = "conf-del-ns", "del-srv"
	seedConformanceServer(t, ns, slug, "1.0.0")

	// Mark the server as deleted via PATCH /status.
	patchBody, _ := json.Marshal(map[string]string{"status": "deleted"})
	patchReq := httptest.NewRequest(http.MethodPatch,
		"/v0/servers/"+ns+"/"+slug+"/status",
		bytes.NewBuffer(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH status to deleted failed: %d %s", patchRec.Code, patchRec.Body.String())
	}

	// Default list must NOT include the deleted server.
	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)
	body := decodeJSON(t, rec.Body)
	for _, s := range body["servers"].([]any) {
		entry := s.(map[string]any)
		serverObj := entry["server"].(map[string]any)
		if strings.Contains(serverObj["name"].(string), slug) {
			t.Error("deleted server must NOT appear in default list (no include_deleted)")
		}
	}

	// With include_deleted=true the deleted server MUST appear.
	req2 := httptest.NewRequest(http.MethodGet, "/v0/servers?include_deleted=true", nil)
	rec2 := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec2, req2)
	body2 := decodeJSON(t, rec2.Body)
	found := false
	for _, s := range body2["servers"].([]any) {
		entry := s.(map[string]any)
		serverObj := entry["server"].(map[string]any)
		if strings.Contains(serverObj["name"].(string), slug) {
			found = true
			break
		}
	}
	if !found {
		t.Error("deleted server MUST appear when include_deleted=true")
	}
}

// Spec: GET /v0.1/servers supports a "version" query parameter.
// version=latest returns each server's most-recently-published version (default
// behaviour). version=<semver> returns only servers that have that exact version
// published, and shows that version's data.
func TestV0Conformance_ListServers_VersionParam(t *testing.T) {
	resetTables(t)

	// Seed server A with only version 1.0.0.
	seedConformanceServer(t, "conf-vf-ns1", "srv-a", "1.0.0")
	// Seed server B with version 2.0.0.
	seedConformanceServer(t, "conf-vf-ns2", "srv-b", "2.0.0")

	// version=latest: both servers appear (spec default; isLatest=true for all).
	req := httptest.NewRequest(http.MethodGet, "/v0/servers?version=latest", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("version=latest: status = %d, want 200", rec.Code)
	}
	body := decodeJSON(t, rec.Body)
	if len(body["servers"].([]any)) < 2 {
		t.Errorf("version=latest: expected ≥2 servers, got %d", len(body["servers"].([]any)))
	}

	// version=1.0.0: only server A should appear.
	req2 := httptest.NewRequest(http.MethodGet, "/v0/servers?version=1.0.0", nil)
	rec2 := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("version=1.0.0: status = %d, want 200", rec2.Code)
	}
	body2 := decodeJSON(t, rec2.Body)
	servers2 := body2["servers"].([]any)
	if len(servers2) != 1 {
		t.Fatalf("version=1.0.0: got %d servers, want 1 (srv-a only)", len(servers2))
	}
	entry := servers2[0].(map[string]any)
	serverObj := entry["server"].(map[string]any)
	if serverObj["version"] != "1.0.0" {
		t.Errorf("version=1.0.0 filter returned wrong version: %q", serverObj["version"])
	}

	// version=2.0.0: only server B should appear.
	req3 := httptest.NewRequest(http.MethodGet, "/v0/servers?version=2.0.0", nil)
	rec3 := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("version=2.0.0: status = %d, want 200", rec3.Code)
	}
	body3 := decodeJSON(t, rec3.Body)
	servers3 := body3["servers"].([]any)
	if len(servers3) != 1 {
		t.Fatalf("version=2.0.0: got %d servers, want 1 (srv-b only)", len(servers3))
	}

	// version=9.9.9: no servers match — must return empty array, not 404.
	req4 := httptest.NewRequest(http.MethodGet, "/v0/servers?version=9.9.9", nil)
	rec4 := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec4, req4)
	if rec4.Code != http.StatusOK {
		t.Fatalf("version=9.9.9: status = %d, want 200", rec4.Code)
	}
	body4 := decodeJSON(t, rec4.Body)
	if len(body4["servers"].([]any)) != 0 {
		t.Errorf("version=9.9.9: expected 0 servers, got %d", len(body4["servers"].([]any)))
	}
}

// Spec: cursor-based pagination: nextCursor in metadata enables fetching the next page.
func TestV0Conformance_ListServers_CursorPagination(t *testing.T) {
	resetTables(t)
	for i := range 5 {
		ns := "conf-page-ns"
		slug := "srv" + string(rune('a'+i))
		pubID := seedPublisher(t, ns+"-"+slug, ns+"-"+slug)
		srv, err := testDB.CreateMCPServer(context.Background(), store.CreateMCPServerParams{
			PublisherID: pubID, Slug: slug, Name: slug,
		})
		if err != nil {
			t.Fatalf("CreateMCPServer: %v", err)
		}
		if err := testDB.SetMCPServerVisibility(context.Background(), srv.ID, "public"); err != nil {
			t.Fatalf("SetMCPServerVisibility: %v", err)
		}
	}

	// First page: limit=2
	req := httptest.NewRequest(http.MethodGet, "/v0/servers?limit=2", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	servers := body["servers"].([]any)
	meta := body["metadata"].(map[string]any)

	if len(servers) != 2 {
		t.Fatalf("page 1: got %d servers, want 2", len(servers))
	}

	cursor, _ := meta["nextCursor"].(string)
	if cursor == "" {
		t.Fatal("metadata.nextCursor must be set when more items exist")
	}

	// Second page using cursor
	req2 := httptest.NewRequest(http.MethodGet, "/v0/servers?limit=2&cursor="+cursor, nil)
	rec2 := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec2, req2)

	body2 := decodeJSON(t, rec2.Body)
	servers2 := body2["servers"].([]any)
	if len(servers2) == 0 {
		t.Error("second page must contain results when cursor was provided")
	}
}

// Spec: Content-Type of all responses must be application/json.
func TestV0Conformance_ListServers_ContentType(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /v0/servers/{id}  (spec: GET /v0.1/servers/{serverName}/versions/{version})
// ─────────────────────────────────────────────────────────────────────────────

// Spec: detail response MUST have a top-level "server" key.
func TestV0Conformance_GetServer_TopLevelServerKey(t *testing.T) {
	resetTables(t)
	id := seedConformanceServer(t, "conf-det-ns", "det-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/"+id, nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body)
	if _, ok := body["server"]; !ok {
		t.Fatal("detail response missing required top-level 'server' key")
	}
}

// Spec: server object must have "name" in namespace/slug format.
func TestV0Conformance_GetServer_NameFormat(t *testing.T) {
	resetTables(t)
	id := seedConformanceServer(t, "conf-nm-ns", "nm-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/"+id, nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	server := body["server"].(map[string]any)
	name, _ := server["name"].(string)
	if !strings.Contains(name, "/") {
		t.Errorf("server.name = %q, must be in 'namespace/slug' format", name)
	}
}

// Spec: server object must include "_meta" with official registry metadata.
func TestV0Conformance_GetServer_MetaShape(t *testing.T) {
	resetTables(t)
	id := seedConformanceServer(t, "conf-dmeta-ns", "dmeta-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/"+id, nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)

	// _meta is at the top response level, not inside server
	meta, ok := body["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("response missing top-level '_meta' object; got %T", body["_meta"])
	}
	if _, ok := meta["io.modelcontextprotocol.registry/official"]; !ok {
		t.Errorf("'_meta' missing 'io.modelcontextprotocol.registry/official'; got keys: %v", mapKeys(meta))
	}
}

// Spec: server lookup is also by serverName (namespace/slug), not just by ULID.
func TestV0Conformance_GetServer_LookupByName(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-lbn-ns", "lbn-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/conf-lbn-ns/lbn-srv", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body)
	server := body["server"].(map[string]any)
	name, _ := server["name"].(string)
	if name != "conf-lbn-ns/lbn-srv" {
		t.Errorf("server.name = %q, want conf-lbn-ns/lbn-srv", name)
	}
}

// Spec: GET /v0.1/servers/{serverName}/versions lists all versions for a server by name.
func TestV0Conformance_ListVersionsByServerName(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-lv-ns", "lv-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/conf-lv-ns/lv-srv/versions", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body)
	// Versions list now uses ServerList shape: { servers: [...], metadata: {...} }
	servers, ok := body["servers"].([]any)
	if !ok {
		t.Fatalf("response missing 'servers' array; got %T", body["servers"])
	}
	if len(servers) == 0 {
		t.Error("expected at least one version for a published server")
	}
}

// Spec: GET /v0.1/servers/{serverName}/versions/{version} returns a specific version.
func TestV0Conformance_GetVersionByServerNameAndVersion(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-gv-ns", "gv-srv", "3.2.1")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/conf-gv-ns/gv-srv/versions/3.2.1", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body)
	if _, ok := body["server"]; !ok {
		t.Fatal("version detail missing top-level 'server' key")
	}
	server := body["server"].(map[string]any)
	if server["version"] != "3.2.1" {
		t.Errorf("server.version = %q, want 3.2.1", server["version"])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// POST /v0/publish  (spec: POST /v0.1/publish)
// ─────────────────────────────────────────────────────────────────────────────

// Spec: publish request body is a ServerDetail object directly (not wrapped).
func TestV0Conformance_Publish_RequestBodyNotWrapped(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "conf-flat-ns", "conf-flat-ns")

	// Send flat body (no "server" wrapper) — should succeed.
	req := httptest.NewRequest(http.MethodPost, "/v0/publish",
		bytes.NewBuffer(conformancePublishBody("conf-flat-ns", "flat-srv", "1.0.0")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("flat body publish failed: status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

// Spec: publish response is a ServerResponse ({ server: ServerDetail, _meta: ... }).
func TestV0Conformance_Publish_ResponseIsServerResponse(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "conf-sr-ns", "conf-sr-ns")

	req := httptest.NewRequest(http.MethodPost, "/v0/publish",
		bytes.NewBuffer(conformancePublishBody("conf-sr-ns", "sr-srv", "1.0.0")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("publish failed: status = %d", rec.Code)
	}
	body := decodeJSON(t, rec.Body)
	if _, ok := body["server"]; !ok {
		t.Fatal("publish response missing top-level 'server' key (spec: ServerResponse shape)")
	}
	server := body["server"].(map[string]any)
	if _, ok := server["name"]; !ok {
		t.Error("publish response server missing 'name' field")
	}
}

// Spec: "name" field must match pattern ^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$
func TestV0Conformance_Publish_NamePatternValidation(t *testing.T) {
	resetTables(t)

	invalidNames := []string{
		"no-slash",
		"invalid ns!/srv",
		"/leading-slash",
		"trailing-slash/",
		"ns/has spaces",
	}
	for _, name := range invalidNames {
		body, _ := json.Marshal(map[string]any{
			"name": name, "description": "desc",
			"version": "1.0.0", "protocolVersion": "2025-01-01",
			"packages": []map[string]any{{
				"registryType": "npm", "identifier": "@t/p", "version": "1.0.0",
				"transport": map[string]string{"type": "stdio"},
			}},
		})
		req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newV0MCPRouter().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("name=%q: status = %d, want 422", name, rec.Code)
		}
	}
}

// Spec: "description" is REQUIRED (1-100 chars) on ServerDetail.
func TestV0Conformance_Publish_DescriptionRequired(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "conf-desc-ns", "conf-desc-ns")

	body, _ := json.Marshal(map[string]any{
		"name": "conf-desc-ns/srv", "version": "1.0.0", "protocolVersion": "2025-01-01",
		"packages": []map[string]any{{
			"registryType": "npm", "identifier": "@t/p", "version": "1.0.0",
			"transport": map[string]string{"type": "stdio"},
		}},
		// description intentionally omitted
	})
	req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 for missing description", rec.Code)
	}
}

// Spec: packages[].registryType must be one of: npm | pypi | oci | nuget | mcpb
func TestV0Conformance_Publish_PackageRegistryTypeEnum(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "conf-rt-ns", "conf-rt-ns")

	invalidTypes := []string{"maven", "gem", "cargo", "composer", "hex"}
	for _, rt := range invalidTypes {
		body, _ := json.Marshal(map[string]any{
			"name": "conf-rt-ns/srv", "description": "desc",
			"version": "1.0.0", "protocolVersion": "2025-01-01",
			"packages": []map[string]any{{
				"registryType": rt, "identifier": "foo/bar", "version": "1.0.0",
				"transport": map[string]string{"type": "stdio"},
			}},
		})
		req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newV0MCPRouter().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("registryType=%q: status = %d, want 422", rt, rec.Code)
		}
	}
}

// Spec: packages[].version MUST NOT be "latest".
func TestV0Conformance_Publish_PackageVersionNotLatest(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "conf-pvl-ns", "conf-pvl-ns")

	body, _ := json.Marshal(map[string]any{
		"name": "conf-pvl-ns/srv", "description": "desc",
		"version": "1.0.0", "protocolVersion": "2025-01-01",
		"packages": []map[string]any{{
			"registryType": "npm", "identifier": "@t/p", "version": "latest",
			"transport": map[string]string{"type": "stdio"},
		}},
	})
	req := httptest.NewRequest(http.MethodPost, "/v0/publish", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 for version='latest'", rec.Code)
	}
}

// Spec: the publish endpoint returns 200 (not 201).
func TestV0Conformance_Publish_StatusCode(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "conf-sc-ns", "conf-sc-ns")

	req := httptest.NewRequest(http.MethodPost, "/v0/publish",
		bytes.NewBuffer(conformancePublishBody("conf-sc-ns", "sc-srv", "1.0.0")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("publish status = %d, spec requires 200", rec.Code)
	}
}

// Spec: publish with a valid body must succeed (happy path smoke test).
func TestV0Conformance_Publish_HappyPath(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "conf-pub-ns", "conf-pub-ns")

	req := httptest.NewRequest(http.MethodPost, "/v0/publish",
		bytes.NewBuffer(conformancePublishBody("conf-pub-ns", "srv", "1.0.0")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("publish failed: status = %d, body: %s", rec.Code, rec.Body.String())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PATCH status endpoints (spec: PATCH /v0.1/servers/{serverName}/status
//                               PATCH /v0.1/servers/{serverName}/versions/{version}/status)
// ─────────────────────────────────────────────────────────────────────────────

// Spec: PATCH /v0.1/servers/{serverName}/status updates all versions' status.
// Returns 200 AllVersionsStatusResponse.
func TestV0Conformance_PatchServerStatus(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-patch-ns", "patch-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{"status": "deprecated"})
	req := httptest.NewRequest(http.MethodPatch, "/v0/servers/conf-patch-ns/patch-srv/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	resp := decodeJSON(t, rec.Body)
	if _, ok := resp["updatedCount"]; !ok {
		t.Error("PATCH server status response missing 'updatedCount'")
	}
}

// Spec: PATCH /v0.1/servers/{serverName}/versions/{version}/status updates one version.
// Returns 200 ServerResponse.
func TestV0Conformance_PatchVersionStatus(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-pvs-ns", "pvs-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{"status": "deprecated"})
	req := httptest.NewRequest(http.MethodPatch,
		"/v0/servers/conf-pvs-ns/pvs-srv/versions/1.0.0/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	resp := decodeJSON(t, rec.Body)
	if _, ok := resp["server"]; !ok {
		t.Error("PATCH version status response missing 'server'")
	}
	if _, ok := resp["_meta"]; !ok {
		t.Error("PATCH version status response missing '_meta'")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Error response shape
// ─────────────────────────────────────────────────────────────────────────────

// Spec: 404 responses have { "error": string }.
func TestV0Conformance_ErrorShape_404(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/conf-no-ns/no-srv", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	body := decodeJSON(t, rec.Body)
	if _, ok := body["error"]; !ok {
		t.Errorf("404 error response missing 'error' field; got keys: %v", mapKeys(body))
	}
	if errStr, ok := body["error"].(string); !ok || errStr == "" {
		t.Error("'error' field must be a non-empty string")
	}
}

// Our error responses must be valid JSON with a non-empty body.
func TestV0Conformance_ErrorShape_IsJSON(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/nonexistent-id-00000000000", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/") {
		t.Errorf("error Content-Type = %q, expected application/json", ct)
	}
	var m map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&m); err != nil {
		t.Errorf("error body is not valid JSON: %v", err)
	}
	if len(m) == 0 {
		t.Error("error body must not be empty")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Wire format field presence — _meta publishedAt is a valid RFC 3339 timestamp
// ─────────────────────────────────────────────────────────────────────────────

func TestV0Conformance_GetServer_PublishedAtIsRFC3339(t *testing.T) {
	resetTables(t)
	id := seedConformanceServer(t, "conf-ts-ns", "ts-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/"+id, nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	// _meta is at the top response level
	meta := body["_meta"].(map[string]any)
	official := meta["io.modelcontextprotocol.registry/official"].(map[string]any)

	publishedAt, ok := official["publishedAt"].(string)
	if !ok || publishedAt == "" {
		t.Fatal("_meta.official.publishedAt must be a non-empty string")
	}
	if _, err := time.Parse(time.RFC3339, publishedAt); err != nil {
		if _, err2 := time.Parse(time.RFC3339Nano, publishedAt); err2 != nil {
			t.Errorf("publishedAt = %q is not a valid RFC 3339 timestamp: %v", publishedAt, err)
		}
	}
}

// Spec: packages field in server entry must be an array when present.
func TestV0Conformance_GetServer_PackagesIsArray(t *testing.T) {
	resetTables(t)
	id := seedConformanceServer(t, "conf-pkgs-ns", "pkgs-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/"+id, nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	server := body["server"].(map[string]any)

	if rawPkgs, ok := server["packages"]; ok && rawPkgs != nil {
		if _, ok := rawPkgs.([]any); !ok {
			t.Errorf("packages must be an array, got %T", rawPkgs)
		}
	}
}

// Spec: each package must have registryType, identifier, and transport fields.
func TestV0Conformance_GetServer_PackageShape(t *testing.T) {
	resetTables(t)
	id := seedConformanceServer(t, "conf-pkgsh-ns", "pkgsh-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/"+id, nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	body := decodeJSON(t, rec.Body)
	server := body["server"].(map[string]any)
	packages, ok := server["packages"].([]any)
	// The shared seeder uses validPackages (mcp_test.go) which always has one
	// npm stdio entry, so the only way this branch triggers is if the seeder
	// regressed. Fail loudly rather than silently skipping — a silent skip
	// would hide a serialization bug on the /v0/ wire format, and that's
	// precisely what this suite exists to catch.
	if !ok {
		t.Fatalf("server.packages must be a JSON array; got %T (%v)", server["packages"], server["packages"])
	}
	if len(packages) == 0 {
		t.Fatal("server.packages is empty — seeder regression: expected validPackages to have at least one entry")
	}

	pkg := packages[0].(map[string]any)
	// registryType + identifier + transport are the three fields the spec
	// requires on every package entry. transport itself is an object; we drill
	// one level deeper because a missing transport.type is the exact bug this
	// test needs to catch (the handler currently defaults it to "stdio" when
	// absent, which would mask the problem in production).
	for _, required := range []string{"registryType", "identifier", "version", "transport"} {
		if _, ok := pkg[required]; !ok {
			t.Errorf("package missing required field %q", required)
		}
	}
	transport, ok := pkg["transport"].(map[string]any)
	if !ok {
		t.Fatalf("package.transport must be an object; got %T", pkg["transport"])
	}
	if _, ok := transport["type"]; !ok {
		t.Error("package.transport.type is required by the MCP spec")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// statusMessage and statusChangedAt
// ─────────────────────────────────────────────────────────────────────────────

// Spec: _meta.official.statusMessage is included in the response when set on a
// deprecated or deleted version.
func TestV0Conformance_PatchVersionStatus_StatusMessageStoredAndReturned(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-sm-ns", "sm-srv", "1.0.0")

	const wantMsg = "deprecated: use v2 instead"
	body, _ := json.Marshal(map[string]string{
		"status":        "deprecated",
		"statusMessage": wantMsg,
	})
	req := httptest.NewRequest(http.MethodPatch,
		"/v0/servers/conf-sm-ns/sm-srv/versions/1.0.0/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH version status: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	resp := decodeJSON(t, rec.Body)
	meta := resp["_meta"].(map[string]any)
	official := meta["io.modelcontextprotocol.registry/official"].(map[string]any)

	gotMsg, _ := official["statusMessage"].(string)
	if gotMsg != wantMsg {
		t.Errorf("statusMessage = %q, want %q", gotMsg, wantMsg)
	}
}

// Spec: statusMessage must NOT be accepted when status is "active".
func TestV0Conformance_PatchVersionStatus_StatusMessageRejectedForActive(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-sma-ns", "sma-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{
		"status":        "active",
		"statusMessage": "this must be rejected",
	})
	req := httptest.NewRequest(http.MethodPatch,
		"/v0/servers/conf-sma-ns/sma-srv/versions/1.0.0/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 when statusMessage is set with status=active", rec.Code)
	}
}

// Spec: _meta.official.statusChangedAt is present after a status change.
func TestV0Conformance_PatchVersionStatus_StatusChangedAtPresent(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-sca-ns", "sca-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{"status": "deprecated"})
	req := httptest.NewRequest(http.MethodPatch,
		"/v0/servers/conf-sca-ns/sca-srv/versions/1.0.0/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH version status: status = %d", rec.Code)
	}

	resp := decodeJSON(t, rec.Body)
	meta := resp["_meta"].(map[string]any)
	official := meta["io.modelcontextprotocol.registry/official"].(map[string]any)

	sca, ok := official["statusChangedAt"].(string)
	if !ok || sca == "" {
		t.Fatal("_meta.official.statusChangedAt must be present and non-empty after a status change")
	}
	if _, err := time.Parse(time.RFC3339, sca); err != nil {
		if _, err2 := time.Parse(time.RFC3339Nano, sca); err2 != nil {
			t.Errorf("statusChangedAt = %q is not a valid RFC 3339 timestamp", sca)
		}
	}
}

// Spec: AllVersionsStatusResponse.servers contains the updated ServerResponse
// items with the new status reflected in _meta.
func TestV0Conformance_PatchServerStatus_ServersContainUpdatedStatus(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-pss-ns", "pss-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{"status": "deprecated"})
	req := httptest.NewRequest(http.MethodPatch,
		"/v0/servers/conf-pss-ns/pss-srv/status",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH server status: status = %d, body: %s", rec.Code, rec.Body.String())
	}

	resp := decodeJSON(t, rec.Body)

	updatedCount, ok := resp["updatedCount"].(float64)
	if !ok {
		t.Fatal("response missing 'updatedCount' field")
	}
	if int(updatedCount) < 1 {
		t.Errorf("updatedCount = %d, want ≥1", int(updatedCount))
	}

	servers, ok := resp["servers"].([]any)
	if !ok {
		t.Fatal("AllVersionsStatusResponse missing 'servers' array")
	}
	if len(servers) == 0 {
		t.Fatal("AllVersionsStatusResponse.servers must not be empty after update")
	}

	entry := servers[0].(map[string]any)
	meta, ok := entry["_meta"].(map[string]any)
	if !ok {
		t.Fatal("server entry in AllVersionsStatusResponse missing '_meta'")
	}
	official := meta["io.modelcontextprotocol.registry/official"].(map[string]any)
	if official["status"] != "deprecated" {
		t.Errorf("_meta.official.status = %q, want deprecated", official["status"])
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PUT and DELETE — spec optional, our implementation returns 501
// ─────────────────────────────────────────────────────────────────────────────

// Spec: PUT /v0.1/servers/{serverName}/versions/{version} is optional.
// Our implementation MUST return 501 Not Implemented (not 404 or 405).
func TestV0Conformance_PutServerVersion_Returns501(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-put-ns", "put-srv", "1.0.0")

	body, _ := json.Marshal(map[string]string{"description": "updated"})
	req := httptest.NewRequest(http.MethodPut,
		"/v0/servers/conf-put-ns/put-srv/versions/1.0.0/",
		bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("PUT version: status = %d, want 501 Not Implemented", rec.Code)
	}
}

// Spec: DELETE /v0.1/servers/{serverName}/versions/{version} is optional.
// Our implementation MUST return 501 Not Implemented (not 404 or 405).
func TestV0Conformance_DeleteServerVersion_Returns501(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-del2-ns", "del2-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodDelete,
		"/v0/servers/conf-del2-ns/del2-srv/versions/1.0.0/", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Errorf("DELETE version: status = %d, want 501 Not Implemented", rec.Code)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GET /v0/servers/{namespace}/{slug}/versions/latest
// ─────────────────────────────────────────────────────────────────────────────

// Spec: version="latest" is a special value that resolves to the most-recently
// published version of a server.
func TestV0Conformance_GetServerVersion_LatestAlias(t *testing.T) {
	resetTables(t)
	seedConformanceServer(t, "conf-lat-ns", "lat-srv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet,
		"/v0/servers/conf-lat-ns/lat-srv/versions/latest/", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET versions/latest: status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := decodeJSON(t, rec.Body)
	if _, ok := body["server"]; !ok {
		t.Fatal("versions/latest response missing 'server' key")
	}
	server := body["server"].(map[string]any)
	if server["version"] != "1.0.0" {
		t.Errorf("versions/latest returned version %q, want 1.0.0", server["version"])
	}
	// isLatest must be true.
	meta := body["_meta"].(map[string]any)
	official := meta["io.modelcontextprotocol.registry/official"].(map[string]any)
	isLatest, _ := official["isLatest"].(bool)
	if !isLatest {
		t.Error("versions/latest: _meta.official.isLatest must be true")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
