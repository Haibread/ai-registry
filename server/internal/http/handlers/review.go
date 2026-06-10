// Package handlers — change-approval workflow endpoints.
//
// All seven workflow operations are wired up per resource type:
//
//   submit / withdraw                  -> RequirePublisherRole (Editor)
//   approve / reject                   -> RequirePublisherRole (Reviewer)
//   deletion-request                   -> RequirePublisherRole (Editor)
//   deletion-request/approve / reject  -> RequirePublisherRole (Reviewer)
//
// All non-OK responses use RFC 7807 problem+json with a slug that maps to
// the type URI documented in the OpenAPI spec.

package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// ReviewHandlers serves the change-approval workflow endpoints. The
// handlers depend on the same store + audit logger as the existing MCP
// and agent handlers.
type ReviewHandlers struct {
	db    *store.DB
	audit store.AuditLogger
}

// NewReviewHandlers builds a ReviewHandlers wired to the given DB and
// audit logger.
func NewReviewHandlers(db *store.DB, audit store.AuditLogger) *ReviewHandlers {
	return &ReviewHandlers{db: db, audit: audit}
}

// reviewActor turns the request context into a store.Actor by reading
// the JWT subject and email through the existing auditActor helper.
// The pair is denormalised into the version's audit columns at action
// time so later Keycloak email changes do not rewrite history.
func reviewActor(r *http.Request) store.Actor {
	subject, email := auditActor(r.Context())
	return store.Actor{Subject: subject, Email: email}
}

// writeReviewProblem maps a workflow store error to a discriminated 409
// (or 404) response. The slug values match the type-URI suffixes
// documented in the OpenAPI spec under
// `urn:ai-registry:problem:review-...` semantics.
func writeReviewProblem(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, store.ErrNotFound):
		problem.Write(w, http.StatusNotFound, "not-found",
			"the targeted version or entry does not exist", r.URL.Path)
	case errors.Is(err, store.ErrReviewStateMismatch):
		problem.Write(w, http.StatusConflict, "review-state-mismatch",
			"the version is no longer in pending_review (already approved, rejected, or withdrawn)", r.URL.Path)
	case errors.Is(err, store.ErrReviewRevisionMismatch):
		problem.Write(w, http.StatusConflict, "review-revision-mismatch",
			"the version was edited since you loaded it; reload and review again", r.URL.Path)
	case errors.Is(err, store.ErrAlreadyPublished):
		problem.Write(w, http.StatusConflict, "already-published",
			"the version is already published and cannot be re-reviewed", r.URL.Path)
	case errors.Is(err, store.ErrChangeNotApplicable):
		problem.Write(w, http.StatusConflict, "change-not-applicable",
			"the entry is no longer in a state where this change can be applied (e.g. already deprecated or deleted); reject this request", r.URL.Path)
	case errors.Is(err, store.ErrConflict):
		problem.Write(w, http.StatusConflict, "review-already-pending",
			"another version on this entry is already in pending_review", r.URL.Path)
	default:
		return false
	}
	return true
}

// ── Review queue ────────────────────────────────────────────────────────

// reviewScopeStore is the store slice reviewerScope needs: the caller's role
// grants, used to find the publishers they review. *store.DB satisfies it.
type reviewScopeStore interface {
	ListGrantsForPrincipal(ctx context.Context, userID string, claimGroupSlugs []string) ([]store.PrincipalGrant, error)
}

// reviewerScope resolves which publishers' pending items the caller may see in
// the review queue. seeAll is true for a Server Admin or a holder of
// a global Reviewer grant (the seeded reviewer group carries one) — they see
// every publisher's queue. Otherwise publisherIDs lists the publishers on which
// the caller holds a Reviewer grant; an empty list means the caller reviews
// nothing (the handler 403s). authed is false when the request carries no
// identity (the handler 401s). Only the Reviewer role counts: an Editor or a
// publisher Admin is not an approver, so neither ever sees the queue.
func reviewerScope(ctx context.Context, st reviewScopeStore) (publisherIDs []string, seeAll, authed bool, err error) {
	if auth.IsServerAdminFromContext(ctx) {
		return nil, true, true, nil
	}
	userID, groups, ok := auth.IdentityFromContext(ctx)
	if !ok {
		return nil, false, false, nil
	}
	grants, err := st.ListGrantsForPrincipal(ctx, userID, groups)
	if err != nil {
		return nil, false, true, err
	}
	for _, g := range grants {
		if g.Role != domain.RoleReviewer {
			continue
		}
		if g.PublisherID == "" { // a global Reviewer grant covers every publisher
			seeAll = true
		} else {
			publisherIDs = append(publisherIDs, g.PublisherID)
		}
	}
	return publisherIDs, seeAll, true, nil
}

