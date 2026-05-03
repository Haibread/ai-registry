package handlers_test

// Forensic leakage checks for the /v0/ MCP-spec-compatible surface:
// pending_review and rejected versions must never appear on the wire.
// The store-level filter on ListMCPServerVersions / GetMCPServerVersion
// (publicOnly=true → published_at IS NOT NULL) was added in #30; these
// tests pin the HTTP behaviour so a future handler change can't
// silently re-leak the workflow's draft / pending / rejected states.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// seedMCPVersion adds a draft version to an existing server. Caller decides
// whether to submit / publish / reject it afterwards.
func seedMCPVersion(t *testing.T, serverID, version string) {
	t.Helper()
	if _, err := testDB.CreateMCPServerVersion(context.Background(), store.CreateMCPServerVersionParams{
		ServerID:        serverID,
		Version:         version,
		Runtime:         domain.RuntimeStdio,
		Packages:        validPackages,
		ProtocolVersion: "2025-01-01",
	}); err != nil {
		t.Fatalf("CreateMCPServerVersion(%s): %v", version, err)
	}
}

func TestV0MCPHandler_ListServerVersions_OnlyShowsPublished(t *testing.T) {
	resetTables(t)
	srvID := seedPublicMCPWithVersion(t, "leak-ns", "weather", "1.0.0")
	actor := store.Actor{Subject: "submitter-uuid", Email: "submitter@example.com"}

	// 1.1.0 → pending_review.
	seedMCPVersion(t, srvID, "1.1.0")
	if err := testDB.SubmitMCPVersion(context.Background(), srvID, "1.1.0", actor); err != nil {
		t.Fatalf("submit 1.1.0: %v", err)
	}
	// 1.2.0 → rejected. Withdraw 1.1.0 first to free the
	// one-pending-per-entry partial unique index, reject 1.2.0, then
	// re-submit 1.1.0 so the entry has both pending and rejected on
	// the wire when /v0/ is queried.
	seedMCPVersion(t, srvID, "1.2.0")
	if err := testDB.WithdrawMCPVersion(context.Background(), srvID, "1.1.0", actor); err != nil {
		t.Fatalf("withdraw 1.1.0: %v", err)
	}
	if err := testDB.SubmitMCPVersion(context.Background(), srvID, "1.2.0", actor); err != nil {
		t.Fatalf("submit 1.2.0: %v", err)
	}
	if err := testDB.RejectMCPVersion(context.Background(), srvID, "1.2.0", 1, "no", actor); err != nil {
		t.Fatalf("reject 1.2.0: %v", err)
	}
	if err := testDB.SubmitMCPVersion(context.Background(), srvID, "1.1.0", actor); err != nil {
		t.Fatalf("re-submit 1.1.0: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/leak-ns/weather/versions", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Servers []struct {
			Server map[string]any `json:"server"`
		} `json:"servers"`
		Metadata struct {
			Count int `json:"count"`
		} `json:"metadata"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// Only the published 1.0.0 version may appear. The pending 1.1.0
	// and rejected 1.2.0 must be filtered out at the store layer
	// (publicOnly → published_at IS NOT NULL).
	if body.Metadata.Count != 1 {
		t.Errorf("count = %d, want 1 (published only)", body.Metadata.Count)
	}
	for _, e := range body.Servers {
		v := e.Server["version"]
		if v != "1.0.0" {
			t.Errorf("unexpected version on the wire: %v", v)
		}
	}
}

func TestV0MCPHandler_GetServerByName_LatestPublishedOnly(t *testing.T) {
	resetTables(t)
	srvID := seedPublicMCPWithVersion(t, "leak-ns", "weather", "1.0.0")
	actor := store.Actor{Subject: "submitter-uuid", Email: "submitter@example.com"}

	// Add a newer 1.1.0 version and leave it pending.
	seedMCPVersion(t, srvID, "1.1.0")
	if err := testDB.SubmitMCPVersion(context.Background(), srvID, "1.1.0", actor); err != nil {
		t.Fatalf("submit 1.1.0: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v0/servers/leak-ns/weather", nil)
	rec := httptest.NewRecorder()
	newV0MCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	server := body["server"].(map[string]any)
	// The wire must echo the latest *published* version, not the newer
	// pending one.
	if server["version"] != "1.0.0" {
		t.Errorf("version on the wire = %v, want 1.0.0 (pending 1.1.0 must not leak)", server["version"])
	}
}
