package store_test

import (
	"context"
	"testing"

	"github.com/haibread/ai-registry/internal/store"
)

func TestCreateAndGetWorkspace(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")

	w, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubID,
		Slug:        "claude-team",
		Name:        "Claude team",
		Description: "Stuff the Claude team owns",
		Contact:     "claude-team@acme.example",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace() error = %v", err)
	}
	if w.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if w.PublisherID != pubID {
		t.Errorf("publisher_id = %q, want %q", w.PublisherID, pubID)
	}
	if w.Slug != "claude-team" {
		t.Errorf("slug = %q, want %q", w.Slug, "claude-team")
	}

	got, err := sharedDB.GetWorkspace(ctx, pubID, "claude-team")
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if got.ID != w.ID {
		t.Errorf("id = %q, want %q", got.ID, w.ID)
	}
	if got.Description != "Stuff the Claude team owns" {
		t.Errorf("description = %q", got.Description)
	}
	if got.Contact != "claude-team@acme.example" {
		t.Errorf("contact = %q", got.Contact)
	}
}

func TestCreateWorkspace_ConflictOnDuplicateSlug(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")

	params := store.CreateWorkspaceParams{
		PublisherID: pubID, Slug: "default", Name: "Default",
	}
	if _, err := sharedDB.CreateWorkspace(ctx, params); err != nil {
		t.Fatalf("first create: %v", err)
	}

	if _, err := sharedDB.CreateWorkspace(ctx, params); err != store.ErrConflict {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestCreateWorkspace_AllowsSameSlugUnderDifferentPublishers(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubA := insertPublisher(t, "acme", "Acme")
	pubB := insertPublisher(t, "globex", "Globex")

	if _, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubA, Slug: "default", Name: "Default A",
	}); err != nil {
		t.Fatalf("create under acme: %v", err)
	}
	if _, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubB, Slug: "default", Name: "Default B",
	}); err != nil {
		t.Errorf("create under globex with same slug: %v", err)
	}
}

func TestCreateWorkspace_NotFoundOnUnknownPublisher(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	_, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: "01J0000000000000000000NOPE",
		Slug:        "default",
		Name:        "Default",
	})
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestGetWorkspace_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")

	if _, err := sharedDB.GetWorkspace(ctx, pubID, "missing"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestResolveWorkspace(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")
	if _, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubID, Slug: "claude-team", Name: "Claude team",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	w, err := sharedDB.ResolveWorkspace(ctx, "acme", "claude-team")
	if err != nil {
		t.Fatalf("ResolveWorkspace() error = %v", err)
	}
	if w.PublisherID != pubID {
		t.Errorf("publisher_id = %q, want %q", w.PublisherID, pubID)
	}

	if _, err := sharedDB.ResolveWorkspace(ctx, "acme", "missing"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing slug, got %v", err)
	}
	if _, err := sharedDB.ResolveWorkspace(ctx, "missing-publisher", "claude-team"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing publisher, got %v", err)
	}
}

func TestListWorkspaces(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubA := insertPublisher(t, "acme", "Acme")
	pubB := insertPublisher(t, "globex", "Globex")

	for _, slug := range []string{"alpha", "beta", "gamma"} {
		if _, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
			PublisherID: pubA, Slug: slug, Name: slug,
		}); err != nil {
			t.Fatalf("create %s under acme: %v", slug, err)
		}
	}
	if _, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubB, Slug: "alpha", Name: "alpha",
	}); err != nil {
		t.Fatalf("create alpha under globex: %v", err)
	}

	got, err := sharedDB.ListWorkspaces(ctx, store.ListWorkspacesParams{
		PublisherID: pubA, Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 workspaces under acme, got %d", len(got))
	}
	for _, w := range got {
		if w.PublisherID != pubA {
			t.Errorf("got workspace from wrong publisher: %q (want %q)", w.PublisherID, pubA)
		}
	}
}

