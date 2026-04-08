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
