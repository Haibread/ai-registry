package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/haibread/ai-registry/internal/domain"
)

// ErrReviewStateMismatch is returned when an approve / reject targets a
// version that is no longer in pending_review (already approved by another
// reviewer, withdrawn, or never submitted). Maps to RFC 7807 type
// `urn:ai-registry:problem:review-state-mismatch` (HTTP 409).
var ErrReviewStateMismatch = errors.New("version is not in pending_review")

// ErrReviewRevisionMismatch is returned when an approve / reject is
// submitted with a `revision` that no longer matches the row — the
// publisher edited the version since the reviewer last loaded it. Maps
// to `urn:ai-registry:problem:review-revision-mismatch` (HTTP 409).
var ErrReviewRevisionMismatch = errors.New("version revision was bumped since the reviewer last loaded it")

// ErrAlreadyPublished is returned when a workflow operation targets a
// version whose `published_at` is non-NULL (versions are immutable post-
// publish). Distinct from ErrImmutable so callers can map it to a
// specific 409 type if desired.
var ErrAlreadyPublished = errors.New("version is already published")

// Actor identifies who performed a review action. The pair is denormalised
// into the version row's audit columns at action time, so later Keycloak
// email changes do not rewrite history (ADR 0003).
type Actor struct {
	Subject string
	Email   string
}

// ── MCP server versions: workflow ────────────────────────────────────────

// SubmitMCPVersion transitions a Draft or Rejected version to
// PendingReview. The partial unique index `mcp_server_versions_one_pending_idx`
// rejects the submission with ErrConflict if another version on the same
// server is already in PendingReview.
func (db *DB) SubmitMCPVersion(ctx context.Context, serverID, version string, a Actor) error {
	ctx, span := startSpan(ctx, "SubmitMCPVersion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE mcp_server_versions
		SET review_state       = 'pending_review',
		    submitted_at       = NOW(),
		    submitted_by       = $3,
		    submitted_by_email = $4,
		    rejection_reason   = NULL,
		    updated_at         = NOW()
		WHERE server_id = $1
		  AND version   = $2
		  AND review_state IN ('none', 'rejected')
		  AND published_at IS NULL`,
		serverID, version, a.Subject, a.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return ErrConflict
		}
		recordErr(span, err)
		return fmt.Errorf("submitting version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseMCPReviewMiss(ctx, db, serverID, version, span)
	}
	return nil
}

// WithdrawMCPVersion transitions a PendingReview version back to Draft
// (review_state='none'), clearing the submitted-* audit columns. Revision
// is intentionally left in place — it monotonically grows across the
// version's whole lifetime per ADR 0003.
func (db *DB) WithdrawMCPVersion(ctx context.Context, serverID, version string, _ Actor) error {
	ctx, span := startSpan(ctx, "WithdrawMCPVersion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE mcp_server_versions
		SET review_state       = 'none',
		    submitted_at       = NULL,
		    submitted_by       = NULL,
		    submitted_by_email = NULL,
		    updated_at         = NOW()
		WHERE server_id = $1
		  AND version   = $2
		  AND review_state = 'pending_review'`,
		serverID, version)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("withdrawing version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseMCPReviewMiss(ctx, db, serverID, version, span)
	}
	return nil
}

