package store_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

var validPackages = json.RawMessage(
	`[{"registryType":"npm","identifier":"@scope/pkg","version":"1.0.0","transport":{"type":"stdio"}}]`)

func TestCreateAndGetMCPServer(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubID := insertPublisher(t, "acme", "Acme Corp")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID,
		Slug:        "my-server",
		Name:        "My Server",
		Description: "A test server",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer() error = %v", err)
	}
	if srv.ID == "" {
		t.Error("expected non-empty ID")
	}
	if srv.Status != domain.StatusDraft {
		t.Errorf("status = %v, want draft", srv.Status)
	}
	if srv.Visibility != domain.VisibilityPrivate {
		t.Errorf("visibility = %v, want private", srv.Visibility)
	}

	// GetMCPServer by namespace/slug as admin (no public filter).
	got, err := sharedDB.GetMCPServer(ctx, "acme", "my-server", false)
	if err != nil {
		t.Fatalf("GetMCPServer() error = %v", err)
	}
	if got.ID != srv.ID {
		t.Errorf("id = %v, want %v", got.ID, srv.ID)
	}
	if got.Namespace != "acme" {
		t.Errorf("namespace = %v, want acme", got.Namespace)
	}
}

func TestCreateMCPServer_ConflictOnDuplicateSlug(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme2", "Acme 2")

	params := store.CreateMCPServerParams{PublisherID: pubID, Slug: "dup", Name: "Dup"}
	if _, err := sharedDB.CreateMCPServer(ctx, params); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := sharedDB.CreateMCPServer(ctx, params)
	if err != store.ErrConflict {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestGetMCPServer_NotFoundWhenPrivateAndPublicOnly(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme3", "Acme 3")

	if _, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "priv", Name: "Private",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := sharedDB.GetMCPServer(ctx, "acme3", "priv", true)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for private server with publicOnly=true, got %v", err)
	}
}

func TestMCPServerVersionLifecycle(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "lifecycle", "Lifecycle Corp")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "srv", Name: "Server",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	// Create a draft version.
	ver, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         "1.0.0",
		Runtime:         domain.RuntimeStdio,
		Packages:        validPackages,
		Capabilities:    json.RawMessage(`{"tools":[]}`),
		ProtocolVersion: "2024-11-05",
	})
	if err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}
	if ver.IsPublished() {
		t.Error("newly created version should not be published")
	}

	// Publish it.
	if err := sharedDB.PublishMCPServerVersion(ctx, srv.ID, "1.0.0"); err != nil {
		t.Fatalf("PublishMCPServerVersion: %v", err)
	}

	// Fetch and verify published.
	got, err := sharedDB.GetMCPServerVersion(ctx, srv.ID, "1.0.0")
	if err != nil {
		t.Fatalf("GetMCPServerVersion: %v", err)
	}
	if !got.IsPublished() {
		t.Error("version should be published after publish call")
	}

	// Publishing again should return ErrImmutable.
	if err := sharedDB.PublishMCPServerVersion(ctx, srv.ID, "1.0.0"); err != store.ErrImmutable {
		t.Errorf("expected ErrImmutable on re-publish, got %v", err)
	}

	// Duplicate version should return ErrConflict.
	_, err = sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "1.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, ProtocolVersion: "2024-11-05",
	})
	if err != store.ErrConflict {
		t.Errorf("expected ErrConflict on duplicate version, got %v", err)
	}
}

func TestListMCPServers_Filtering(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub1 := insertPublisher(t, "ns1", "Namespace 1")
	pub2 := insertPublisher(t, "ns2", "Namespace 2")

	// Create a public server under ns1.
	srv1, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pub1, Slug: "public-srv", Name: "Public Server",
	})
	sharedDB.SetMCPServerVisibility(ctx, srv1.ID, domain.VisibilityPublic)

	// Create a private server under ns2.
	sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pub2, Slug: "private-srv", Name: "Private Server",
	})

	// PublicOnly=true should return only public entries.
	rows, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{PublicOnly: true, Limit: 20})
	if err != nil {
		t.Fatalf("ListMCPServers: %v", err)
	}
	for _, r := range rows {
		if r.Visibility != domain.VisibilityPublic {
			t.Errorf("expected public visibility, got %v for server %v", r.Visibility, r.Slug)
		}
	}

	// Namespace filter.
	rows2, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Namespace: "ns1", Limit: 20})
	if err != nil {
		t.Fatalf("ListMCPServers with namespace: %v", err)
	}
	for _, r := range rows2 {
		if r.Namespace != "ns1" {
			t.Errorf("expected namespace ns1, got %v", r.Namespace)
		}
	}
}

func TestDeprecateMCPServer(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "dep-ns", "Deprecate NS")

	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "dep-srv", Name: "Dep Server",
	})

	// Can't deprecate a draft server.
	if err := sharedDB.DeprecateMCPServer(ctx, srv.ID); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound when deprecating draft, got %v", err)
	}

	// Publish a version first, which promotes server to published.
	sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "1.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, ProtocolVersion: "2024-11-05",
	})
	sharedDB.PublishMCPServerVersion(ctx, srv.ID, "1.0.0")

	if err := sharedDB.DeprecateMCPServer(ctx, srv.ID); err != nil {
		t.Fatalf("DeprecateMCPServer: %v", err)
	}
}

func TestGetMCPServerByID(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "byid-ns", "ByID NS")

	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "by-id-server", Name: "By ID Server",
	})

	got, err := sharedDB.GetMCPServerByID(ctx, srv.ID)
	if err != nil {
		t.Fatalf("GetMCPServerByID: %v", err)
	}
	if got.ID != srv.ID {
		t.Errorf("id = %v, want %v", got.ID, srv.ID)
	}

	_, err = sharedDB.GetMCPServerByID(ctx, "nonexistent")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
