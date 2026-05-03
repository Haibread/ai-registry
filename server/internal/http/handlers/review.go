// Package handlers — change-approval workflow endpoints.
//
// All seven workflow operations are wired up per resource type:
//
//   submit / withdraw                  -> RequireWorkspaceWrite
//   approve / reject                   -> RequireReviewer (revision-checked)
//   deletion-request                   -> RequireWorkspaceWrite
//   deletion-request/approve / reject  -> RequireReviewer
//
// All non-OK responses use RFC 7807 problem+json with a slug that maps to
// the type URI documented in the OpenAPI spec.

package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

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
	case errors.Is(err, store.ErrConflict):
		problem.Write(w, http.StatusConflict, "review-already-pending",
			"another version on this entry is already in pending_review", r.URL.Path)
	default:
		return false
	}
	return true
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