// ApproveMCPVersion transitions a PendingReview version to published:
// review_state='none', published_at=NOW() (if not already set), and the
// parent server's status flips to 'published' when it was 'draft'. The
// revision check closes the two-reviewers-approving-simultaneously race —
// the conditional UPDATE matches at most one row.
func (db *DB) ApproveMCPVersion(ctx context.Context, serverID, version string, expectedRevision int, a Actor) error {
	ctx, span := startSpan(ctx, "ApproveMCPVersion")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx, `
		UPDATE mcp_server_versions
		SET review_state      = 'none',
		    reviewed_at       = NOW(),
		    reviewed_by       = $3,
		    reviewed_by_email = $4,
		    review_decision   = 'approved',
		    published_at      = COALESCE(published_at, NOW()),
		    updated_at        = NOW()
		WHERE server_id    = $1
		  AND version      = $2
		  AND review_state = 'pending_review'
		  AND revision     = $5`,
		serverID, version, a.Subject, a.Email, expectedRevision)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("approving version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Fall back to a discriminating SELECT so the caller can map the
		// failure to the correct 409 type (or 404).
		if err := diagnoseMCPApproveMissTx(ctx, tx, serverID, version, expectedRevision); err != nil {
			recordErr(span, err)
			return err
		}
		// Defensive: should not reach here.
		return ErrReviewStateMismatch
	}

	// Promote the parent entry from draft → published once the first
	// version lands. Idempotent for non-first publishes.
	if _, err := tx.Exec(ctx,
		`UPDATE mcp_servers SET status='published', updated_at=NOW() WHERE id=$1 AND status='draft'`,
		serverID); err != nil {
		recordErr(span, err)
		return fmt.Errorf("promoting server status: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		recordErr(span, err)
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// RejectMCPVersion transitions a PendingReview version to Rejected with
// the supplied reason. The revision check is the same as approve — both
// share the conditional-UPDATE pattern that closes the simultaneous-
// reviewer race (ADR 0003).
func (db *DB) RejectMCPVersion(ctx context.Context, serverID, version string, expectedRevision int, reason string, a Actor) error {
	ctx, span := startSpan(ctx, "RejectMCPVersion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE mcp_server_versions
		SET review_state      = 'rejected',
		    reviewed_at       = NOW(),
		    reviewed_by       = $3,
		    reviewed_by_email = $4,
		    review_decision   = 'rejected',
		    rejection_reason  = $5,
		    updated_at        = NOW()
		WHERE server_id    = $1
		  AND version      = $2
		  AND review_state = 'pending_review'
		  AND revision     = $6`,
		serverID, version, a.Subject, a.Email, reason, expectedRevision)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("rejecting version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseMCPApproveMiss(ctx, db, serverID, version, expectedRevision, span)
	}
	return nil
}

// ── Diagnostics: turn a 0-row UPDATE into a meaningful sentinel ──────────

// diagnoseMCPReviewMiss is the SELECT-discriminator for submit/withdraw —
// either of which fails when the row is absent or in the wrong state. The
// revision is not relevant for these transitions.
func diagnoseMCPReviewMiss(ctx context.Context, db *DB, serverID, version string, span any) error {
	var (
		state       string
		publishedAt *time.Time
	)
	err := db.Pool.QueryRow(ctx,
		`SELECT review_state, published_at FROM mcp_server_versions
		 WHERE server_id = $1 AND version = $2`,
		serverID, version).Scan(&state, &publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnosing review miss: %w", err)
	}
	if publishedAt != nil {
		return ErrAlreadyPublished
	}
	return ErrReviewStateMismatch
}

// diagnoseMCPApproveMiss covers the approve / reject path, where revision
// matters too. Returns ErrNotFound | ErrReviewStateMismatch |
// ErrReviewRevisionMismatch | ErrAlreadyPublished.
func diagnoseMCPApproveMiss(ctx context.Context, db *DB, serverID, version string, expectedRevision int, _ any) error {
	var (
		state       string
		revision    int
		publishedAt *time.Time
	)
	err := db.Pool.QueryRow(ctx,
		`SELECT review_state, revision, published_at FROM mcp_server_versions
		 WHERE server_id = $1 AND version = $2`,
		serverID, version).Scan(&state, &revision, &publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnosing approve miss: %w", err)
	}
	if publishedAt != nil {
		return ErrAlreadyPublished
	}
	if state != string(domain.ReviewStatePendingReview) {
		return ErrReviewStateMismatch
	}
	if revision != expectedRevision {
		return ErrReviewRevisionMismatch
	}
	// The conditional UPDATE matched zero rows but no condition disagrees:
	// shouldn't happen but be explicit.
	return ErrReviewStateMismatch
}

// ── Agent versions: workflow ─────────────────────────────────────────────

// SubmitAgentVersion is the agent equivalent of SubmitMCPVersion.
func (db *DB) SubmitAgentVersion(ctx context.Context, agentID, version string, a Actor) error {
	ctx, span := startSpan(ctx, "SubmitAgentVersion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agent_versions
		SET review_state       = 'pending_review',
		    submitted_at       = NOW(),
		    submitted_by       = $3,
		    submitted_by_email = $4,
		    rejection_reason   = NULL,
		    updated_at         = NOW()
		WHERE agent_id = $1
		  AND version  = $2
		  AND review_state IN ('none', 'rejected')
		  AND published_at IS NULL`,
		agentID, version, a.Subject, a.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return ErrConflict
		}
		recordErr(span, err)
		return fmt.Errorf("submitting agent version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseAgentReviewMiss(ctx, db, agentID, version)
	}
	return nil
}

// WithdrawAgentVersion is the agent equivalent of WithdrawMCPVersion.
func (db *DB) WithdrawAgentVersion(ctx context.Context, agentID, version string, _ Actor) error {
	ctx, span := startSpan(ctx, "WithdrawAgentVersion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agent_versions
		SET review_state       = 'none',
		    submitted_at       = NULL,
		    submitted_by       = NULL,
		    submitted_by_email = NULL,
		    updated_at         = NOW()
		WHERE agent_id = $1
		  AND version  = $2
		  AND review_state = 'pending_review'`,
		agentID, version)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("withdrawing agent version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseAgentReviewMiss(ctx, db, agentID, version)
	}
	return nil
}

// ApproveAgentVersion is the agent equivalent of ApproveMCPVersion.
func (db *DB) ApproveAgentVersion(ctx context.Context, agentID, version string, expectedRevision int, a Actor) error {
	ctx, span := startSpan(ctx, "ApproveAgentVersion")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	tag, err := tx.Exec(ctx, `
		UPDATE agent_versions
		SET review_state      = 'none',
		    reviewed_at       = NOW(),
		    reviewed_by       = $3,
		    reviewed_by_email = $4,
		    review_decision   = 'approved',
		    published_at      = COALESCE(published_at, NOW()),
		    updated_at        = NOW()
		WHERE agent_id     = $1
		  AND version      = $2
		  AND review_state = 'pending_review'
		  AND revision     = $5`,
		agentID, version, a.Subject, a.Email, expectedRevision)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("approving agent version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if err := diagnoseAgentApproveMissTx(ctx, tx, agentID, version, expectedRevision); err != nil {
			recordErr(span, err)
			return err
		}
		return ErrReviewStateMismatch
	}
	if _, err := tx.Exec(ctx,
		`UPDATE agents SET status='published', updated_at=NOW() WHERE id=$1 AND status='draft'`,
		agentID); err != nil {
		recordErr(span, err)
		return fmt.Errorf("promoting agent status: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		recordErr(span, err)
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// RejectAgentVersion is the agent equivalent of RejectMCPVersion.
func (db *DB) RejectAgentVersion(ctx context.Context, agentID, version string, expectedRevision int, reason string, a Actor) error {
	ctx, span := startSpan(ctx, "RejectAgentVersion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agent_versions
		SET review_state      = 'rejected',
		    reviewed_at       = NOW(),
		    reviewed_by       = $3,
		    reviewed_by_email = $4,
		    review_decision   = 'rejected',
		    rejection_reason  = $5,
		    updated_at        = NOW()
		WHERE agent_id     = $1
		  AND version      = $2
		  AND review_state = 'pending_review'
		  AND revision     = $6`,
		agentID, version, a.Subject, a.Email, reason, expectedRevision)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("rejecting agent version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseAgentApproveMiss(ctx, db, agentID, version, expectedRevision)
	}
	return nil
}

func diagnoseAgentReviewMiss(ctx context.Context, db *DB, agentID, version string) error {
	var (
		state       string
		publishedAt *time.Time
	)
	err := db.Pool.QueryRow(ctx,
		`SELECT review_state, published_at FROM agent_versions
		 WHERE agent_id = $1 AND version = $2`,
		agentID, version).Scan(&state, &publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnosing agent review miss: %w", err)
	}
	if publishedAt != nil {
		return ErrAlreadyPublished
	}
	return ErrReviewStateMismatch
}

func diagnoseAgentApproveMiss(ctx context.Context, db *DB, agentID, version string, expectedRevision int) error {
	var (
		state       string
		revision    int
		publishedAt *time.Time
	)
	err := db.Pool.QueryRow(ctx,
		`SELECT review_state, revision, published_at FROM agent_versions
		 WHERE agent_id = $1 AND version = $2`,
		agentID, version).Scan(&state, &revision, &publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnosing agent approve miss: %w", err)
	}
	if publishedAt != nil {
		return ErrAlreadyPublished
	}
	if state != string(domain.ReviewStatePendingReview) {
		return ErrReviewStateMismatch
	}
	if revision != expectedRevision {
		return ErrReviewRevisionMismatch
	}
	return ErrReviewStateMismatch
}

func diagnoseAgentApproveMissTx(ctx context.Context, tx pgx.Tx, agentID, version string, expectedRevision int) error {
	var (
		state       string
		revision    int
		publishedAt *time.Time
	)
	err := tx.QueryRow(ctx,
		`SELECT review_state, revision, published_at FROM agent_versions
		 WHERE agent_id = $1 AND version = $2`,
		agentID, version).Scan(&state, &revision, &publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnosing agent approve miss: %w", err)
	}
	if publishedAt != nil {
		return ErrAlreadyPublished
	}
	if state != string(domain.ReviewStatePendingReview) {
		return ErrReviewStateMismatch
	}
	if revision != expectedRevision {
		return ErrReviewRevisionMismatch
	}
	return ErrReviewStateMismatch
}

// diagnoseMCPApproveMissTx is the in-transaction variant used by
// ApproveMCPVersion (the read must see the same snapshot as the failed
// UPDATE).
func diagnoseMCPApproveMissTx(ctx context.Context, tx pgx.Tx, serverID, version string, expectedRevision int) error {
	var (
		state       string
		revision    int
		publishedAt *time.Time
	)
	err := tx.QueryRow(ctx,
		`SELECT review_state, revision, published_at FROM mcp_server_versions
		 WHERE server_id = $1 AND version = $2`,
		serverID, version).Scan(&state, &revision, &publishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnosing approve miss: %w", err)
	}
	if publishedAt != nil {
		return ErrAlreadyPublished
	}
	if state != string(domain.ReviewStatePendingReview) {
		return ErrReviewStateMismatch
	}
	if revision != expectedRevision {
		return ErrReviewRevisionMismatch
	}
	return ErrReviewStateMismatch
}
