package store_test

// Tests for GetRegistryCounts — the query that powers the admin dashboard.
// Regression: dashboard showed "—" for all counts because the endpoint
// didn't exist; these tests ensure the counts are accurate.

import (
	"context"
	"testing"

	"github.com/haibread/ai-registry/internal/store"
)

func TestGetRegistryCounts_Empty(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	counts, err := sharedDB.GetRegistryCounts(ctx)
	if err != nil {
		t.Fatalf("GetRegistryCounts() error = %v", err)
	}
	if counts.MCPServers != 0 || counts.Agents != 0 || counts.Publishers != 0 {
		t.Errorf("expected all zeros on empty DB, got %+v", counts)
	}
}

func TestGetRegistryCounts_ReflectsInserts(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Create 2 publishers.
	pub1ID := insertPublisher(t, "cnt-pub1", "Count Pub 1")
	insertPublisher(t, "cnt-pub2", "Count Pub 2")

	// Create 3 MCP servers under pub1.
	for _, slug := range []string{"srv-a", "srv-b", "srv-c"} {
		if _, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
			PublisherID: pub1ID, Slug: slug, Name: slug,
		}); err != nil {
			t.Fatalf("CreateMCPServer(%q): %v", slug, err)
		}
	}

	// Create 2 agents under pub1.
	for _, slug := range []string{"agent-a", "agent-b"} {
		if _, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
			PublisherID: pub1ID, Slug: slug, Name: slug,
		}); err != nil {
			t.Fatalf("CreateAgent(%q): %v", slug, err)
		}
	}

	counts, err := sharedDB.GetRegistryCounts(ctx)
	if err != nil {
		t.Fatalf("GetRegistryCounts() error = %v", err)
	}
	if counts.Publishers != 2 {
		t.Errorf("Publishers = %d, want 2", counts.Publishers)
	}
	if counts.MCPServers != 3 {
		t.Errorf("MCPServers = %d, want 3", counts.MCPServers)
	}
	if counts.Agents != 2 {
		t.Errorf("Agents = %d, want 2", counts.Agents)
	}
}

func TestGetRegistryCounts_IncludesPrivateEntries(t *testing.T) {
	// Counts are for the admin view — private/draft entries must be included.
	resetDB(t)
	ctx := context.Background()

	pubID := insertPublisher(t, "priv-cnt", "Private Count")

	// Private server (default visibility after CreateMCPServer).
	if _, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "private-srv", Name: "Private",
	}); err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	counts, err := sharedDB.GetRegistryCounts(ctx)
	if err != nil {
		t.Fatalf("GetRegistryCounts: %v", err)
	}
	if counts.MCPServers != 1 {
		t.Errorf("MCPServers = %d, want 1 (private entry must be counted)", counts.MCPServers)
	}
}
