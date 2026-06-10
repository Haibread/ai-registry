package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/haibread/ai-registry/internal/domain"
)

// ErrChangeNotApplicable is returned when an entry-change request is approved
// but the underlying entry is no longer in a state where the change can be
// applied (e.g. a queued deprecation whose entry was meanwhile deleted, or a
// metadata edit on a now-deleted entry). Maps to RFC 7807 type
// `urn:ai-registry:problem:change-not-applicable` (HTTP 409). The reviewer
// should reject the stale request.
var ErrChangeNotApplicable = errors.New("entry change can no longer be applied")

// querier is the subset of pgx satisfied by both *pgxpool.Pool and pgx.Tx, so
// the entry-mutation apply helpers can run either standalone (immediate path)
// or inside the approve transaction.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ── Entry-mutation apply helpers (shared by the immediate path and approve) ──
//
// Each returns the affected row count so callers can map 0 to the right
// sentinel (ErrNotFound for the immediate path, ErrChangeNotApplicable when a
// state-guarded apply misses during approval).

func applyMCPVisibility(ctx context.Context, q querier, serverID string, vis domain.Visibility) (int64, error) {
	tag, err := q.Exec(ctx,
		`UPDATE mcp_servers SET visibility=$1, updated_at=now() WHERE id=$2`,
		vis, serverID)
	return tag.RowsAffected(), err
}

func applyMCPDeprecation(ctx context.Context, q querier, serverID string) (int64, error) {
	tag, err := q.Exec(ctx,
		`UPDATE mcp_servers SET status='deprecated', updated_at=now() WHERE id=$1 AND status='published'`,
		serverID)
	return tag.RowsAffected(), err
}

func applyMCPUndeprecation(ctx context.Context, q querier, serverID string) (int64, error) {
	tag, err := q.Exec(ctx,
		`UPDATE mcp_servers SET status='published', updated_at=now() WHERE id=$1 AND status='deprecated'`,
		serverID)
	return tag.RowsAffected(), err
}

func applyMCPMetadata(ctx context.Context, q querier, serverID string, p UpdateMCPServerParams) (int64, error) {
	tag, err := q.Exec(ctx, `
		UPDATE mcp_servers
		SET name=$1, description=$2, homepage_url=$3, repo_url=$4, license=$5, updated_at=now()
		WHERE id=$6 AND status != 'deleted'`,
		p.Name, p.Description, p.HomepageURL, p.RepoURL, p.License, serverID)
	return tag.RowsAffected(), err
}

func applyAgentVisibility(ctx context.Context, q querier, agentID string, vis domain.Visibility) (int64, error) {
	tag, err := q.Exec(ctx,
		`UPDATE agents SET visibility=$1, updated_at=now() WHERE id=$2`,
		vis, agentID)
	return tag.RowsAffected(), err
}

func applyAgentDeprecation(ctx context.Context, q querier, agentID string) (int64, error) {
	tag, err := q.Exec(ctx,
		`UPDATE agents SET status='deprecated', updated_at=now() WHERE id=$1 AND status='published'`,
		agentID)
	return tag.RowsAffected(), err
}

func applyAgentUndeprecation(ctx context.Context, q querier, agentID string) (int64, error) {
	tag, err := q.Exec(ctx,
		`UPDATE agents SET status='published', updated_at=now() WHERE id=$1 AND status='deprecated'`,
		agentID)
	return tag.RowsAffected(), err
}

func applyAgentMetadata(ctx context.Context, q querier, agentID string, p UpdateAgentParams) (int64, error) {
	tag, err := q.Exec(ctx, `
		UPDATE agents
		SET name=$1, description=$2, updated_at=now()
		WHERE id=$3 AND status != 'deleted'`,
		p.Name, p.Description, agentID)
	return tag.RowsAffected(), err
}

// ── Change-request CRUD ──────────────────────────────────────────────────────

// entryChangeSelectCols is the column list shared by the single-row reads.
const entryChangeSelectCols = `id, resource_type, entry_id, action, payload, state, revision,
	submitted_at, coalesce(submitted_by, ''), coalesce(submitted_by_email, ''),
	reviewed_at, coalesce(reviewed_by, ''), coalesce(reviewed_by_email, ''),
	coalesce(decision, ''), coalesce(rejection_reason, '')`

func scanEntryChange(s interface{ Scan(...any) error }) (domain.EntryChangeRequest, error) {
	var cr domain.EntryChangeRequest
	var resourceType, action, state, decision string
	err := s.Scan(
		&cr.ID, &resourceType, &cr.EntryID, &action, &cr.Payload, &state, &cr.Revision,
		&cr.SubmittedAt, &cr.SubmittedBy, &cr.SubmittedByEmail,
		&cr.ReviewedAt, &cr.ReviewedBy, &cr.ReviewedByEmail,
		&decision, &cr.RejectionReason,
	)
	cr.ResourceType = domain.EntryResourceType(resourceType)
	cr.Action = domain.EntryChangeAction(action)
	cr.State = domain.EntryChangeState(state)
	cr.Decision = domain.ReviewDecision(decision)
	return cr, err
}

