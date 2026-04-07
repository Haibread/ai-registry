package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// SetMCPServerVisibility handles POST /api/v1/mcp/servers/{namespace}/{slug}/visibility.
func (h *MCPHandlers) SetVisibility(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	var body struct {
		Visibility string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		return
	}
	if body.Visibility != "public" && body.Visibility != "private" {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error",
			`visibility must be "public" or "private"`, r.URL.Path)
		return
	}

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"MCP server '"+ns+"/"+slug+"' does not exist", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	if err := h.db.SetMCPServerVisibility(r.Context(), srv.ID, domain.Visibility(body.Visibility)); err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject,
			ActorEmail:   claims.Email,
			Action:       domain.ActionMCPServerVisibility,
			ResourceType: "mcp_server",
			ResourceID:   srv.ID,
			ResourceNS:   ns,
			ResourceSlug: slug,
			Metadata:     map[string]any{"visibility": body.Visibility},
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"visibility": body.Visibility})
}

// SetAgentVisibility handles POST /api/v1/agents/{namespace}/{slug}/visibility.
func (h *AgentHandlers) SetVisibility(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	var body struct {
		Visibility string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		return
	}
	if body.Visibility != "public" && body.Visibility != "private" {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error",
			`visibility must be "public" or "private"`, r.URL.Path)
		return
	}

	agent, err := h.db.GetAgent(r.Context(), ns, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"agent '"+ns+"/"+slug+"' does not exist", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	if err := h.db.SetAgentVisibility(r.Context(), agent.ID, domain.Visibility(body.Visibility)); err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject,
			ActorEmail:   claims.Email,
			Action:       domain.ActionAgentVisibility,
			ResourceType: "agent",
			ResourceID:   agent.ID,
			ResourceNS:   ns,
			ResourceSlug: slug,
			Metadata:     map[string]any{"visibility": body.Visibility},
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"visibility": body.Visibility})
}
