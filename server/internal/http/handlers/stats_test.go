package handlers_test

// Regression tests for the stats handler.
// Root cause of the bug: /api/v1/stats didn't exist, so the admin dashboard
// relied on total_count from list endpoints (which never returned it),
// causing all counts to show "—".

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

func newStatsRouter() *chi.Mux {
	h := handlers.NewStatsHandlers(testDB)
	r := chi.NewRouter()
	r.Get("/api/v1/stats", h.GetStats)
	return r
}

func getStats(t *testing.T) map[string]int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	newStatsRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("stats: status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var counts map[string]int
	if err := json.NewDecoder(rec.Body).Decode(&counts); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	return counts
}

func TestStatsHandler_ZeroOnEmptyDB(t *testing.T) {
	resetTables(t)
	counts := getStats(t)

	for _, key := range []string{"mcp_servers", "agents", "publishers"} {
		if counts[key] != 0 {
			t.Errorf("%s = %d, want 0", key, counts[key])
		}
	}
}

func TestStatsHandler_CountsMatchInserts(t *testing.T) {
	resetTables(t)
	ctx := context.Background()

	pubID := seedPublisher(t, "stats-pub", "Stats Pub")

	// Insert 2 MCP servers and 1 agent.
	for _, slug := range []string{"srv-1", "srv-2"} {
		_, err := testDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
			PublisherID: pubID, Slug: slug, Name: slug,
		})
		if err != nil {
			t.Fatalf("CreateMCPServer(%q): %v", slug, err)
		}
	}
	_, err := testDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "ag-1", Name: "Ag 1",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	counts := getStats(t)

	if counts["publishers"] != 1 {
		t.Errorf("publishers = %d, want 1", counts["publishers"])
	}
	if counts["mcp_servers"] != 2 {
		t.Errorf("mcp_servers = %d, want 2", counts["mcp_servers"])
	}
	if counts["agents"] != 1 {
		t.Errorf("agents = %d, want 1", counts["agents"])
	}
}

func TestStatsHandler_IncludesPrivateAndDraftEntries(t *testing.T) {
	// The admin dashboard must count everything, not just public/published rows.
	resetTables(t)
	ctx := context.Background()

	pubID := seedPublisher(t, "priv-stats", "Priv Stats")

	// Server is private+draft by default after creation.
	if _, err := testDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "hidden", Name: "Hidden",
	}); err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	counts := getStats(t)
	if counts["mcp_servers"] != 1 {
		t.Errorf("mcp_servers = %d, want 1 (private entry must be counted)", counts["mcp_servers"])
	}
}

func TestStatsHandler_ResponseShape(t *testing.T) {
	// Ensure the JSON keys match what the dashboard reads.
	resetTables(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	rec := httptest.NewRecorder()
	newStatsRouter().ServeHTTP(rec, req)

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"mcp_servers", "agents", "publishers"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("response missing key %q", key)
		}
	}
}
