package store_test

import (
	"context"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

func TestChangelog_EmptyWhenNoPublished(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	entries, err := sharedDB.ListChangelog(ctx, 50)
	if err != nil {
		t.Fatalf("ListChangelog: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestChangelog_IncludesPublishedPublicVersions(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubID := insertPublisher(t, "cl-ns", "Changelog Publisher")

	// MCP server with a published version
	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "pubd-srv", Name: "Public Server",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	if err := sharedDB.SetMCPServerVisibility(ctx, srv.ID, domain.VisibilityPublic); err != nil {
		t.Fatalf("SetMCPServerVisibility: %v", err)
	}
	v, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "1.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, Capabilities: []byte(`{}`), ProtocolVersion: "2025-03-26",
	})
	if err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}
	if err := sharedDB.PublishMCPServerVersion(ctx, srv.ID, v.Version); err != nil {
		t.Fatalf("PublishMCPServerVersion: %v", err)
	}

	// Private MCP server with a published version — should be excluded
	priv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "priv-srv", Name: "Private Server",
	})
	pv, _ := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: priv.ID, Version: "1.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, Capabilities: []byte(`{}`), ProtocolVersion: "2025-03-26",
	})
	if err := sharedDB.PublishMCPServerVersion(ctx, priv.ID, pv.Version); err != nil {
		t.Fatalf("PublishMCPServerVersion private: %v", err)
	}

	entries, err := sharedDB.ListChangelog(ctx, 50)
	if err != nil {
		t.Fatalf("ListChangelog: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (only the public one), got %d", len(entries))
	}
	got := entries[0]
	if got.ResourceType != "mcp_server" {
		t.Errorf("resource_type = %q, want mcp_server", got.ResourceType)
	}
	if got.Namespace != "cl-ns" {
		t.Errorf("namespace = %q, want cl-ns", got.Namespace)
	}
	if got.Slug != "pubd-srv" {
		t.Errorf("slug = %q, want pubd-srv", got.Slug)
	}
	if got.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", got.Version)
	}
}

func TestChangelog_ExcludesUnpublishedVersions(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubID := insertPublisher(t, "cl-ns2", "CL2")
	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "draft-srv", Name: "Draft Server",
	})
	_ = sharedDB.SetMCPServerVisibility(ctx, srv.ID, domain.VisibilityPublic)
	// Create draft version — never published
	if _, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "0.1.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, Capabilities: []byte(`{}`), ProtocolVersion: "2025-03-26",
	}); err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}

	entries, err := sharedDB.ListChangelog(ctx, 50)
	if err != nil {
		t.Fatalf("ListChangelog: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries (draft excluded), got %d", len(entries))
	}
}

func TestChangelog_LimitClamped(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Request limit <= 0 should fall back to default (and still succeed)
	if _, err := sharedDB.ListChangelog(ctx, 0); err != nil {
		t.Errorf("limit 0: %v", err)
	}
	// Request limit > 200 should be clamped (and still succeed)
	if _, err := sharedDB.ListChangelog(ctx, 9999); err != nil {
		t.Errorf("limit 9999: %v", err)
	}
}
