package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// Actor identifies who performed a review action. The pair is
// denormalised into the version row's audit columns at action time so
// later Keycloak email changes do not rewrite history.
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
// (review_state='none'), clearing the submitted-* audit columns.
// Revision is intentionally left in place — it monotonically grows
// across the version's whole lifetime.
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

// RejectMCPVersion transitions a PendingReview version to Rejected
// with the supplied reason. The revision check is the same as approve
// — both share the conditional-UPDATE pattern that closes the
// simultaneous-reviewer race.
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

// ── Deletion-request flow ────────────────────────────────────────────────

// RequestMCPDeletion marks an MCP server as having a pending deletion
// review. Returns ErrConflict if a deletion is already pending or the
// entry is already soft-deleted; ErrNotFound if the row doesn't exist.
// The entry stays visible on public reads (the current published
// version is unchanged) until the deletion is approved.
func (db *DB) RequestMCPDeletion(ctx context.Context, serverID string, a Actor) error {
	ctx, span := startSpan(ctx, "RequestMCPDeletion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE mcp_servers
		SET deletion_requested_at       = NOW(),
		    deletion_requested_by       = $2,
		    deletion_requested_by_email = $3,
		    updated_at                  = NOW()
		WHERE id = $1
		  AND deletion_requested_at IS NULL
		  AND deleted_at             IS NULL`,
		serverID, a.Subject, a.Email)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("requesting mcp deletion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseMCPDeletionMiss(ctx, db, serverID)
	}
	return nil
}

// ApproveMCPDeletion confirms a pending deletion: the entry's
// deleted_at is set, and `status` is also flipped to 'deleted' so the
// existing read-path filters (which all check `status != 'deleted'`)
// exclude the row without further changes. The deletion_requested_*
// columns are intentionally retained as audit.
func (db *DB) ApproveMCPDeletion(ctx context.Context, serverID string, _ Actor) error {
	ctx, span := startSpan(ctx, "ApproveMCPDeletion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE mcp_servers
		SET deleted_at = NOW(),
		    status     = 'deleted',
		    updated_at = NOW()
		WHERE id = $1
		  AND deletion_requested_at IS NOT NULL
		  AND deleted_at             IS NULL`, serverID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("approving mcp deletion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseMCPDeletionMiss(ctx, db, serverID)
	}
	return nil
}

// RejectMCPDeletion clears a pending deletion request. The reason
// (passed by the handler) lives in the audit log; the entry row drops
// the request markers and stays visible.
func (db *DB) RejectMCPDeletion(ctx context.Context, serverID string, _ Actor) error {
	ctx, span := startSpan(ctx, "RejectMCPDeletion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE mcp_servers
		SET deletion_requested_at       = NULL,
		    deletion_requested_by       = NULL,
		    deletion_requested_by_email = NULL,
		    updated_at                  = NOW()
		WHERE id = $1
		  AND deletion_requested_at IS NOT NULL
		  AND deleted_at             IS NULL`, serverID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("rejecting mcp deletion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseMCPDeletionMiss(ctx, db, serverID)
	}
	return nil
}

func diagnoseMCPDeletionMiss(ctx context.Context, db *DB, serverID string) error {
	var (
		requestedAt *time.Time //nolint:wastedassign // scanned for symmetry; not consulted
		deletedAt   *time.Time
	)
	_ = requestedAt
	err := db.Pool.QueryRow(ctx,
		`SELECT deletion_requested_at, deleted_at FROM mcp_servers WHERE id=$1`,
		serverID).Scan(&requestedAt, &deletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnosing deletion miss: %w", err)
	}
	// already-deleted is functionally not-found from a workflow standpoint.
	if deletedAt != nil {
		return ErrNotFound
	}
	// Either there's no pending request when one was expected, or there's
	// a pending request when none was expected — both surface as conflict.
	return ErrConflict
}

// RequestAgentDeletion is the agent equivalent of RequestMCPDeletion.
func (db *DB) RequestAgentDeletion(ctx context.Context, agentID string, a Actor) error {
	ctx, span := startSpan(ctx, "RequestAgentDeletion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agents
		SET deletion_requested_at       = NOW(),
		    deletion_requested_by       = $2,
		    deletion_requested_by_email = $3,
		    updated_at                  = NOW()
		WHERE id = $1
		  AND deletion_requested_at IS NULL
		  AND deleted_at             IS NULL`,
		agentID, a.Subject, a.Email)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("requesting agent deletion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseAgentDeletionMiss(ctx, db, agentID)
	}
	return nil
}

// ApproveAgentDeletion is the agent equivalent of ApproveMCPDeletion.
// Sets both deleted_at and status='deleted' so existing read filters
// keep working without any change.
func (db *DB) ApproveAgentDeletion(ctx context.Context, agentID string, _ Actor) error {
	ctx, span := startSpan(ctx, "ApproveAgentDeletion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agents
		SET deleted_at = NOW(),
		    status     = 'deleted',
		    updated_at = NOW()
		WHERE id = $1
		  AND deletion_requested_at IS NOT NULL
		  AND deleted_at             IS NULL`, agentID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("approving agent deletion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseAgentDeletionMiss(ctx, db, agentID)
	}
	return nil
}

// RejectAgentDeletion is the agent equivalent of RejectMCPDeletion.
func (db *DB) RejectAgentDeletion(ctx context.Context, agentID string, _ Actor) error {
	ctx, span := startSpan(ctx, "RejectAgentDeletion")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agents
		SET deletion_requested_at       = NULL,
		    deletion_requested_by       = NULL,
		    deletion_requested_by_email = NULL,
		    updated_at                  = NOW()
		WHERE id = $1
		  AND deletion_requested_at IS NOT NULL
		  AND deleted_at             IS NULL`, agentID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("rejecting agent deletion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return diagnoseAgentDeletionMiss(ctx, db, agentID)
	}
	return nil
}

func diagnoseAgentDeletionMiss(ctx context.Context, db *DB, agentID string) error {
	var (
		requestedAt *time.Time //nolint:wastedassign // scanned for symmetry; not consulted
		deletedAt   *time.Time
	)
	_ = requestedAt
	err := db.Pool.QueryRow(ctx,
		`SELECT deletion_requested_at, deleted_at FROM agents WHERE id=$1`,
		agentID).Scan(&requestedAt, &deletedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnosing deletion miss: %w", err)
	}
	if deletedAt != nil {
		return ErrNotFound
	}
	return ErrConflict
}

// ── Review queue ─────────────────────────────────────────────────────────

// ReviewQueueItemKind discriminates the kinds of items that show up in
// the reviewer's queue.
type ReviewQueueItemKind string

const (
	ReviewQueueItemMCPVersion    ReviewQueueItemKind = "mcp_version"
	ReviewQueueItemAgentVersion  ReviewQueueItemKind = "agent_version"
	ReviewQueueItemMCPDeletion   ReviewQueueItemKind = "mcp_deletion"
	ReviewQueueItemAgentDeletion ReviewQueueItemKind = "agent_deletion"
)

// ReviewQueueItem is a flattened entry in the reviewer's queue. Version
// items have non-empty Version and a Revision >= 1; deletion items have
// empty Version and Revision == 0.
type ReviewQueueItem struct {
	Kind             ReviewQueueItemKind
	PublisherSlug    string
	EntrySlug        string
	EntryID          string
	Version          string
	Revision         int
	SubmittedAt      time.Time
	SubmittedBy      string
	SubmittedByEmail string
}

// ListReviewQueueParams paginates the queue. The cursor is the
// (submitted_at, entry_id, version) triple of the last item from the
// previous page. Cursor uses the same compact encoding as other list
// queries.
type ListReviewQueueParams struct {
	Limit  int32
	Cursor string
	// PublisherIDs, when non-empty, restricts the queue to items owned by these
	// publishers — the reviewer-scoping filter (ADR 0006): a per-publisher
	// Reviewer sees only the publishers they review. Empty means no filter; the
	// handler passes nil for Server Admins and global-Reviewer-grant holders
	// (see-all) and short-circuits to an empty result for a caller who reviews
	// no publisher, so an empty slice here never silently widens the queue.
	PublisherIDs []string
}

// ListReviewQueue returns pending_review versions and pending deletions,
// newest first. The query unions the four sources (MCP version, agent version,
// MCP deletion, agent deletion) and sorts by the submission / request
// timestamp. When p.PublisherIDs is non-empty the result is restricted to those
// owning publishers (reviewer scoping, ADR 0006); empty means no filter.
//
// Reviewers and admins are the only legitimate callers; the handler resolves
// the caller's reviewer scope (Server Admin / global Reviewer → all publishers;
// a per-publisher Reviewer → their publishers) before calling this.
func (db *DB) ListReviewQueue(ctx context.Context, p ListReviewQueueParams) ([]ReviewQueueItem, error) {
	ctx, span := startSpan(ctx, "ListReviewQueue")
	defer span.End()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	args := []any{}
	argN := 1
	var conds []string
	if p.Cursor != "" {
		at, id, err := decodeCursor(p.Cursor)
		if err == nil {
			conds = append(conds, fmt.Sprintf("(submitted_at, entry_id) < ($%d, $%d)", argN, argN+1))
			args = append(args, at, id)
			argN += 2
		}
	}
	if len(p.PublisherIDs) > 0 {
		conds = append(conds, fmt.Sprintf("publisher_id = ANY($%d)", argN))
		args = append(args, p.PublisherIDs)
		argN++
	}
	whereClause := ""
	if len(conds) > 0 {
		whereClause = " WHERE " + strings.Join(conds, " AND ")
	}

	args = append(args, p.Limit)
	q := fmt.Sprintf(`
		WITH q AS (
		    SELECT 'mcp_version'::text   AS kind,
		           p.id                  AS publisher_id,
		           p.slug                AS publisher_slug,
		           s.slug                AS entry_slug,
		           s.id                  AS entry_id,
		           v.version             AS version,
		           v.revision            AS revision,
		           v.submitted_at        AS submitted_at,
		           coalesce(v.submitted_by, '')       AS submitted_by,
		           coalesce(v.submitted_by_email, '') AS submitted_by_email
		    FROM mcp_server_versions v
		    JOIN mcp_servers s ON s.id = v.server_id
		    JOIN publishers p ON p.id = s.publisher_id
		    WHERE v.review_state = 'pending_review' AND v.submitted_at IS NOT NULL

		    UNION ALL
		    SELECT 'agent_version'::text, p.id, p.slug, a.slug, a.id,
		           av.version, av.revision, av.submitted_at,
		           coalesce(av.submitted_by, ''), coalesce(av.submitted_by_email, '')
		    FROM agent_versions av
		    JOIN agents     a ON a.id = av.agent_id
		    JOIN publishers p ON p.id = a.publisher_id
		    WHERE av.review_state = 'pending_review' AND av.submitted_at IS NOT NULL

		    UNION ALL
		    SELECT 'mcp_deletion'::text, p.id, p.slug, s.slug, s.id,
		           '', 0, s.deletion_requested_at,
		           coalesce(s.deletion_requested_by, ''), coalesce(s.deletion_requested_by_email, '')
		    FROM mcp_servers s
		    JOIN publishers p ON p.id = s.publisher_id
		    WHERE s.deletion_requested_at IS NOT NULL AND s.deleted_at IS NULL

		    UNION ALL
		    SELECT 'agent_deletion'::text, p.id, p.slug, a.slug, a.id,
		           '', 0, a.deletion_requested_at,
		           coalesce(a.deletion_requested_by, ''), coalesce(a.deletion_requested_by_email, '')
		    FROM agents a
		    JOIN publishers p ON p.id = a.publisher_id
		    WHERE a.deletion_requested_at IS NOT NULL AND a.deleted_at IS NULL
		)
		SELECT kind, publisher_slug, entry_slug, entry_id, version, revision,
		       submitted_at, submitted_by, submitted_by_email
		FROM q
		%s
		ORDER BY submitted_at DESC, entry_id DESC
		LIMIT $%d`, whereClause, argN)

	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing review queue: %w", err)
	}
	defer rows.Close()

	var result []ReviewQueueItem
	for rows.Next() {
		var it ReviewQueueItem
		var kind string
		if err := rows.Scan(
			&kind, &it.PublisherSlug, &it.EntrySlug, &it.EntryID,
			&it.Version, &it.Revision, &it.SubmittedAt,
			&it.SubmittedBy, &it.SubmittedByEmail,
		); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning review queue row: %w", err)
		}
		it.Kind = ReviewQueueItemKind(kind)
		result = append(result, it)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
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
