package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/observability"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// AgentHandlers holds dependencies for agent registry HTTP handlers.
type AgentHandlers struct {
	db      *store.DB
	audit   store.AuditLogger
	metrics *observability.Metrics
}

// NewAgentHandlers creates AgentHandlers with the given store, audit logger, and metrics.
func NewAgentHandlers(db *store.DB, audit store.AuditLogger, metrics *observability.Metrics) *AgentHandlers {
	return &AgentHandlers{db: db, audit: audit, metrics: metrics}
}

// ── GET /api/v1/agents ────────────────────────────────────────────────────

func (h *AgentHandlers) ListAgents(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	// Validate optional enum filters — silently ignore unknown values.
	status := r.URL.Query().Get("status")
	if status != "draft" && status != "published" && status != "deprecated" {
		status = ""
	}
	visibility := r.URL.Query().Get("visibility")
	if visibility != "public" && visibility != "private" {
		visibility = ""
	}
	sort := r.URL.Query().Get("sort")
	if sort != "created_at_desc" && sort != "updated_at_desc" && sort != "name_asc" && sort != "name_desc" {
		sort = ""
	}

	rows, total, err := h.db.ListAgents(r.Context(), store.ListAgentsParams{
		PublicOnly: !auth.IsAdminFromContext(r.Context()),
		Namespace:  r.URL.Query().Get("namespace"),
		Status:     status,
		Visibility: visibility,
		Query:      r.URL.Query().Get("q"),
		Limit:      limit + 1,
		Cursor:     r.URL.Query().Get("cursor"),
		Sort:       sort,
	})
	if errors.Is(err, store.ErrInvalidCursor) {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "invalid cursor", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		cursorTime := last.CreatedAt
		if sort == "updated_at_desc" {
			cursorTime = last.UpdatedAt
		}
		nextCursor = store.EncodeCursor(cursorTime, last.ID)
	}

	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		items = append(items, agentToResponse(&rows[i]))
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
		"total_count": total,
	})
}

// ── GET /api/v1/agents/{namespace}/{slug} ─────────────────────────────────

func (h *AgentHandlers) GetAgent(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	agent, err := h.db.GetAgent(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, agentToResponse(agent))
}

// ── POST /api/v1/agents ───────────────────────────────────────────────────

func (h *AgentHandlers) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Namespace   string `json:"namespace"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Namespace == "" || body.Slug == "" || body.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"namespace, slug, and name are required", r.URL.Path)
		return
	}
	if err := domain.ValidateSlug(body.Namespace); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			fmt.Sprintf("namespace: %s", err), r.URL.Path)
		return
	}
	if err := domain.ValidateSlug(body.Slug); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			fmt.Sprintf("slug: %s", err), r.URL.Path)
		return
	}

	publisherID, err := h.db.GetPublisherBySlug(r.Context(), body.Namespace)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			fmt.Sprintf("publisher '%s' does not exist", body.Namespace), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	agent, err := h.db.CreateAgent(r.Context(), store.CreateAgentParams{
		PublisherID: publisherID,
		Slug:        body.Slug,
		Name:        body.Name,
		Description: body.Description,
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("agent '%s/%s' already exists", body.Namespace, body.Slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentCreated, ResourceType: "agent",
		ResourceID: agent.ID, ResourceNS: body.Namespace, ResourceSlug: body.Slug,
	})
	if h.metrics != nil {
		h.metrics.AgentsTotal.Add(r.Context(), 1)
	}
	writeJSON(w, r, http.StatusCreated, agent)
}

// ── GET /api/v1/agents/{namespace}/{slug}/versions ────────────────────────

func (h *AgentHandlers) ListVersions(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	agent, err := h.db.GetAgent(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	versions, err := h.db.ListAgentVersions(r.Context(), agent.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": versions})
}

// ── GET /api/v1/agents/{namespace}/{slug}/versions/{version} ──────────────

func (h *AgentHandlers) GetVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	agent, err := h.db.GetAgent(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	v, err := h.db.GetAgentVersion(r.Context(), agent.ID, ver)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("version '%s' does not exist for %s/%s", ver, ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, v)
}

// ── POST /api/v1/agents/{namespace}/{slug}/versions ───────────────────────

func (h *AgentHandlers) CreateVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

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

	var body struct {
		Version            string          `json:"version"`
		EndpointURL        string          `json:"endpoint_url"`
		Skills             json.RawMessage `json:"skills"`
		Capabilities       json.RawMessage `json:"capabilities"`
		Authentication     json.RawMessage `json:"authentication"`
		DefaultInputModes  []string        `json:"default_input_modes"`
		DefaultOutputModes []string        `json:"default_output_modes"`
		Provider           json.RawMessage `json:"provider"`
		DocumentationURL   string          `json:"documentation_url"`
		IconURL            string          `json:"icon_url"`
		ProtocolVersion    string          `json:"protocol_version"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Version == "" || body.EndpointURL == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"version and endpoint_url are required", r.URL.Path)
		return
	}
	if body.ProtocolVersion == "" {
		body.ProtocolVersion = domain.A2AProtocolVersion
	}
	if err := domain.ValidateSkills(body.Skills); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}
	if err := domain.ValidateAuthentication(body.Authentication); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}

	v, err := h.db.CreateAgentVersion(r.Context(), store.CreateAgentVersionParams{
		AgentID:            agent.ID,
		Version:            body.Version,
		EndpointURL:        body.EndpointURL,
		Skills:             body.Skills,
		Capabilities:       body.Capabilities,
		Authentication:     body.Authentication,
		DefaultInputModes:  body.DefaultInputModes,
		DefaultOutputModes: body.DefaultOutputModes,
		Provider:           body.Provider,
		DocumentationURL:   body.DocumentationURL,
		IconURL:            body.IconURL,
		ProtocolVersion:    body.ProtocolVersion,
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("version '%s' already exists for %s/%s", body.Version, ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentVersionCreated, ResourceType: "agent",
		ResourceID: agent.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": body.Version},
	})
	writeJSON(w, r, http.StatusCreated, v)
}

