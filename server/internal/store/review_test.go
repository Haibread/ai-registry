package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// seedDraftMCPVersion creates a publisher + server + draft version and
// returns the server ID. Reused across the workflow tests.
func seedDraftMCPVersion(t *testing.T, ctx context.Context, ns, slug, ver string) string {
	t.Helper()
	pubID := insertPublisher(t, ns, ns)
	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: slug, Name: slug,
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	if _, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         ver,
		Runtime:         domain.RuntimeStdio,
		Packages:        validPackages,
		Capabilities:    json.RawMessage(`{}`),
		ProtocolVersion: "2024-11-05",
	}); err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}
	return srv.ID
}

func actor() store.Actor {
	return store.Actor{Subject: "test-subject", Email: "test@example.com"}
}

func reviewer() store.Actor {
	return store.Actor{Subject: "reviewer-subject", Email: "reviewer@example.com"}
}

// readReviewState returns the (review_state, revision, rejection_reason)
// triple for a version row. Used by tests to assert side effects.
func readReviewState(t *testing.T, ctx context.Context, serverID, version string) (state string, revision int, reason string, publishedAt *string) {
	t.Helper()
	var pa *string
	err := sharedDB.Pool.QueryRow(ctx, `
		SELECT review_state, revision, coalesce(rejection_reason,''),
		       to_char(published_at, 'YYYY-MM-DD"T"HH24:MI:SS')
		FROM mcp_server_versions WHERE server_id=$1 AND version=$2`,
		serverID, version,
	).Scan(&state, &revision, &reason, &pa)
	if err != nil {
		t.Fatalf("read review state: %v", err)
	}
	return state, revision, reason, pa
}

// ── Submit ──────────────────────────────────────────────────────────────

func TestSubmitMCPVersion_DraftToPending(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")

	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	state, _, _, pa := readReviewState(t, ctx, srvID, "1.0.0")
	if state != "pending_review" {
		t.Errorf("state = %q, want pending_review", state)
	}
	if pa != nil {
		t.Errorf("published_at = %v, want NULL on pending", pa)
	}
}

func TestSubmitMCPVersion_RejectedToPendingClearsReason(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")

	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := sharedDB.RejectMCPVersion(ctx, srvID, "1.0.0", 1, "needs more docs", reviewer()); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if _, _, reason, _ := readReviewState(t, ctx, srvID, "1.0.0"); reason != "needs more docs" {
		t.Fatalf("expected reason set after reject, got %q", reason)
	}

	// Re-submit clears the reason and flips back to pending.
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("re-submit: %v", err)
	}
	state, _, reason, _ := readReviewState(t, ctx, srvID, "1.0.0")
	if state != "pending_review" {
		t.Errorf("state = %q, want pending_review", state)
	}
	if reason != "" {
		t.Errorf("reason = %q, want cleared", reason)
	}
}

func TestSubmitMCPVersion_AlreadyPendingIs409Equivalent(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor())
	if !errors.Is(err, store.ErrReviewStateMismatch) {
		t.Errorf("err = %v, want ErrReviewStateMismatch", err)
	}
}

func TestSubmitMCPVersion_StackingRejectedByPartialUniqueIndex(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if _, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        srvID,
		Version:         "1.1.0",
		Runtime:         domain.RuntimeStdio,
		Packages:        validPackages,
		Capabilities:    json.RawMessage(`{}`),
		ProtocolVersion: "2024-11-05",
	}); err != nil {
		t.Fatalf("create v1.1.0: %v", err)
	}
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("submit v1.0.0: %v", err)
	}
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.1.0", actor()); !errors.Is(err, store.ErrConflict) {
		t.Errorf("expected ErrConflict (stacking pending_review), got %v", err)
	}
}

