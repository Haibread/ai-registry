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

func TestDeleteWorkspace_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	if err := sharedDB.DeleteWorkspace(ctx, "01J0000000000000000000NOPE"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