// ListReviewQueue: GET /api/v1/review-queue
//
// Lists pending_review versions and pending deletions, newest first; each entry
// carries a `kind` discriminator so the UI can render the right row type. The
// route is authenticated-only — authorization and per-publisher scoping happen
// here: a Server Admin or global Reviewer sees every publisher's
// queue, a per-publisher Reviewer sees only the publishers they review, and a
// caller who reviews nothing gets 403.
func (h *ReviewHandlers) ListReviewQueue(w http.ResponseWriter, r *http.Request) {
	publisherIDs, seeAll, authed, err := reviewerScope(r.Context(), h.db)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if !authed {
		problem.Write(w, http.StatusUnauthorized, "unauthorized",
			"Missing or invalid bearer token", r.URL.Path)
		return
	}
	if !seeAll && len(publisherIDs) == 0 {
		problem.Write(w, http.StatusForbidden, "forbidden",
			"Reviewing requires the Reviewer role on at least one publisher.", r.URL.Path)
		return
	}

	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	params := store.ListReviewQueueParams{
		Limit:  limit + 1,
		Cursor: r.URL.Query().Get("cursor"),
	}
	// Leave PublisherIDs nil for see-all callers (no filter). A scoped reviewer
	// passes their publishers; the len==0 case was already rejected above.
	if !seeAll {
		params.PublisherIDs = publisherIDs
	}
	rows, err := h.db.ListReviewQueue(r.Context(), params)
	if err != nil {
		internalError(w, r, err)
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nextCursor = store.EncodeCursor(last.SubmittedAt, last.EntryID)
	}

	items := make([]map[string]any, 0, len(rows))
	for _, it := range rows {
		entry := map[string]any{
			"kind":               string(it.Kind),
			"publisher_slug":     it.PublisherSlug,
			"entry_slug":         it.EntrySlug,
			"entry_id":           it.EntryID,
			"submitted_at":       it.SubmittedAt,
			"submitted_by":       it.SubmittedBy,
			"submitted_by_email": it.SubmittedByEmail,
		}
		// Version-flavored items carry a version + revision; deletion-
		// flavored items don't (revision is meaningless and the version
		// field is empty by convention).
		if it.Version != "" {
			entry["version"] = it.Version
			entry["revision"] = it.Revision
		}
		// Entry-change items carry the change id, the action, the revision
		// (for the approve/reject optimistic-concurrency token), and the
		// proposed mutation payload so the UI can render a diff.
		if it.ChangeID != "" {
			entry["change_id"] = it.ChangeID
			entry["action"] = it.Action
			entry["revision"] = it.Revision
			if len(it.Payload) > 0 {
				entry["payload"] = it.Payload
			}
		}
		items = append(items, entry)
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	})
}

// ── MCP server version flow ─────────────────────────────────────────────

// SubmitMCPVersion: POST /api/v1/mcp/servers/{ns}/{slug}/versions/{ver}/submit
func (h *ReviewHandlers) SubmitMCPVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")
	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.SubmitMCPVersion(r.Context(), srv.ID, ver, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPVersionSubmitted, ResourceType: "mcp_server_version",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": ver},
	})
	w.WriteHeader(http.StatusNoContent)
}

// WithdrawMCPVersion: POST .../versions/{ver}/withdraw
func (h *ReviewHandlers) WithdrawMCPVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")
	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.WithdrawMCPVersion(r.Context(), srv.ID, ver, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPVersionWithdrawn, ResourceType: "mcp_server_version",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": ver},
	})
	w.WriteHeader(http.StatusNoContent)
}