func TestSubmitMCPVersion_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	if err := sharedDB.SubmitMCPVersion(ctx, "01J0NOPE0000000000000NOPE0", "1.0.0", actor()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ── Withdraw ────────────────────────────────────────────────────────────

func TestWithdrawMCPVersion_PendingToDraft(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := sharedDB.WithdrawMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	state, _, _, _ := readReviewState(t, ctx, srvID, "1.0.0")
	if state != "none" {
		t.Errorf("state = %q, want none after withdraw", state)
	}
}

func TestWithdrawMCPVersion_NotPendingMismatch(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	err := sharedDB.WithdrawMCPVersion(ctx, srvID, "1.0.0", actor())
	if !errors.Is(err, store.ErrReviewStateMismatch) {
		t.Errorf("err = %v, want ErrReviewStateMismatch", err)
	}
}

// ── Approve ─────────────────────────────────────────────────────────────

func TestApproveMCPVersion_HappyPath(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if err := sharedDB.ApproveMCPVersion(ctx, srvID, "1.0.0", 1, reviewer()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	state, _, _, pa := readReviewState(t, ctx, srvID, "1.0.0")
	if state != "none" {
		t.Errorf("state = %q, want none after approve", state)
	}
	if pa == nil {
		t.Errorf("published_at not set after approve")
	}

	// Parent server status promoted to 'published'.
	var serverStatus string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT status FROM mcp_servers WHERE id=$1`, srvID).Scan(&serverStatus); err != nil {
		t.Fatalf("read server status: %v", err)
	}
	if serverStatus != "published" {
		t.Errorf("server.status = %q, want published", serverStatus)
	}
}

func TestApproveMCPVersion_RevisionMismatch(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	err := sharedDB.ApproveMCPVersion(ctx, srvID, "1.0.0", 99, reviewer())
	if !errors.Is(err, store.ErrReviewRevisionMismatch) {
		t.Errorf("err = %v, want ErrReviewRevisionMismatch", err)
	}
}

func TestApproveMCPVersion_StateMismatchOnDraft(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	err := sharedDB.ApproveMCPVersion(ctx, srvID, "1.0.0", 1, reviewer())
	if !errors.Is(err, store.ErrReviewStateMismatch) {
		t.Errorf("err = %v, want ErrReviewStateMismatch on Draft approve", err)
	}
}

func TestApproveMCPVersion_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	err := sharedDB.ApproveMCPVersion(ctx, "01J0NOPE0000000000000NOPE0", "1.0.0", 1, reviewer())
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestApproveMCPVersion_TwoReviewersRace(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	// First reviewer wins.
	if err := sharedDB.ApproveMCPVersion(ctx, srvID, "1.0.0", 1, reviewer()); err != nil {
		t.Fatalf("first approve: %v", err)
	}
	// Second reviewer tries on the same revision but the row is no longer
	// in pending_review — the diagnostic SELECT picks it up as
	// already-published per the cross-field check.
	err := sharedDB.ApproveMCPVersion(ctx, srvID, "1.0.0", 1, reviewer())
	if !errors.Is(err, store.ErrAlreadyPublished) && !errors.Is(err, store.ErrReviewStateMismatch) {
		t.Errorf("err = %v, want ErrAlreadyPublished or ErrReviewStateMismatch", err)
	}
}

// ── Reject ──────────────────────────────────────────────────────────────

func TestRejectMCPVersion_HappyPath(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}

	if err := sharedDB.RejectMCPVersion(ctx, srvID, "1.0.0", 1, "missing docs", reviewer()); err != nil {
		t.Fatalf("reject: %v", err)
	}
	state, _, reason, _ := readReviewState(t, ctx, srvID, "1.0.0")
	if state != "rejected" {
		t.Errorf("state = %q, want rejected", state)
	}
	if reason != "missing docs" {
		t.Errorf("reason = %q, want 'missing docs'", reason)
	}
}

func TestRejectMCPVersion_RevisionMismatch(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	err := sharedDB.RejectMCPVersion(ctx, srvID, "1.0.0", 99, "bad", reviewer())
	if !errors.Is(err, store.ErrReviewRevisionMismatch) {
		t.Errorf("err = %v, want ErrReviewRevisionMismatch", err)
	}
}

func TestRejectMCPVersion_StateMismatchOnDraft(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	err := sharedDB.RejectMCPVersion(ctx, srvID, "1.0.0", 1, "no", reviewer())
	if !errors.Is(err, store.ErrReviewStateMismatch) {
		t.Errorf("err = %v, want ErrReviewStateMismatch", err)
	}
}

// ── Agent version workflow ──────────────────────────────────────────────

func seedDraftAgentVersion(t *testing.T, ctx context.Context, ns, slug, ver string) string {
	t.Helper()
	pubID := insertPublisher(t, ns, ns)
	ag, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: slug, Name: slug,
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID:            ag.ID,
		Version:            ver,
		EndpointURL:        "https://agent.example/api",
		Skills:             json.RawMessage(`[{"id":"s1","name":"s","description":"d","tags":["x"]}]`),
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		ProtocolVersion:    "0.3.0",
	}); err != nil {
		t.Fatalf("CreateAgentVersion: %v", err)
	}
	return ag.ID
}

func readAgentReviewState(t *testing.T, ctx context.Context, agentID, version string) (state string, revision int, reason string, publishedAt *string) {
	t.Helper()
	var pa *string
	err := sharedDB.Pool.QueryRow(ctx, `
		SELECT review_state, revision, coalesce(rejection_reason,''),
		       to_char(published_at, 'YYYY-MM-DD"T"HH24:MI:SS')
		FROM agent_versions WHERE agent_id=$1 AND version=$2`,
		agentID, version,
	).Scan(&state, &revision, &reason, &pa)
	if err != nil {
		t.Fatalf("read agent review state: %v", err)
	}
	return state, revision, reason, pa
}

func TestSubmitAgentVersion_DraftToPending(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.1.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	state, _, _, _ := readAgentReviewState(t, ctx, agID, "0.1.0")
	if state != "pending_review" {
		t.Errorf("state = %q, want pending_review", state)
	}
}

func TestApproveAgentVersion_HappyPath(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.1.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := sharedDB.ApproveAgentVersion(ctx, agID, "0.1.0", 1, reviewer()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	state, _, _, pa := readAgentReviewState(t, ctx, agID, "0.1.0")
	if state != "none" {
		t.Errorf("state = %q, want none", state)
	}
	if pa == nil {
		t.Errorf("published_at should be set")
	}

	// Parent agent status should have flipped to 'published'.
	var st string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT status FROM agents WHERE id=$1`, agID).Scan(&st); err != nil {
		t.Fatalf("read agent status: %v", err)
	}
	if st != "published" {
		t.Errorf("agent.status = %q, want published", st)
	}
}

func TestApproveAgentVersion_RevisionMismatch(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.1.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	err := sharedDB.ApproveAgentVersion(ctx, agID, "0.1.0", 99, reviewer())
	if !errors.Is(err, store.ErrReviewRevisionMismatch) {
		t.Errorf("err = %v, want ErrReviewRevisionMismatch", err)
	}
}

func TestRejectAgentVersion_HappyPath(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.1.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := sharedDB.RejectAgentVersion(ctx, agID, "0.1.0", 1, "needs polish", reviewer()); err != nil {
		t.Fatalf("reject: %v", err)
	}
	state, _, reason, _ := readAgentReviewState(t, ctx, agID, "0.1.0")
	if state != "rejected" {
		t.Errorf("state = %q, want rejected", state)
	}
	if reason != "needs polish" {
		t.Errorf("reason = %q", reason)
	}
}

func TestWithdrawAgentVersion_PendingToDraft(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.1.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := sharedDB.WithdrawAgentVersion(ctx, agID, "0.1.0", actor()); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	state, _, _, _ := readAgentReviewState(t, ctx, agID, "0.1.0")
	if state != "none" {
		t.Errorf("state = %q, want none", state)
	}
}

// ── Deletion flow ───────────────────────────────────────────────────────

func TestRequestMCPDeletion_HappyPath(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.RequestMCPDeletion(ctx, srvID, actor()); err != nil {
		t.Fatalf("request: %v", err)
	}
	var requestedAt *string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT to_char(deletion_requested_at,'YYYY-MM-DD"T"HH24:MI:SS') FROM mcp_servers WHERE id=$1`,
		srvID,
	).Scan(&requestedAt); err != nil {
		t.Fatalf("read: %v", err)
	}
	if requestedAt == nil {
		t.Errorf("deletion_requested_at not set")
	}
}

func TestRequestMCPDeletion_AlreadyPendingConflict(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.RequestMCPDeletion(ctx, srvID, actor()); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := sharedDB.RequestMCPDeletion(ctx, srvID, actor()); !errors.Is(err, store.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict", err)
	}
}

func TestApproveMCPDeletion_SoftDeletes(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.RequestMCPDeletion(ctx, srvID, actor()); err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := sharedDB.ApproveMCPDeletion(ctx, srvID, reviewer()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var deletedAt *string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT to_char(deleted_at,'YYYY-MM-DD"T"HH24:MI:SS') FROM mcp_servers WHERE id=$1`,
		srvID,
	).Scan(&deletedAt); err != nil {
		t.Fatalf("read: %v", err)
	}
	if deletedAt == nil {
		t.Errorf("deleted_at not set after approve")
	}
}

