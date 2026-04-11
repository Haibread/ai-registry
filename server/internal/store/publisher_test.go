package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/store"
)

func TestCreateAndGetPublisher(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub, err := sharedDB.CreatePublisher(ctx, store.CreatePublisherParams{
		Slug:    "test-org",
		Name:    "Test Organisation",
		Contact: "contact@test-org.example",
	})
	if err != nil {
		t.Fatalf("CreatePublisher() error = %v", err)
	}
	if pub.ID == "" {
		t.Error("expected non-empty ID")
	}
	if pub.Slug != "test-org" {
		t.Errorf("slug = %q, want %q", pub.Slug, "test-org")
	}
	if pub.Verified {
		t.Error("new publisher should not be verified")
	}

	// GetPublisher by slug.
	got, err := sharedDB.GetPublisher(ctx, "test-org")
	if err != nil {
		t.Fatalf("GetPublisher() error = %v", err)
	}
	if got.ID != pub.ID {
		t.Errorf("id = %q, want %q", got.ID, pub.ID)
	}
	if got.Contact != "contact@test-org.example" {
		t.Errorf("contact = %q, want %q", got.Contact, "contact@test-org.example")
	}
}

func TestCreatePublisher_ConflictOnDuplicateSlug(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	params := store.CreatePublisherParams{Slug: "dup-pub", Name: "Dup"}
	if _, err := sharedDB.CreatePublisher(ctx, params); err != nil {
		t.Fatalf("first create: %v", err)
	}

	_, err := sharedDB.CreatePublisher(ctx, params)
	if err != store.ErrConflict {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestGetPublisher_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	_, err := sharedDB.GetPublisher(ctx, "does-not-exist")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListPublishers(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	slugs := []string{"alpha", "beta", "gamma"}
	for _, s := range slugs {
		if _, err := sharedDB.CreatePublisher(ctx, store.CreatePublisherParams{
			Slug: s, Name: s,
		}); err != nil {
			t.Fatalf("create %q: %v", s, err)
		}
	}

	rows, err := sharedDB.ListPublishers(ctx, store.ListPublishersParams{Limit: 10})
	if err != nil {
		t.Fatalf("ListPublishers() error = %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("len = %d, want 3", len(rows))
	}
	// Results should be DESC by created_at; order may vary within same second,
	// but all three slugs must be present.
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.Slug] = true
	}
	for _, s := range slugs {
		if !seen[s] {
			t.Errorf("slug %q missing from results", s)
		}
	}
}

func TestListPublishers_Pagination(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		slug := []string{"p1", "p2", "p3", "p4", "p5"}[i]
		if _, err := sharedDB.CreatePublisher(ctx, store.CreatePublisherParams{
			Slug: slug, Name: slug,
		}); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	page1, err := sharedDB.ListPublishers(ctx, store.ListPublishersParams{Limit: 3})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 3 {
		t.Errorf("page1 len = %d, want 3", len(page1))
	}

	cursor := store.EncodeCursor(page1[len(page1)-1].CreatedAt, page1[len(page1)-1].ID)
	page2, err := sharedDB.ListPublishers(ctx, store.ListPublishersParams{Limit: 3, Cursor: cursor})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2))
	}

	// No overlap between pages.
	seen := map[string]bool{}
	for _, p := range page1 {
		seen[p.ID] = true
	}
	for _, p := range page2 {
		if seen[p.ID] {
			t.Errorf("publisher %s appeared on both pages", p.ID)
		}
	}
}

func TestSetPublisherVerified(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub, err := sharedDB.CreatePublisher(ctx, store.CreatePublisherParams{
		Slug: "verify-me", Name: "Verify Me",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if pub.Verified {
		t.Fatal("should not be verified initially")
	}

	if err := sharedDB.SetPublisherVerified(ctx, pub.ID, true); err != nil {
		t.Fatalf("SetPublisherVerified: %v", err)
	}

	got, _ := sharedDB.GetPublisher(ctx, "verify-me")
	if !got.Verified {
		t.Error("publisher should be verified after update")
	}

	// Unverify.
	if err := sharedDB.SetPublisherVerified(ctx, pub.ID, false); err != nil {
		t.Fatalf("SetPublisherVerified(false): %v", err)
	}
	got, _ = sharedDB.GetPublisher(ctx, "verify-me")
	if got.Verified {
		t.Error("publisher should be unverified after second update")
	}
}

func TestSetPublisherVerified_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	err := sharedDB.SetPublisherVerified(ctx, "nonexistent-id", true)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestGetPublisherBySlug exercises the existing helper used by server/agent creation.
func TestGetPublisherBySlug(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub, _ := sharedDB.CreatePublisher(ctx, store.CreatePublisherParams{
		Slug: "byslug", Name: "By Slug",
	})

	id, err := sharedDB.GetPublisherBySlug(ctx, "byslug")
	if err != nil {
		t.Fatalf("GetPublisherBySlug: %v", err)
	}
	if id != pub.ID {
		t.Errorf("id = %q, want %q", id, pub.ID)
	}

	_, err = sharedDB.GetPublisherBySlug(ctx, "missing")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdatePublisher(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub, err := sharedDB.CreatePublisher(ctx, store.CreatePublisherParams{
		Slug: "upd-pub", Name: "Original", Contact: "original@example.com",
	})
	if err != nil {
		t.Fatalf("CreatePublisher: %v", err)
	}

	updated, err := sharedDB.UpdatePublisher(ctx, pub.ID, store.UpdatePublisherParams{
		Name:    "Updated",
		Contact: "updated@example.com",
	})
	if err != nil {
		t.Fatalf("UpdatePublisher: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("name = %q, want %q", updated.Name, "Updated")
	}
	if updated.Contact != "updated@example.com" {
		t.Errorf("contact = %q, want %q", updated.Contact, "updated@example.com")
	}
	if updated.Slug != pub.Slug {
		t.Errorf("slug changed: got %q, want %q", updated.Slug, pub.Slug)
	}
}

func TestUpdatePublisher_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	_, err := sharedDB.UpdatePublisher(ctx, store.NewULID(), store.UpdatePublisherParams{Name: "X"})
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeletePublisher(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub, err := sharedDB.CreatePublisher(ctx, store.CreatePublisherParams{
		Slug: "del-pub", Name: "Delete Me",
	})
	if err != nil {
		t.Fatalf("CreatePublisher: %v", err)
	}

	if err := sharedDB.DeletePublisher(ctx, pub.ID); err != nil {
		t.Fatalf("DeletePublisher: %v", err)
	}

	// Should be gone.
	_, err = sharedDB.GetPublisher(ctx, "del-pub")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDeletePublisher_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	if err := sharedDB.DeletePublisher(ctx, store.NewULID()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestDeletePublisher_ConflictWithActiveEntries(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub, err := sharedDB.CreatePublisher(ctx, store.CreatePublisherParams{
		Slug: "busy-pub", Name: "Busy Publisher",
	})
	if err != nil {
		t.Fatalf("CreatePublisher: %v", err)
	}
	// Add an active MCP server.
	if _, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pub.ID,
		Slug:        "active-srv",
		Name:        "Active Server",
	}); err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	err = sharedDB.DeletePublisher(ctx, pub.ID)
	if !errors.Is(err, store.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}