// ── POST /api/v1/agents/{namespace}/{slug}/versions/{version}/publish ─────

func (h *AgentHandlers) PublishVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")

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

	if err := h.db.PublishAgentVersion(r.Context(), agent.ID, ver); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("version '%s' does not exist", ver), r.URL.Path)
		return
	} else if errors.Is(err, store.ErrImmutable) {
		problem.Write(w, http.StatusConflict, "immutable",
			fmt.Sprintf("version '%s' is already published", ver), r.URL.Path)
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentVersionPublished, ResourceType: "agent",
		ResourceID: agent.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": ver},
	})
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "published"})
}

// ── POST /api/v1/agents/{namespace}/{slug}/deprecate ──────────────────────

func (h *AgentHandlers) DeprecateAgent(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

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

	if err := h.db.DeprecateAgent(r.Context(), agent.ID); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("agent '%s/%s' is not in published status", ns, slug), r.URL.Path)
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentDeprecated, ResourceType: "agent",
		ResourceID: agent.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "deprecated"})
}

// ── PATCH /api/v1/agents/{namespace}/{slug} ───────────────────────────────

func (h *AgentHandlers) PatchAgent(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

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

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	p := store.UpdateAgentParams{
		Name:        agent.Name,
		Description: agent.Description,
	}
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.Description != nil {
		p.Description = *body.Description
	}

	if p.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"name is required", r.URL.Path)
		return
	}

	updated, err := h.db.UpdateAgent(r.Context(), agent.ID, p)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentUpdated, ResourceType: "agent",
		ResourceID: agent.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	writeJSON(w, r, http.StatusOK, agentToResponse(updated))
}

// ── DELETE /api/v1/agents/{namespace}/{slug} ──────────────────────────────

func (h *AgentHandlers) DeleteAgent(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

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

	if err := h.db.DeleteAgent(r.Context(), agent.ID); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentDeleted, ResourceType: "agent",
		ResourceID: agent.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── PATCH /api/v1/agents/{namespace}/{slug}/versions/{version}/status ────────

// PatchVersionStatus updates the lifecycle status of a specific agent version.
// Body: { "status": "active"|"deprecated"|"deleted", "statusMessage": "..." }
func (h *AgentHandlers) PatchVersionStatus(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")

	var body struct {
		Status        string `json:"status"`
		StatusMessage string `json:"statusMessage"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	validStatuses := map[string]bool{"active": true, "deprecated": true, "deleted": true}
	if !validStatuses[body.Status] {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"status must be one of: active, deprecated, deleted", r.URL.Path)
		return
	}
	if body.Status == "active" && body.StatusMessage != "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"statusMessage must not be set when status is active", r.URL.Path)
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

	domainStatus := domain.VersionStatus(body.Status)
	if err := h.db.SetAgentVersionStatus(r.Context(), agent.ID, ver, domainStatus, body.StatusMessage); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("version '%s' does not exist", ver), r.URL.Path)
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}

	v, err := h.db.GetAgentVersion(r.Context(), agent.ID, ver)
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionAgentVersionPublished, ResourceType: "agent",
		ResourceID: agent.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": ver, "status": body.Status},
	})
	writeJSON(w, r, http.StatusOK, v)
}

// ── helper ────────────────────────────────────────────────────────────────

func agentToResponse(a *store.AgentRow) map[string]any {
	m := map[string]any{
		"id":          a.ID,
		"namespace":   a.Namespace,
		"slug":        a.Slug,
		"name":        a.Name,
		"description": a.Description,
		"visibility":  string(a.Visibility),
		"status":      string(a.Status),
		"created_at":  a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":  a.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if lv := a.LatestVersion; lv != nil {
		m["latest_version"] = map[string]any{
			"version":              lv.Version,
			"endpoint_url":         lv.EndpointURL,
			"skills":               lv.Skills,
			"default_input_modes":  lv.DefaultInputModes,
			"default_output_modes": lv.DefaultOutputModes,
			"authentication":       lv.Authentication,
			"protocol_version":     lv.ProtocolVersion,
			"published_at":         lv.PublishedAt,
		}
	}
	return m
}
