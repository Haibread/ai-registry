package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// seedPublishedMCPServer creates a publisher + server and flips the server to
// 'published' so deprecation/visibility-to-public preconditions hold.
func seedPublishedMCPServer(t *testing.T, ctx context.Context, ns, slug string) string {
	t.Helper()
	pubID := insertPublisher(t, ns, ns)
	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: slug, Name: slug,
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	if _, err := sharedDB.Pool.Exec(ctx,
		`UPDATE mcp_servers SET status='published' WHERE id=$1`, srv.ID); err != nil {
		t.Fatalf("publish server: %v", err)
	}
	return srv.ID
}

func readMCPServerRow(t *testing.T, ctx context.Context, id string) (visibility, status, name string) {
	t.Helper()
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT visibility, status, name FROM mcp_servers WHERE id=$1`, id,
	).Scan(&visibility, &status, &name); err != nil {
		t.Fatalf("read server: %v", err)
	}
	return visibility, status, name
}

func createChange(t *testing.T, ctx context.Context, entryID string, action domain.EntryChangeAction, payload string) string {
	t.Helper()
	id, err := sharedDB.CreateEntryChangeRequest(ctx, domain.EntryResourceMCPServer, entryID, action, json.RawMessage(payload), actor())
	if err != nil {
		t.Fatalf("CreateEntryChangeRequest(%s): %v", action, err)
	}
	return id
}

func TestEntryChange_ApproveVisibilityApplies(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")

	id := createChange(t, ctx, srvID, domain.EntryChangeVisibility, `{"visibility":"public"}`)
	if _, err := sharedDB.ApproveEntryChangeRequest(ctx, id, 1, reviewer()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if vis, _, _ := readMCPServerRow(t, ctx, srvID); vis != "public" {
		t.Errorf("visibility = %q, want public", vis)
	}
}

func TestEntryChange_ApproveDeprecationApplies(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")

	id := createChange(t, ctx, srvID, domain.EntryChangeDeprecation, `{}`)
	if _, err := sharedDB.ApproveEntryChangeRequest(ctx, id, 1, reviewer()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, status, _ := readMCPServerRow(t, ctx, srvID); status != "deprecated" {
		t.Errorf("status = %q, want deprecated", status)
	}
}

func TestEntryChange_ApproveMetadataApplies(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")

	id := createChange(t, ctx, srvID, domain.EntryChangeMetadataEdit, `{"name":"Renamed","description":"d","homepage_url":"","repo_url":"","license":"MIT"}`)
	if _, err := sharedDB.ApproveEntryChangeRequest(ctx, id, 1, reviewer()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if _, _, name := readMCPServerRow(t, ctx, srvID); name != "Renamed" {
		t.Errorf("name = %q, want Renamed", name)
	}
}

func TestEntryChange_RejectLeavesEntryUnchanged(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")

	id := createChange(t, ctx, srvID, domain.EntryChangeVisibility, `{"visibility":"public"}`)
	if err := sharedDB.RejectEntryChangeRequest(ctx, id, 1, "not yet", reviewer()); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if vis, _, _ := readMCPServerRow(t, ctx, srvID); vis != "private" {
		t.Errorf("visibility = %q, want private (unchanged)", vis)
	}
	// A rejected change frees the one-pending index: a new one can be created.
	if _, err := sharedDB.CreateEntryChangeRequest(ctx, domain.EntryResourceMCPServer, srvID,
		domain.EntryChangeVisibility, json.RawMessage(`{"visibility":"public"}`), actor()); err != nil {
		t.Fatalf("create after reject: %v", err)
	}
}

func TestEntryChange_WithdrawFreesPendingSlot(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")

	id := createChange(t, ctx, srvID, domain.EntryChangeVisibility, `{"visibility":"public"}`)
	if err := sharedDB.WithdrawEntryChangeRequest(ctx, id, actor()); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if _, ok, err := sharedDB.GetPendingEntryChange(ctx, domain.EntryResourceMCPServer, srvID); err != nil || ok {
		t.Fatalf("GetPendingEntryChange after withdraw: ok=%v err=%v, want false/nil", ok, err)
	}
	// The unique index is freed; a fresh change succeeds.
	createChange(t, ctx, srvID, domain.EntryChangeDeprecation, `{}`)
}

func TestEntryChange_DuplicatePendingConflicts(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")

	createChange(t, ctx, srvID, domain.EntryChangeVisibility, `{"visibility":"public"}`)
	_, err := sharedDB.CreateEntryChangeRequest(ctx, domain.EntryResourceMCPServer, srvID,
		domain.EntryChangeDeprecation, json.RawMessage(`{}`), actor())
	if !errors.Is(err, store.ErrConflict) {
		t.Errorf("second create err = %v, want ErrConflict", err)
	}
}

func TestEntryChange_ApproveRevisionMismatch(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")

	id := createChange(t, ctx, srvID, domain.EntryChangeVisibility, `{"visibility":"public"}`)
	_, err := sharedDB.ApproveEntryChangeRequest(ctx, id, 2, reviewer())
	if !errors.Is(err, store.ErrReviewRevisionMismatch) {
		t.Errorf("approve err = %v, want ErrReviewRevisionMismatch", err)
	}
}

func TestEntryChange_ApproveTwiceStateMismatch(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")

	id := createChange(t, ctx, srvID, domain.EntryChangeVisibility, `{"visibility":"public"}`)
	if _, err := sharedDB.ApproveEntryChangeRequest(ctx, id, 1, reviewer()); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	_, err := sharedDB.ApproveEntryChangeRequest(ctx, id, 1, reviewer())
	if !errors.Is(err, store.ErrReviewStateMismatch) {
		t.Errorf("second approve err = %v, want ErrReviewStateMismatch", err)
	}
}

func TestEntryChange_ApproveNotApplicableWhenEntryGone(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")

	// Queue a deprecation, then move the entry out of 'published' so the
	// state-guarded apply touches zero rows.
	id := createChange(t, ctx, srvID, domain.EntryChangeDeprecation, `{}`)
	if _, err := sharedDB.Pool.Exec(ctx,
		`UPDATE mcp_servers SET status='deleted' WHERE id=$1`, srvID); err != nil {
		t.Fatalf("delete server: %v", err)
	}
	_, err := sharedDB.ApproveEntryChangeRequest(ctx, id, 1, reviewer())
	if !errors.Is(err, store.ErrChangeNotApplicable) {
		t.Errorf("approve err = %v, want ErrChangeNotApplicable", err)
	}
}

func TestEntryChange_QueueListsPendingChange(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedPublishedMCPServer(t, ctx, "acme", "weather")
	createChange(t, ctx, srvID, domain.EntryChangeVisibility, `{"visibility":"public"}`)

	items, err := sharedDB.ListReviewQueue(ctx, store.ListReviewQueueParams{Limit: 20})
	if err != nil {
		t.Fatalf("ListReviewQueue: %v", err)
	}
	var found *store.ReviewQueueItem
	for i := range items {
		if items[i].Kind == store.ReviewQueueItemMCPChange {
			found = &items[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("no mcp_change item in queue; got %d items", len(items))
	}
	if found.Action != string(domain.EntryChangeVisibility) {
		t.Errorf("action = %q, want visibility", found.Action)
	}
	if found.ChangeID == "" {
		t.Error("change_id is empty")
	}
	if len(found.Payload) == 0 {
		t.Error("payload is empty")
	}
}

func TestEntryChange_AgentVisibilityApplies(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "acme")
	ag, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "router", Name: "router",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := sharedDB.Pool.Exec(ctx,
		`UPDATE agents SET status='published' WHERE id=$1`, ag.ID); err != nil {
		t.Fatalf("publish agent: %v", err)
	}

	id, err := sharedDB.CreateEntryChangeRequest(ctx, domain.EntryResourceAgent, ag.ID,
		domain.EntryChangeVisibility, json.RawMessage(`{"visibility":"public"}`), actor())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := sharedDB.ApproveEntryChangeRequest(ctx, id, 1, reviewer()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var vis string
	if err := sharedDB.Pool.QueryRow(ctx, `SELECT visibility FROM agents WHERE id=$1`, ag.ID).Scan(&vis); err != nil {
		t.Fatalf("read agent: %v", err)
	}
	if vis != "public" {
		t.Errorf("agent visibility = %q, want public", vis)
	}
}