// CreateEntryChangeRequest enqueues a pending entry-level mutation. The partial
// unique index ecr_one_pending_per_entry_idx rejects a second pending change on
// the same entry with ErrConflict. Mirrors RequestMCPDeletion.
func (db *DB) CreateEntryChangeRequest(ctx context.Context, resourceType domain.EntryResourceType, entryID string, action domain.EntryChangeAction, payload json.RawMessage, a Actor) (string, error) {
	ctx, span := startSpan(ctx, "CreateEntryChangeRequest")
	defer span.End()

	id := NewULID()
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO entry_change_requests
		    (id, resource_type, entry_id, action, payload, submitted_by, submitted_by_email)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, resourceType, entryID, action, payload, a.Subject, a.Email)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return "", ErrConflict
		}
		recordErr(span, err)
		return "", fmt.Errorf("creating entry change request: %w", err)
	}
	return id, nil
}

// GetPendingEntryChange returns the single pending change for an entry, if any.
// Powers the "pending review" affordance and lets the approve/reject/withdraw
// handlers resolve the change id + revision from a path-scoped ns/slug.
func (db *DB) GetPendingEntryChange(ctx context.Context, resourceType domain.EntryResourceType, entryID string) (domain.EntryChangeRequest, bool, error) {
	ctx, span := startSpan(ctx, "GetPendingEntryChange")
	defer span.End()

	cr, err := scanEntryChange(db.Pool.QueryRow(ctx,
		`SELECT `+entryChangeSelectCols+`
		 FROM entry_change_requests
		 WHERE resource_type=$1 AND entry_id=$2 AND state='pending_review'`,
		resourceType, entryID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EntryChangeRequest{}, false, nil
	}
	if err != nil {
		recordErr(span, err)
		return domain.EntryChangeRequest{}, false, fmt.Errorf("getting pending entry change: %w", err)
	}
	return cr, true, nil
}

// WithdrawEntryChangeRequest removes a pending change (author action). The row
// is deleted so the one-pending unique index frees up; the withdrawal itself is
// captured in the audit log by the handler. Mirrors WithdrawMCPVersion.
func (db *DB) WithdrawEntryChangeRequest(ctx context.Context, id string, _ Actor) error {
	ctx, span := startSpan(ctx, "WithdrawEntryChangeRequest")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`DELETE FROM entry_change_requests WHERE id=$1 AND state='pending_review'`, id)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("withdrawing entry change: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return db.diagnoseEntryChangeMiss(ctx, id, 0)
	}
	return nil
}

