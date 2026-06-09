// Package handlers — entry-change approval workflow.
//
// An Editor's request to change an entry's visibility, deprecate it, or edit
// its metadata is no longer applied immediately; it is enqueued as a pending
// entry-change request that a Reviewer approves or rejects. (Server Admins keep
// the immediate path as a break-glass escape hatch — see the visibility /
// deprecate / patch handlers.) These approve/reject/withdraw endpoints mirror
// the deletion-request flow: path-scoped per ns/slug, reusing requireReviewerNS
// / requireMCPServerNS, resolving the single pending change for the entry.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// pendingChangeInfo returns a compact summary of an entry's pending change for
// the detail response, or ok=false when none is pending (or on error — a
// transient lookup failure must not break the read path).
func pendingChangeInfo(ctx context.Context, db *store.DB, resourceType domain.EntryResourceType, entryID string) (map[string]any, bool) {
	cr, ok, err := db.GetPendingEntryChange(ctx, resourceType, entryID)
	if err != nil || !ok {
		return nil, false
	}
	info := map[string]any{
		"change_id":          cr.ID,
		"action":             string(cr.Action),
		"revision":           cr.Revision,
		"submitted_at":       cr.SubmittedAt.Format("2006-01-02T15:04:05Z07:00"),
		"submitted_by_email": cr.SubmittedByEmail,
	}
	if len(cr.Payload) > 0 {
		info["payload"] = json.RawMessage(cr.Payload)
	}
	return info, true
}

// enqueueEntryChange creates a pending entry-change request and writes the 202
// response, mapping a duplicate-pending collision to a discriminated 409. Used
// by the visibility / deprecate / patch handlers on the non-admin path.
func enqueueEntryChange(
	w http.ResponseWriter, r *http.Request, db *store.DB, audit store.AuditLogger,
	resourceType domain.EntryResourceType, entryID, ns, slug string,
	action domain.EntryChangeAction, payload any, auditAction domain.AuditAction,
) {
	raw, err := json.Marshal(payload)
	if err != nil {
		internalError(w, r, err)
		return
	}
	id, err := db.CreateEntryChangeRequest(r.Context(), resourceType, entryID, action, raw, reviewActor(r))
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "change-already-pending",
			"this entry already has a change awaiting review; withdraw or resolve it first", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: auditAction, ResourceType: string(resourceType),
		ResourceID: entryID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"action": string(action), "change_id": id},
	})
	writeJSON(w, r, http.StatusAccepted, map[string]any{
		"status":    "pending_review",
		"change_id": id,
	})
}

// ── MCP entry-change review endpoints ────────────────────────────────────────

// ApproveMCPChange: POST /api/v1/mcp/servers/{ns}/{slug}/change-request/approve
// body: {"revision": N}
func (h *ReviewHandlers) ApproveMCPChange(w http.ResponseWriter, r *http.Request) {
	ns, slug := chi.URLParam(r, "namespace"), chi.URLParam(r, "slug")
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
	if !h.resolvedEntry(w, r, err, "MCP server", ns, slug) {
		return
	}
	cr, ok := h.pendingChange(w, r, domain.EntryResourceMCPServer, srv.ID)
	if !ok {
		return
	}
	applied, err := h.db.ApproveEntryChangeRequest(r.Context(), cr.ID, body.Revision, reviewActor(r))
	if err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPChangeApproved, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"action": string(applied.Action), "change_id": applied.ID},
	})
	w.WriteHeader(http.StatusNoContent)
}

// RejectMCPChange: POST .../change-request/reject   body: {"revision": N, "reason": "..."}
func (h *ReviewHandlers) RejectMCPChange(w http.ResponseWriter, r *http.Request) {
	ns, slug := chi.URLParam(r, "namespace"), chi.URLParam(r, "slug")
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
	if !h.resolvedEntry(w, r, err, "MCP server", ns, slug) {
		return
	}
	cr, ok := h.pendingChange(w, r, domain.EntryResourceMCPServer, srv.ID)
	if !ok {
		return
	}
	if err := h.db.RejectEntryChangeRequest(r.Context(), cr.ID, body.Revision, body.Reason, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPChangeRejected, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"action": string(cr.Action), "change_id": cr.ID, "reason": body.Reason},
	})
	w.WriteHeader(http.StatusNoContent)
}

