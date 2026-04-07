package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// AgentHandlers holds dependencies for agent registry HTTP handlers.
type AgentHandlers struct {
	db    *store.DB
	audit store.AuditLogger
}

// NewAgentHandlers creates AgentHandlers with the given store and audit logger.
func NewAgentHandlers(db *store.DB, audit store.AuditLogger) *AgentHandlers {
	return &AgentHandlers{db: db, audit: audit}
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

	rows, err := h.db.ListAgents(r.Context(), store.ListAgentsParams{
		PublicOnly: !auth.IsAdminFromContext(r.Context()),
		Namespace:  r.URL.Query().Get("namespace"),
		Query:      r.URL.Query().Get("q"),
		Limit:      limit + 1,
		Cursor:     r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "failed to list agents", r.URL.Path)
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nextCursor = store.EncodeCursor(last.CreatedAt, last.ID)
	}

	type item struct {
		ID          string `json:"id"`
		Namespace   string `json:"namespace"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Visibility  string `json:"visibility"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}

	items := make([]item, 0, len(rows))
	for _, r := range rows {
		items = append(items, item{
			ID:          r.ID,
			Namespace:   r.Namespace,
			Slug:        r.Slug,
			Name:        r.Name,
			Description: r.Description,
			Visibility:  string(r.Visibility),
			Status:      string(r.Status),
			CreatedAt:   r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:   r.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	})
}

// ── GET /api/v1/agents/{namespace}/{slug} ─────────────────────────────────

func (h *AgentHandlers) GetAgent(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	agent, err := h.db.GetAgent(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"agent '"+ns+"/"+slug+"' does not exist", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, agent)
}

// ── POST /api/v1/agents ───────────────────────────────────────────────────

func (h *AgentHandlers) CreateAgent(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Namespace   string `json:"namespace"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		return
	}
	if body.Namespace == "" || body.Slug == "" || body.Name == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error",
			"namespace, slug, and name are required", r.URL.Path)
		return
	}

	publisherID, err := h.db.GetPublisherBySlug(r.Context(), body.Namespace)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error",
			"publisher '"+body.Namespace+"' does not exist", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	agent, err := h.db.CreateAgent(r.Context(), store.CreateAgentParams{
		PublisherID: publisherID,
		Slug:        body.Slug,
		Name:        body.Name,
		Description: body.Description,
	})
	if errors.Is(err, store.ErrConflict) {
		writeProblem(w, http.StatusConflict, "conflict",
			"agent '"+body.Namespace+"/"+body.Slug+"' already exists", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionAgentCreated, ResourceType: "agent",
			ResourceID: agent.ID, ResourceNS: body.Namespace, ResourceSlug: body.Slug,
		})
	}
	writeJSON(w, http.StatusCreated, agent)
}

// ── GET /api/v1/agents/{namespace}/{slug}/versions ────────────────────────

func (h *AgentHandlers) ListVersions(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	agent, err := h.db.GetAgent(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"agent '"+ns+"/"+slug+"' does not exist", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	versions, err := h.db.ListAgentVersions(r.Context(), agent.ID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": versions})
}

// ── GET /api/v1/agents/{namespace}/{slug}/versions/{version} ──────────────

func (h *AgentHandlers) GetVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	agent, err := h.db.GetAgent(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"agent '"+ns+"/"+slug+"' does not exist", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	v, err := h.db.GetAgentVersion(r.Context(), agent.ID, ver)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"version '"+ver+"' does not exist for "+ns+"/"+slug, r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// ── POST /api/v1/agents/{namespace}/{slug}/versions ───────────────────────

func (h *AgentHandlers) CreateVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		return
	}
	if body.Version == "" || body.EndpointURL == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error",
			"version and endpoint_url are required", r.URL.Path)
		return
	}
	if body.ProtocolVersion == "" {
		body.ProtocolVersion = domain.A2AProtocolVersion
	}
	if err := domain.ValidateSkills(body.Skills); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}
	if err := domain.ValidateAuthentication(body.Authentication); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
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
		writeProblem(w, http.StatusConflict, "conflict",
			"version '"+body.Version+"' already exists for "+ns+"/"+slug, r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionAgentVersionCreated, ResourceType: "agent",
			ResourceID: agent.ID, ResourceNS: ns, ResourceSlug: slug,
			Metadata: map[string]any{"version": body.Version},
		})
	}
	writeJSON(w, http.StatusCreated, v)
}

// ── POST /api/v1/agents/{namespace}/{slug}/versions/{version}/publish ─────

func (h *AgentHandlers) PublishVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")

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

	if err := h.db.PublishAgentVersion(r.Context(), agent.ID, ver); errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found", "version '"+ver+"' does not exist", r.URL.Path)
		return
	} else if errors.Is(err, store.ErrImmutable) {
		writeProblem(w, http.StatusConflict, "immutable", "version '"+ver+"' is already published", r.URL.Path)
		return
	} else if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionAgentVersionPublished, ResourceType: "agent",
			ResourceID: agent.ID, ResourceNS: ns, ResourceSlug: slug,
			Metadata: map[string]any{"version": ver},
		})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
}

// ── POST /api/v1/agents/{namespace}/{slug}/deprecate ──────────────────────

func (h *AgentHandlers) DeprecateAgent(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

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

	if err := h.db.DeprecateAgent(r.Context(), agent.ID); errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusConflict, "conflict",
			"agent '"+ns+"/"+slug+"' is not in published status", r.URL.Path)
		return
	} else if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionAgentDeprecated, ResourceType: "agent",
			ResourceID: agent.ID, ResourceNS: ns, ResourceSlug: slug,
		})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deprecated"})
}