// ApproveMCPVersion: POST .../versions/{ver}/approve   body: {"revision": N}
func (h *ReviewHandlers) ApproveMCPVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")

	var body struct {
		Revision int `json:"revision"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Revision <= 0 {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"revision is required and must be > 0", r.URL.Path)
		return
	}

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.ApproveMCPVersion(r.Context(), srv.ID, ver, body.Revision, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPVersionApproved, ResourceType: "mcp_server_version",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": ver, "revision": body.Revision},
	})
	w.WriteHeader(http.StatusNoContent)
}

// RejectMCPVersion: POST .../versions/{ver}/reject   body: {"revision": N, "reason": "..."}
func (h *ReviewHandlers) RejectMCPVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")

	var body struct {
		Revision int    `json:"revision"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Revision <= 0 {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"revision is required and must be > 0", r.URL.Path)
		return
	}
	if body.Reason == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"reason is required on reject", r.URL.Path)
		return
	}

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.RejectMCPVersion(r.Context(), srv.ID, ver, body.Revision, body.Reason, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPVersionRejected, ResourceType: "mcp_server_version",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{
			"version":  ver,
			"revision": body.Revision,
			"reason":   body.Reason,
		},
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── MCP server deletion flow ────────────────────────────────────────────

// RequestMCPDeletion: POST /api/v1/mcp/servers/{ns}/{slug}/deletion-request
func (h *ReviewHandlers) RequestMCPDeletion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.RequestMCPDeletion(r.Context(), srv.ID, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPDeletionRequested, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	w.WriteHeader(http.StatusAccepted)
}

// ApproveMCPDeletion: POST .../deletion-request/approve
func (h *ReviewHandlers) ApproveMCPDeletion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.ApproveMCPDeletion(r.Context(), srv.ID, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPDeletionApproved, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	w.WriteHeader(http.StatusNoContent)
}

// RejectMCPDeletion: POST .../deletion-request/reject   body: {"reason": "..."}
func (h *ReviewHandlers) RejectMCPDeletion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	var body struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Reason == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"reason is required on reject", r.URL.Path)
		return
	}

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.RejectMCPDeletion(r.Context(), srv.ID, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPDeletionRejected, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"reason": body.Reason},
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── Agent version flow ──────────────────────────────────────────────────

// SubmitAgentVersion: POST /api/v1/agents/{ns}/{slug}/versions/{ver}/submit
func (h *ReviewHandlers) SubmitAgentVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")
	ag, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.SubmitAgentVersion(r.Context(), ag.ID, ver, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentVersionSubmitted, ResourceType: "agent_version",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": ver},
	})
	w.WriteHeader(http.StatusNoContent)
}

// WithdrawAgentVersion: POST .../versions/{ver}/withdraw
func (h *ReviewHandlers) WithdrawAgentVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")
	ag, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.WithdrawAgentVersion(r.Context(), ag.ID, ver, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentVersionWithdrawn, ResourceType: "agent_version",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": ver},
	})
	w.WriteHeader(http.StatusNoContent)
}

// ApproveAgentVersion: POST .../versions/{ver}/approve   body: {"revision": N}
func (h *ReviewHandlers) ApproveAgentVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")

	var body struct {
		Revision int `json:"revision"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Revision <= 0 {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"revision is required and must be > 0", r.URL.Path)
		return
	}

	ag, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.ApproveAgentVersion(r.Context(), ag.ID, ver, body.Revision, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentVersionApproved, ResourceType: "agent_version",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": ver, "revision": body.Revision},
	})
	w.WriteHeader(http.StatusNoContent)
}

// RejectAgentVersion: POST .../versions/{ver}/reject   body: {"revision": N, "reason": "..."}
func (h *ReviewHandlers) RejectAgentVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")

	var body struct {
		Revision int    `json:"revision"`
		Reason   string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Revision <= 0 {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"revision is required and must be > 0", r.URL.Path)
		return
	}
	if body.Reason == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"reason is required on reject", r.URL.Path)
		return
	}

	ag, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.RejectAgentVersion(r.Context(), ag.ID, ver, body.Revision, body.Reason, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentVersionRejected, ResourceType: "agent_version",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{
			"version":  ver,
			"revision": body.Revision,
			"reason":   body.Reason,
		},
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── Agent deletion flow ─────────────────────────────────────────────────

// RequestAgentDeletion: POST /api/v1/agents/{ns}/{slug}/deletion-request
func (h *ReviewHandlers) RequestAgentDeletion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ag, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.RequestAgentDeletion(r.Context(), ag.ID, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentDeletionRequested, ResourceType: "agent",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	w.WriteHeader(http.StatusAccepted)
}

// ApproveAgentDeletion: POST .../deletion-request/approve
func (h *ReviewHandlers) ApproveAgentDeletion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ag, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.ApproveAgentDeletion(r.Context(), ag.ID, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentDeletionApproved, ResourceType: "agent",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	w.WriteHeader(http.StatusNoContent)
}

// RejectAgentDeletion: POST .../deletion-request/reject   body: {"reason": "..."}
func (h *ReviewHandlers) RejectAgentDeletion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	var body struct {
		Reason string `json:"reason"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Reason == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"reason is required on reject", r.URL.Path)
		return
	}

	ag, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.RejectAgentDeletion(r.Context(), ag.ID, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentDeletionRejected, ResourceType: "agent",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"reason": body.Reason},
	})
	w.WriteHeader(http.StatusNoContent)
}