// WithdrawMCPChange: POST .../change-request/withdraw   (Editor; no body)
func (h *ReviewHandlers) WithdrawMCPChange(w http.ResponseWriter, r *http.Request) {
	ns, slug := chi.URLParam(r, "namespace"), chi.URLParam(r, "slug")
	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, false)
	if !h.resolvedEntry(w, r, err, "MCP server", ns, slug) {
		return
	}
	cr, ok := h.pendingChange(w, r, domain.EntryResourceMCPServer, srv.ID)
	if !ok {
		return
	}
	if err := h.db.WithdrawEntryChangeRequest(r.Context(), cr.ID, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPChangeWithdrawn, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"action": string(cr.Action), "change_id": cr.ID},
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── Agent entry-change review endpoints ──────────────────────────────────────

// ApproveAgentChange: POST /api/v1/agents/{ns}/{slug}/change-request/approve
func (h *ReviewHandlers) ApproveAgentChange(w http.ResponseWriter, r *http.Request) {
	ns, slug := chi.URLParam(r, "namespace"), chi.URLParam(r, "slug")
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
	if !h.resolvedEntry(w, r, err, "agent", ns, slug) {
		return
	}
	cr, ok := h.pendingChange(w, r, domain.EntryResourceAgent, ag.ID)
	if !ok {
		return
	}
	applied, err := h.db.ApproveEntryChangeRequest(r.Context(), cr.ID, body.Revision, reviewActor(r))
	if err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentChangeApproved, ResourceType: "agent",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"action": string(applied.Action), "change_id": applied.ID},
	})
	w.WriteHeader(http.StatusNoContent)
}

// RejectAgentChange: POST .../change-request/reject
func (h *ReviewHandlers) RejectAgentChange(w http.ResponseWriter, r *http.Request) {
	ns, slug := chi.URLParam(r, "namespace"), chi.URLParam(r, "slug")
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
	if !h.resolvedEntry(w, r, err, "agent", ns, slug) {
		return
	}
	cr, ok := h.pendingChange(w, r, domain.EntryResourceAgent, ag.ID)
	if !ok {
		return
	}
	if err := h.db.RejectEntryChangeRequest(r.Context(), cr.ID, body.Revision, body.Reason, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentChangeRejected, ResourceType: "agent",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"action": string(cr.Action), "change_id": cr.ID, "reason": body.Reason},
	})
	w.WriteHeader(http.StatusNoContent)
}

// WithdrawAgentChange: POST .../change-request/withdraw
func (h *ReviewHandlers) WithdrawAgentChange(w http.ResponseWriter, r *http.Request) {
	ns, slug := chi.URLParam(r, "namespace"), chi.URLParam(r, "slug")
	ag, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if !h.resolvedEntry(w, r, err, "agent", ns, slug) {
		return
	}
	cr, ok := h.pendingChange(w, r, domain.EntryResourceAgent, ag.ID)
	if !ok {
		return
	}
	if err := h.db.WithdrawEntryChangeRequest(r.Context(), cr.ID, reviewActor(r)); err != nil {
		if writeReviewProblem(w, r, err) {
			return
		}
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentChangeWithdrawn, ResourceType: "agent",
		ResourceID: ag.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"action": string(cr.Action), "change_id": cr.ID},
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── shared helpers ───────────────────────────────────────────────────────────

// resolvedEntry maps a GetMCPServer/GetAgent lookup error to a 404/500 and
// returns false if the caller should stop. true means the entry was found.
func (h *ReviewHandlers) resolvedEntry(w http.ResponseWriter, r *http.Request, err error, kind, ns, slug string) bool {
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("%s '%s/%s' does not exist", kind, ns, slug), r.URL.Path)
		return false
	}
	if err != nil {
		internalError(w, r, err)
		return false
	}
	return true
}

// pendingChange loads the entry's single pending change, writing a 500/404 and
// returning ok=false if it can't be resolved.
func (h *ReviewHandlers) pendingChange(w http.ResponseWriter, r *http.Request, resourceType domain.EntryResourceType, entryID string) (domain.EntryChangeRequest, bool) {
	cr, ok, err := h.db.GetPendingEntryChange(r.Context(), resourceType, entryID)
	if err != nil {
		internalError(w, r, err)
		return domain.EntryChangeRequest{}, false
	}
	if !ok {
		problem.Write(w, http.StatusNotFound, "not-found",
			"no change is pending review for this entry", r.URL.Path)
		return domain.EntryChangeRequest{}, false
	}
	return cr, true
}
