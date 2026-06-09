package handlers

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// SetMCPServerVisibility handles POST /api/v1/mcp/servers/{namespace}/{slug}/visibility.
func (h *MCPHandlers) SetVisibility(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	var body struct {
		Visibility string `json:"visibility"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Visibility != "public" && body.Visibility != "private" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			`visibility must be "public" or "private"`, r.URL.Path)
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

	// An entry can only be exposed publicly once it has an approved (published)
	// version — you cannot publish an unreviewed draft to the public catalog.
	// Going back to private is always allowed.
	if body.Visibility == "public" && srv.Status != domain.StatusPublished && srv.Status != domain.StatusDeprecated {
		problem.Write(w, http.StatusConflict, "visibility-requires-approval",
			"This entry has no approved version yet; submit it and have a Reviewer approve it before making it public.", r.URL.Path)
		return
	}

	// Editors enqueue the change for review; Server Admins keep the immediate
	// path as a break-glass escape hatch.
	if !auth.IsServerAdminFromContext(r.Context()) {
		enqueueEntryChange(w, r, h.db, h.audit, domain.EntryResourceMCPServer, srv.ID, ns, slug,
			domain.EntryChangeVisibility, map[string]string{"visibility": body.Visibility},
			domain.ActionMCPChangeRequested)
		return
	}

	if err := h.db.SetMCPServerVisibility(r.Context(), srv.ID, domain.Visibility(body.Visibility)); err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject,
		ActorEmail:   email,
		Action:       domain.ActionMCPServerVisibility,
		ResourceType: "mcp_server",
		ResourceID:   srv.ID,
		ResourceNS:   ns,
		ResourceSlug: slug,
		Metadata:     map[string]any{"visibility": body.Visibility},
	})

	writeJSON(w, r, http.StatusOK, map[string]string{"visibility": body.Visibility})
}

// SetAgentVisibility handles POST /api/v1/agents/{namespace}/{slug}/visibility.
func (h *AgentHandlers) SetVisibility(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	var body struct {
		Visibility string `json:"visibility"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Visibility != "public" && body.Visibility != "private" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			`visibility must be "public" or "private"`, r.URL.Path)
		return
	}

	agent, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	// Public exposure requires an approved (published) version (see the MCP
	// handler note). Going back to private is always allowed.
	if body.Visibility == "public" && agent.Status != domain.StatusPublished && agent.Status != domain.StatusDeprecated {
		problem.Write(w, http.StatusConflict, "visibility-requires-approval",
			"This entry has no approved version yet; submit it and have a Reviewer approve it before making it public.", r.URL.Path)
		return
	}

	if !auth.IsServerAdminFromContext(r.Context()) {
		enqueueEntryChange(w, r, h.db, h.audit, domain.EntryResourceAgent, agent.ID, ns, slug,
			domain.EntryChangeVisibility, map[string]string{"visibility": body.Visibility},
			domain.ActionAgentChangeRequested)
		return
	}

	if err := h.db.SetAgentVisibility(r.Context(), agent.ID, domain.Visibility(body.Visibility)); err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject,
		ActorEmail:   email,
		Action:       domain.ActionAgentVisibility,
		ResourceType: "agent",
		ResourceID:   agent.ID,
		ResourceNS:   ns,
		ResourceSlug: slug,
		Metadata:     map[string]any{"visibility": body.Visibility},
	})

	writeJSON(w, r, http.StatusOK, map[string]string{"visibility": body.Visibility})
}