func TestUpdateWorkspace(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")
	w, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubID, Slug: "claude-team", Name: "Old name",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := sharedDB.UpdateWorkspace(ctx, w.ID, store.UpdateWorkspaceParams{
		Name:        "New name",
		Description: "New description",
		Contact:     "new@acme.example",
	})
	if err != nil {
		t.Fatalf("UpdateWorkspace() error = %v", err)
	}
	if updated.Name != "New name" {
		t.Errorf("name = %q, want %q", updated.Name, "New name")
	}
	if updated.Description != "New description" {
		t.Errorf("description = %q", updated.Description)
	}
	if !updated.UpdatedAt.After(w.UpdatedAt) {
		t.Errorf("updated_at not advanced: before=%v after=%v", w.UpdatedAt, updated.UpdatedAt)
	}

	if _, err := sharedDB.UpdateWorkspace(ctx, "01J0000000000000000000NOPE", store.UpdateWorkspaceParams{
		Name: "x",
	}); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for unknown id, got %v", err)
	}
}

func TestDeleteWorkspace_EmptyOK(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")
	w, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubID, Slug: "default", Name: "Default",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := sharedDB.DeleteWorkspace(ctx, w.ID); err != nil {
		t.Errorf("DeleteWorkspace() error = %v", err)
	}
	if _, err := sharedDB.GetWorkspaceByID(ctx, w.ID); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeleteWorkspace_ConflictWhenNonEmpty(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")
	w, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubID, Slug: "claude-team", Name: "Claude team",
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Insert an mcp_server pointing at the workspace via the new column.
	srvID := store.NewULID()
	_, err = sharedDB.Pool.Exec(ctx, `
		INSERT INTO mcp_servers (id, publisher_id, workspace_id, slug, name)
		VALUES ($1, $2, $3, 'weather', 'Weather')`,
		srvID, pubID, w.ID)
	if err != nil {
		t.Fatalf("inserting mcp server: %v", err)
	}

	if err := sharedDB.DeleteWorkspace(ctx, w.ID); err != store.ErrConflict {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestBackfillWorkspaces_CreatesDefaultsAndPopulatesFKs(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubA := insertPublisher(t, "acme", "Acme")
	pubB := insertPublisher(t, "globex", "Globex")

	// Pre-insert resources keyed only on publisher_id (workspace_id NULL).
	srvA := store.NewULID()
	if _, err := sharedDB.Pool.Exec(ctx, `
		INSERT INTO mcp_servers (id, publisher_id, slug, name)
		VALUES ($1, $2, 'weather', 'Weather')`, srvA, pubA); err != nil {
		t.Fatalf("seed mcp_servers: %v", err)
	}
	agA := store.NewULID()
	if _, err := sharedDB.Pool.Exec(ctx, `
		INSERT INTO agents (id, publisher_id, slug, name)
		VALUES ($1, $2, 'planner', 'Planner')`, agA, pubA); err != nil {
		t.Fatalf("seed agents: %v", err)
	}

	res, err := sharedDB.BackfillWorkspaces(ctx)
	if err != nil {
		t.Fatalf("BackfillWorkspaces() error = %v", err)
	}
	if res.WorkspacesCreated != 2 {
		t.Errorf("workspaces created = %d, want 2 (one per publisher)", res.WorkspacesCreated)
	}
	if res.ServersBackfilled != 1 {
		t.Errorf("servers backfilled = %d, want 1", res.ServersBackfilled)
	}
	if res.AgentsBackfilled != 1 {
		t.Errorf("agents backfilled = %d, want 1", res.AgentsBackfilled)
	}

	// Default workspace exists under each publisher.
	for _, pubID := range []string{pubA, pubB} {
		w, err := sharedDB.GetWorkspace(ctx, pubID, "default")
		if err != nil {
			t.Errorf("default workspace missing under %s: %v", pubID, err)
			continue
		}
		if w.Slug != "default" || w.Name != "Default workspace" {
			t.Errorf("unexpected default workspace shape: %+v", w)
		}
	}

	// Resource workspace_id is now non-NULL and points at the default workspace.
	var wsForServer string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT workspace_id FROM mcp_servers WHERE id = $1`, srvA).Scan(&wsForServer); err != nil {
		t.Fatalf("read mcp server workspace: %v", err)
	}
	defaultWS, _ := sharedDB.GetWorkspace(ctx, pubA, "default")
	if wsForServer != defaultWS.ID {
		t.Errorf("server workspace_id = %s, want %s (default for acme)", wsForServer, defaultWS.ID)
	}
	var wsForAgent string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT workspace_id FROM agents WHERE id = $1`, agA).Scan(&wsForAgent); err != nil {
		t.Fatalf("read agent workspace: %v", err)
	}
	if wsForAgent != defaultWS.ID {
		t.Errorf("agent workspace_id = %s, want %s", wsForAgent, defaultWS.ID)
	}
}

func TestBackfillWorkspaces_Idempotent(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	insertPublisher(t, "acme", "Acme")

	if _, err := sharedDB.BackfillWorkspaces(ctx); err != nil {
		t.Fatalf("first run: %v", err)
	}

	res, err := sharedDB.BackfillWorkspaces(ctx)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if res.WorkspacesCreated != 0 {
		t.Errorf("second run created %d workspaces, want 0", res.WorkspacesCreated)
	}
	if res.ServersBackfilled != 0 {
		t.Errorf("second run backfilled %d servers, want 0", res.ServersBackfilled)
	}
	if res.AgentsBackfilled != 0 {
		t.Errorf("second run backfilled %d agents, want 0", res.AgentsBackfilled)
	}
}

func TestBackfillWorkspaces_PreservesExistingDefault(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")

	// Pre-create a default workspace; backfill must reuse it, not duplicate.
	existing, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubID, Slug: "default", Name: "Pre-existing default",
	})
	if err != nil {
		t.Fatalf("create default: %v", err)
	}

	res, err := sharedDB.BackfillWorkspaces(ctx)
	if err != nil {
		t.Fatalf("backfill: %v", err)
	}
	if res.WorkspacesCreated != 0 {
		t.Errorf("expected 0 workspaces created, got %d", res.WorkspacesCreated)
	}

	// The pre-existing workspace must still be there with its custom name.
	w, err := sharedDB.GetWorkspace(ctx, pubID, "default")
	if err != nil {
		t.Fatalf("get default: %v", err)
	}
	if w.ID != existing.ID || w.Name != "Pre-existing default" {
		t.Errorf("default workspace replaced: %+v", w)
	}
}

