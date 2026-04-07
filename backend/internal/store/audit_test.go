package store_test

import (
	"context"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

func TestAuditLog_LogAndList(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Log a few events.
	pubID := insertPublisher(t, "audit-ns", "Audit Corp")

	srv, _ := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "audit-srv", Name: "Audit Server",
	})

	events := []domain.AuditEvent{
		{
			ActorSubject: "user-uuid-1",
			ActorEmail:   "alice@example.com",
			Action:       domain.ActionMCPServerCreated,
			ResourceType: "mcp_server",
			ResourceID:   srv.ID,
			ResourceNS:   "audit-ns",
			ResourceSlug: "audit-srv",
		},
		{
			ActorSubject: "user-uuid-1",
			ActorEmail:   "alice@example.com",
			Action:       domain.ActionMCPServerVisibility,
			ResourceType: "mcp_server",
			ResourceID:   srv.ID,
			ResourceNS:   "audit-ns",
			ResourceSlug: "audit-srv",
			Metadata:     map[string]any{"visibility": "public"},
		},
		{
			ActorSubject: "user-uuid-2",
			ActorEmail:   "bob@example.com",
			Action:       domain.ActionPublisherCreated,
			ResourceType: "publisher",
			ResourceID:   pubID,
			ResourceNS:   "audit-ns",
		},
	}

	for _, e := range events {
		sharedDB.LogAuditEvent(ctx, e)
	}

	// List all events — newest first.
	all, err := sharedDB.ListAuditEvents(ctx, store.ListAuditParams{Limit: 50})
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(all) < 3 {
		t.Fatalf("expected at least 3 events, got %d", len(all))
	}

	// Filter by resource type.
	mcpOnly, err := sharedDB.ListAuditEvents(ctx, store.ListAuditParams{
		ResourceType: "mcp_server", Limit: 50,
	})
	if err != nil {
		t.Fatalf("ListAuditEvents by resource_type: %v", err)
	}
	for _, e := range mcpOnly {
		if e.ResourceType != "mcp_server" {
			t.Errorf("expected resource_type mcp_server, got %q", e.ResourceType)
		}
	}

	// Filter by actor.
	byBob, err := sharedDB.ListAuditEvents(ctx, store.ListAuditParams{
		ActorSubject: "user-uuid-2", Limit: 50,
	})
	if err != nil {
		t.Fatalf("ListAuditEvents by actor: %v", err)
	}
	if len(byBob) != 1 {
		t.Errorf("expected 1 event for bob, got %d", len(byBob))
	}
	if byBob[0].Action != domain.ActionPublisherCreated {
		t.Errorf("expected publisher.created, got %s", byBob[0].Action)
	}

	// Metadata round-trip.
	var visEvent *domain.AuditEvent
	for i := range all {
		if all[i].Action == domain.ActionMCPServerVisibility {
			visEvent = &all[i]
			break
		}
	}
	if visEvent == nil {
		t.Fatal("visibility event not found")
	}
	if v, ok := visEvent.Metadata["visibility"]; !ok || v != "public" {
		t.Errorf("metadata visibility = %v, want public", v)
	}
}

func TestAuditLog_Pagination(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "audit-page-ns", "Audit Page Corp")

	// Insert 5 events.
	for i := 0; i < 5; i++ {
		sharedDB.LogAuditEvent(ctx, domain.AuditEvent{
			ActorSubject: "user-1",
			ActorEmail:   "user@example.com",
			Action:       domain.ActionPublisherCreated,
			ResourceType: "publisher",
			ResourceID:   pubID,
		})
	}

	// First page: 3 items.
	page1, err := sharedDB.ListAuditEvents(ctx, store.ListAuditParams{Limit: 3})
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1) != 3 {
		t.Errorf("page1 len = %d, want 3", len(page1))
	}

	// Second page using cursor from last item on page1.
	cursor := store.EncodeCursor(page1[len(page1)-1].CreatedAt, page1[len(page1)-1].ID)
	page2, err := sharedDB.ListAuditEvents(ctx, store.ListAuditParams{Limit: 3, Cursor: cursor})
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2))
	}

	// No overlap between pages.
	seen := map[string]bool{}
	for _, e := range page1 {
		seen[e.ID] = true
	}
	for _, e := range page2 {
		if seen[e.ID] {
			t.Errorf("event %s appeared on both pages", e.ID)
		}
	}
}