// RejectEntryChangeRequest marks a pending change rejected with a reason. The
// conditional UPDATE guards on state + revision, closing the simultaneous-
// reviewer race. Mirrors RejectMCPVersion.
func (db *DB) RejectEntryChangeRequest(ctx context.Context, id string, expectedRevision int, reason string, a Actor) error {
	ctx, span := startSpan(ctx, "RejectEntryChangeRequest")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE entry_change_requests
		SET state             = 'rejected',
		    decision          = 'rejected',
		    rejection_reason  = $2,
		    reviewed_at       = now(),
		    reviewed_by       = $3,
		    reviewed_by_email = $4,
		    updated_at        = now()
		WHERE id = $1 AND state = 'pending_review' AND revision = $5`,
		id, reason, a.Subject, a.Email, expectedRevision)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("rejecting entry change: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return db.diagnoseEntryChangeMiss(ctx, id, expectedRevision)
	}
	return nil
}

// ApproveEntryChangeRequest marks a pending change approved and applies the
// underlying entry mutation in the same transaction, dispatching on
// (resource_type, action). Returns the approved request so the handler can
// audit with the concrete action + payload. Mirrors ApproveMCPVersion.
func (db *DB) ApproveEntryChangeRequest(ctx context.Context, id string, expectedRevision int, a Actor) (domain.EntryChangeRequest, error) {
	ctx, span := startSpan(ctx, "ApproveEntryChangeRequest")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return domain.EntryChangeRequest{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Lock the request row so a concurrent approve/reject can't race us.
	cr, err := scanEntryChange(tx.QueryRow(ctx,
		`SELECT `+entryChangeSelectCols+`
		 FROM entry_change_requests WHERE id=$1 FOR UPDATE`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.EntryChangeRequest{}, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return domain.EntryChangeRequest{}, fmt.Errorf("loading entry change: %w", err)
	}
	if cr.State != domain.EntryChangePending {
		return domain.EntryChangeRequest{}, ErrReviewStateMismatch
	}
	if cr.Revision != expectedRevision {
		return domain.EntryChangeRequest{}, ErrReviewRevisionMismatch
	}

	if _, err := tx.Exec(ctx, `
		UPDATE entry_change_requests
		SET state='approved', decision='approved',
		    reviewed_at=now(), reviewed_by=$2, reviewed_by_email=$3, updated_at=now()
		WHERE id=$1`, id, a.Subject, a.Email); err != nil {
		recordErr(span, err)
		return domain.EntryChangeRequest{}, fmt.Errorf("marking change approved: %w", err)
	}

	if err := applyEntryChange(ctx, tx, cr); err != nil {
		recordErr(span, err)
		return domain.EntryChangeRequest{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		recordErr(span, err)
		return domain.EntryChangeRequest{}, fmt.Errorf("commit tx: %w", err)
	}
	cr.State = domain.EntryChangeApproved
	cr.Decision = domain.ReviewDecisionApproved
	return cr, nil
}

// applyEntryChange dispatches an approved change to the matching tx-scoped apply
// helper. A state-guarded apply that touches zero rows means the entry drifted
// out of an applicable state since submit -> ErrChangeNotApplicable.
func applyEntryChange(ctx context.Context, tx querier, cr domain.EntryChangeRequest) error {
	switch cr.ResourceType {
	case domain.EntryResourceMCPServer:
		switch cr.Action {
		case domain.EntryChangeVisibility:
			var p struct {
				Visibility string `json:"visibility"`
			}
			if err := json.Unmarshal(cr.Payload, &p); err != nil {
				return fmt.Errorf("decoding visibility payload: %w", err)
			}
			rows, err := applyMCPVisibility(ctx, tx, cr.EntryID, domain.Visibility(p.Visibility))
			return applyResult(rows, err)
		case domain.EntryChangeDeprecation:
			rows, err := applyMCPDeprecation(ctx, tx, cr.EntryID)
			return applyResult(rows, err)
		case domain.EntryChangeUndeprecation:
			rows, err := applyMCPUndeprecation(ctx, tx, cr.EntryID)
			return applyResult(rows, err)
		case domain.EntryChangeMetadataEdit:
			var p UpdateMCPServerParams
			if err := json.Unmarshal(cr.Payload, &p); err != nil {
				return fmt.Errorf("decoding metadata payload: %w", err)
			}
			rows, err := applyMCPMetadata(ctx, tx, cr.EntryID, p)
			return applyResult(rows, err)
		}
	case domain.EntryResourceAgent:
		switch cr.Action {
		case domain.EntryChangeVisibility:
			var p struct {
				Visibility string `json:"visibility"`
			}
			if err := json.Unmarshal(cr.Payload, &p); err != nil {
				return fmt.Errorf("decoding visibility payload: %w", err)
			}
			rows, err := applyAgentVisibility(ctx, tx, cr.EntryID, domain.Visibility(p.Visibility))
			return applyResult(rows, err)
		case domain.EntryChangeDeprecation:
			rows, err := applyAgentDeprecation(ctx, tx, cr.EntryID)
			return applyResult(rows, err)
		case domain.EntryChangeUndeprecation:
			rows, err := applyAgentUndeprecation(ctx, tx, cr.EntryID)
			return applyResult(rows, err)
		case domain.EntryChangeMetadataEdit:
			var p UpdateAgentParams
			if err := json.Unmarshal(cr.Payload, &p); err != nil {
				return fmt.Errorf("decoding metadata payload: %w", err)
			}
			rows, err := applyAgentMetadata(ctx, tx, cr.EntryID, p)
			return applyResult(rows, err)
		}
	}
	return fmt.Errorf("unknown change (%s, %s)", cr.ResourceType, cr.Action)
}

func applyResult(rows int64, err error) error {
	if err != nil {
		return fmt.Errorf("applying entry change: %w", err)
	}
	if rows == 0 {
		return ErrChangeNotApplicable
	}
	return nil
}

// diagnoseEntryChangeMiss turns a 0-row withdraw/reject into a sentinel: gone ->
// ErrNotFound, wrong state -> ErrReviewStateMismatch, stale revision ->
// ErrReviewRevisionMismatch. expectedRevision == 0 skips the revision check
// (withdraw doesn't carry one).
func (db *DB) diagnoseEntryChangeMiss(ctx context.Context, id string, expectedRevision int) error {
	var state string
	var revision int
	err := db.Pool.QueryRow(ctx,
		`SELECT state, revision FROM entry_change_requests WHERE id=$1`, id).Scan(&state, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("diagnosing entry change miss: %w", err)
	}
	if state != string(domain.EntryChangePending) {
		return ErrReviewStateMismatch
	}
	if expectedRevision != 0 && revision != expectedRevision {
		return ErrReviewRevisionMismatch
	}
	return ErrReviewStateMismatch
}