func TestApproveMCPDeletion_NoPendingConflict(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.ApproveMCPDeletion(ctx, srvID, reviewer()); !errors.Is(err, store.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict (no pending)", err)
	}
}

func TestRejectMCPDeletion_ClearsRequest(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.RequestMCPDeletion(ctx, srvID, actor()); err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := sharedDB.RejectMCPDeletion(ctx, srvID, reviewer()); err != nil {
		t.Fatalf("reject: %v", err)
	}
	var requestedAt, deletedAt *string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT to_char(deletion_requested_at,'YYYY-MM-DD"T"HH24:MI:SS'),
		        to_char(deleted_at,'YYYY-MM-DD"T"HH24:MI:SS')
		 FROM mcp_servers WHERE id=$1`, srvID,
	).Scan(&requestedAt, &deletedAt); err != nil {
		t.Fatalf("read: %v", err)
	}
	if requestedAt != nil {
		t.Errorf("deletion_requested_at = %v, want NULL after reject", requestedAt)
	}
	if deletedAt != nil {
		t.Errorf("deleted_at = %v, want NULL after reject", deletedAt)
	}
}

func TestListReviewQueue_UnionsAllFourSources(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// MCP version: pending_review.
	srvA := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvA, "1.0.0", actor()); err != nil {
		t.Fatalf("submit mcp version: %v", err)
	}
	// MCP deletion: pending request.
	srvB := seedDraftMCPVersion(t, ctx, "globex", "barometer", "1.0.0")
	if err := sharedDB.RequestMCPDeletion(ctx, srvB, actor()); err != nil {
		t.Fatalf("request mcp deletion: %v", err)
	}
	// Agent version: pending_review.
	agA := seedDraftAgentVersion(t, ctx, "initech", "planner", "0.1.0")
	if err := sharedDB.SubmitAgentVersion(ctx, agA, "0.1.0", actor()); err != nil {
		t.Fatalf("submit agent version: %v", err)
	}
	// Agent deletion: pending request.
	agB := seedDraftAgentVersion(t, ctx, "umbrella", "sweeper", "0.1.0")
	if err := sharedDB.RequestAgentDeletion(ctx, agB, actor()); err != nil {
		t.Fatalf("request agent deletion: %v", err)
	}

	// Approved versions and rejected ones must not appear in the queue.
	srvC := seedDraftMCPVersion(t, ctx, "soylent", "ignored", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvC, "1.0.0", actor()); err != nil {
		t.Fatalf("submit ignored: %v", err)
	}
	if err := sharedDB.ApproveMCPVersion(ctx, srvC, "1.0.0", 1, reviewer()); err != nil {
		t.Fatalf("approve ignored: %v", err)
	}

	items, err := sharedDB.ListReviewQueue(ctx, store.ListReviewQueueParams{Limit: 50})
	if err != nil {
		t.Fatalf("ListReviewQueue: %v", err)
	}
	if len(items) != 4 {
		t.Errorf("expected 4 items (1 mcp ver + 1 mcp del + 1 ag ver + 1 ag del), got %d", len(items))
	}
	kinds := map[store.ReviewQueueItemKind]int{}
	for _, it := range items {
		kinds[it.Kind]++
	}
	if kinds[store.ReviewQueueItemMCPVersion] != 1 ||
		kinds[store.ReviewQueueItemAgentVersion] != 1 ||
		kinds[store.ReviewQueueItemMCPDeletion] != 1 ||
		kinds[store.ReviewQueueItemAgentDeletion] != 1 {
		t.Errorf("unexpected kind distribution: %+v", kinds)
	}
}

// Public-read leakage check: after a workflow approve-delete, the entry
// must not appear in public list endpoints. This guards against the
// classic "deleted via workflow but still visible" regression — the
// approve handler relies on `status='deleted'` doing double duty so
// every read site that already filters legacy tombstones keeps working.
func TestApproveMCPDeletion_HidesFromPublicReads(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	srvID := seedDraftMCPVersion(t, ctx, "acme", "weather", "1.0.0")
	if err := sharedDB.SubmitMCPVersion(ctx, srvID, "1.0.0", actor()); err != nil {
		t.Fatalf("submit version: %v", err)
	}
	if err := sharedDB.ApproveMCPVersion(ctx, srvID, "1.0.0", 1, reviewer()); err != nil {
		t.Fatalf("approve version: %v", err)
	}
	// Make the entry public so the default PublicOnly filter would
	// otherwise return it.
	if err := sharedDB.SetMCPServerVisibility(ctx, srvID, domain.VisibilityPublic); err != nil {
		t.Fatalf("set visibility: %v", err)
	}

	rows, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{PublicOnly: true, Limit: 50})
	if err != nil {
		t.Fatalf("list before delete: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 published+public server before delete, got %d", len(rows))
	}

	if err := sharedDB.RequestMCPDeletion(ctx, srvID, actor()); err != nil {
		t.Fatalf("request delete: %v", err)
	}
	if err := sharedDB.ApproveMCPDeletion(ctx, srvID, reviewer()); err != nil {
		t.Fatalf("approve delete: %v", err)
	}

	rows, _, err = sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{PublicOnly: true, Limit: 50})
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 servers in public list after workflow delete, got %d", len(rows))
	}
}

func TestApproveAgentDeletion_SoftDeletes(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")
	if err := sharedDB.RequestAgentDeletion(ctx, agID, actor()); err != nil {
		t.Fatalf("request: %v", err)
	}
	if err := sharedDB.ApproveAgentDeletion(ctx, agID, reviewer()); err != nil {
		t.Fatalf("approve: %v", err)
	}
	var deletedAt *string
	if err := sharedDB.Pool.QueryRow(ctx,
		`SELECT to_char(deleted_at,'YYYY-MM-DD"T"HH24:MI:SS') FROM agents WHERE id=$1`,
		agID,
	).Scan(&deletedAt); err != nil {
		t.Fatalf("read: %v", err)
	}
	if deletedAt == nil {
		t.Errorf("deleted_at not set")
	}
}

// Exercise the agent-side diagnostic functions (non-Tx variants) that
// the happy paths don't touch. Each test forces a 0-row UPDATE so the
// store falls through to the diagnose* helper.
func TestSubmitAgentVersion_DiagnoseHelpers(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")

	// Submit twice → second hits diagnoseAgentReviewMiss because the
	// row is no longer in 'none'/'rejected'.
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.1.0", actor()); err != nil {
		t.Fatalf("first submit: %v", err)
	}
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.1.0", actor()); !errors.Is(err, store.ErrReviewStateMismatch) {
		t.Errorf("err = %v, want ErrReviewStateMismatch via diagnoseAgentReviewMiss", err)
	}

	// SubmitAgentVersion with unknown id → diagnoseAgentReviewMiss returns NotFound.
	if err := sharedDB.SubmitAgentVersion(ctx, "01J0NOPE0000000000000NOPE0", "0.1.0", actor()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRejectAgentVersion_DiagnoseHelpersNonTx(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")

	// RejectAgentVersion runs without a transaction wrapper, so its
	// 0-row path goes through diagnoseAgentApproveMiss (the non-Tx
	// variant). Reject a Draft → state-mismatch.
	if err := sharedDB.RejectAgentVersion(ctx, agID, "0.1.0", 1, "no", reviewer()); !errors.Is(err, store.ErrReviewStateMismatch) {
		t.Errorf("err = %v, want ErrReviewStateMismatch via diagnoseAgentApproveMiss", err)
	}

	// Submit then reject with stale revision → revision-mismatch via
	// the same path.
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.1.0", actor()); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if err := sharedDB.RejectAgentVersion(ctx, agID, "0.1.0", 99, "stale", reviewer()); !errors.Is(err, store.ErrReviewRevisionMismatch) {
		t.Errorf("err = %v, want ErrReviewRevisionMismatch", err)
	}

	// Reject a not-found row → not-found.
	if err := sharedDB.RejectAgentVersion(ctx, "01J0NOPE0000000000000NOPE0", "0.1.0", 1, "x", reviewer()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestAgentDeletion_DiagnoseHelpers(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")

	// Approve when no request is pending → ErrConflict via
	// diagnoseAgentDeletionMiss.
	if err := sharedDB.ApproveAgentDeletion(ctx, agID, reviewer()); !errors.Is(err, store.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict (no pending)", err)
	}

	// Reject when no request is pending → same diagnostic.
	if err := sharedDB.RejectAgentDeletion(ctx, agID, reviewer()); !errors.Is(err, store.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict (reject with no pending)", err)
	}

	// Request twice → second hits diagnoseAgentDeletionMiss.
	if err := sharedDB.RequestAgentDeletion(ctx, agID, actor()); err != nil {
		t.Fatalf("first request: %v", err)
	}
	if err := sharedDB.RequestAgentDeletion(ctx, agID, actor()); !errors.Is(err, store.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict (already pending)", err)
	}

	// Operate on a non-existent agent → not-found.
	if err := sharedDB.RequestAgentDeletion(ctx, "01J0NOPE0000000000000NOPE0", actor()); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSubmitAgentVersion_StackingRejectedByPartialUniqueIndex(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	agID := seedDraftAgentVersion(t, ctx, "acme", "planner", "0.1.0")
	if _, err := sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID:            agID,
		Version:            "0.2.0",
		EndpointURL:        "https://agent.example/api",
		Skills:             json.RawMessage(`[{"id":"s1","name":"s","description":"d","tags":["x"]}]`),
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		ProtocolVersion:    "0.3.0",
	}); err != nil {
		t.Fatalf("create v0.2.0: %v", err)
	}
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.1.0", actor()); err != nil {
		t.Fatalf("submit v0.1.0: %v", err)
	}
	if err := sharedDB.SubmitAgentVersion(ctx, agID, "0.2.0", actor()); !errors.Is(err, store.ErrConflict) {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}