func TestCreateMCPServer_PopulatesWorkspaceID(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID,
		Slug:        "weather",
		Name:        "Weather",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	// The lazy default workspace must now exist.
	defaultWS, err := sharedDB.GetWorkspace(ctx, pubID, "default")
	if err != nil {
		t.Fatalf("expected lazy default workspace: %v", err)
	}

	// And the new server must point at it.
	var wsID string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT workspace_id FROM mcp_servers WHERE id=$1`, srv.ID).Scan(&wsID); err != nil {
		t.Fatalf("read workspace_id: %v", err)
	}
	if wsID != defaultWS.ID {
		t.Errorf("server workspace_id = %s, want %s", wsID, defaultWS.ID)
	}
}

func TestCreateMCPServer_RespectsExplicitWorkspaceID(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")
	customWS, err := sharedDB.CreateWorkspace(ctx, store.CreateWorkspaceParams{
		PublisherID: pubID, Slug: "claude-team", Name: "Claude team",
	})
	if err != nil {
		t.Fatalf("create custom workspace: %v", err)
	}

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID,
		WorkspaceID: customWS.ID,
		Slug:        "weather",
		Name:        "Weather",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	var wsID string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT workspace_id FROM mcp_servers WHERE id=$1`, srv.ID).Scan(&wsID); err != nil {
		t.Fatalf("read workspace_id: %v", err)
	}
	if wsID != customWS.ID {
		t.Errorf("server workspace_id = %s, want %s (custom)", wsID, customWS.ID)
	}
	// No 'default' workspace should have been created since one was supplied.
	if _, err := sharedDB.GetWorkspace(ctx, pubID, "default"); err != store.ErrNotFound {
		t.Errorf("unexpected default workspace creation: %v", err)
	}
}

func TestCreateAgent_PopulatesWorkspaceID(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")

	ag, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID,
		Slug:        "planner",
		Name:        "Planner",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	defaultWS, err := sharedDB.GetWorkspace(ctx, pubID, "default")
	if err != nil {
		t.Fatalf("expected lazy default workspace: %v", err)
	}
	var wsID string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT workspace_id FROM agents WHERE id=$1`, ag.ID).Scan(&wsID); err != nil {
		t.Fatalf("read workspace_id: %v", err)
	}
	if wsID != defaultWS.ID {
		t.Errorf("agent workspace_id = %s, want %s", wsID, defaultWS.ID)
	}
}

func TestDeleteWorkspace_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	if err := sharedDB.DeleteWorkspace(ctx, "01J0000000000000000000NOPE"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
