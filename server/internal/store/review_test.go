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
