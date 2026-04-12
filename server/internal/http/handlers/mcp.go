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

// MCPHandlers holds dependencies for MCP registry HTTP handlers.
type MCPHandlers struct {
	db      *store.DB
	audit   store.AuditLogger
	metrics *observability.Metrics
}

// NewMCPHandlers creates an MCPHandlers with the given store, audit logger, and metrics.
func NewMCPHandlers(db *store.DB, audit store.AuditLogger, metrics *observability.Metrics) *MCPHandlers {
	return &MCPHandlers{db: db, audit: audit, metrics: metrics}
}

// ── GET /api/v1/mcp/servers ───────────────────────────────────────────────

func (h *MCPHandlers) ListServers(w http.ResponseWriter, r *http.Request) {
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
	transport := r.URL.Query().Get("transport")
	if transport != "stdio" && transport != "sse" && transport != "streamable_http" {
		transport = ""
	}
	sort := r.URL.Query().Get("sort")
	if sort != "created_at_desc" && sort != "updated_at_desc" && sort != "name_asc" && sort != "name_desc" {
		sort = ""
	}

	p := store.ListMCPServersParams{
		PublicOnly:   !auth.IsAdminFromContext(r.Context()),
		Namespace:    r.URL.Query().Get("namespace"),
		Status:       status,
		Visibility:   visibility,
		Query:        r.URL.Query().Get("q"),
		Limit:        limit + 1, // fetch one extra to detect next page
		Cursor:       r.URL.Query().Get("cursor"),
		Transport:    transport,
		RegistryType: r.URL.Query().Get("registry_type"),
		Sort:         sort,
	}

	rows, total, err := h.db.ListMCPServers(r.Context(), p)
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
		// Cursor column depends on the sort order.
		cursorTime := last.CreatedAt
		if sort == "updated_at_desc" {
			cursorTime = last.UpdatedAt
		}
		nextCursor = store.EncodeCursor(cursorTime, last.ID)
	}

	items := make([]map[string]any, 0, len(rows))
	for i := range rows {
		items = append(items, serverToResponse(&rows[i]))
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
		"total_count": total,
	})
}

// ── GET /api/v1/mcp/servers/{namespace}/{slug} ────────────────────────────

func (h *MCPHandlers) GetServer(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, serverToResponse(srv))
}

// ── POST /api/v1/mcp/servers ──────────────────────────────────────────────

func (h *MCPHandlers) CreateServer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Namespace   string `json:"namespace"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		HomepageURL string `json:"homepage_url"`
		RepoURL     string `json:"repo_url"`
		License     string `json:"license"`
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

	srv, err := h.db.CreateMCPServer(r.Context(), store.CreateMCPServerParams{
		PublisherID: publisherID,
		Slug:        body.Slug,
		Name:        body.Name,
		Description: body.Description,
		HomepageURL: body.HomepageURL,
		RepoURL:     body.RepoURL,
		License:     body.License,
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("MCP server '%s/%s' already exists", body.Namespace, body.Slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPServerCreated, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: body.Namespace, ResourceSlug: body.Slug,
	})
	if h.metrics != nil {
		h.metrics.MCPServersTotal.Add(r.Context(), 1)
	}
	writeJSON(w, r, http.StatusCreated, srv)
}

// ── GET /api/v1/mcp/servers/{namespace}/{slug}/versions ───────────────────

func (h *MCPHandlers) ListVersions(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	versions, err := h.db.ListMCPServerVersions(r.Context(), srv.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": versions})
}

// ── GET /api/v1/mcp/servers/{namespace}/{slug}/versions/{version} ─────────

func (h *MCPHandlers) GetVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	v, err := h.db.GetMCPServerVersion(r.Context(), srv.ID, ver)
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

// ── POST /api/v1/mcp/servers/{namespace}/{slug}/versions ──────────────────

func (h *MCPHandlers) CreateVersion(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		Version         string          `json:"version"`
		Runtime         string          `json:"runtime"`
		Packages        json.RawMessage `json:"packages"`
		Capabilities    json.RawMessage `json:"capabilities"`
		ProtocolVersion string          `json:"protocol_version"`
		Checksum        string          `json:"checksum"`
		Signature       string          `json:"signature"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Version == "" || body.Runtime == "" || body.ProtocolVersion == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"version, runtime, and protocol_version are required", r.URL.Path)
		return
	}
	if err := domain.ValidatePackages(body.Packages); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}
	if err := domain.ValidateCapabilities(body.Capabilities); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}

	v, err := h.db.CreateMCPServerVersion(r.Context(), store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         body.Version,
		Runtime:         domain.Runtime(body.Runtime),
		Packages:        body.Packages,
		Capabilities:    body.Capabilities,
		ProtocolVersion: body.ProtocolVersion,
		Checksum:        body.Checksum,
		Signature:       body.Signature,
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
		Action: domain.ActionMCPVersionCreated, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": body.Version},
	})
	writeJSON(w, r, http.StatusCreated, v)
}

// ── POST /api/v1/mcp/servers/{namespace}/{slug}/versions/{version}:publish ─

func (h *MCPHandlers) PublishVersion(w http.ResponseWriter, r *http.Request) {
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

	if err := h.db.PublishMCPServerVersion(r.Context(), srv.ID, ver); errors.Is(err, store.ErrNotFound) {
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
		Action: domain.ActionMCPVersionPublished, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		Metadata: map[string]any{"version": ver},
	})
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "published"})
}

// ── POST /api/v1/mcp/servers/{namespace}/{slug}:deprecate ─────────────────

func (h *MCPHandlers) DeprecateServer(w http.ResponseWriter, r *http.Request) {
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

	if err := h.db.DeprecateMCPServer(r.Context(), srv.ID); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("server '%s/%s' is not in published status", ns, slug), r.URL.Path)
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPServerDeprecated, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "deprecated"})
}

// ── PATCH /api/v1/mcp/servers/{namespace}/{slug} ──────────────────────────

func (h *MCPHandlers) PatchServer(w http.ResponseWriter, r *http.Request) {
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

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		HomepageURL *string `json:"homepage_url"`
		RepoURL     *string `json:"repo_url"`
		License     *string `json:"license"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	// Merge non-nil provided fields over current values.
	p := store.UpdateMCPServerParams{
		Name:        srv.Name,
		Description: srv.Description,
		HomepageURL: srv.HomepageURL,
		RepoURL:     srv.RepoURL,
		License:     srv.License,
	}
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.Description != nil {
		p.Description = *body.Description
	}
	if body.HomepageURL != nil {
		p.HomepageURL = *body.HomepageURL
	}
	if body.RepoURL != nil {
		p.RepoURL = *body.RepoURL
	}
	if body.License != nil {
		p.License = *body.License
	}

	if p.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"name is required", r.URL.Path)
		return
	}

	updated, err := h.db.UpdateMCPServer(r.Context(), srv.ID, p)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPServerUpdated, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	writeJSON(w, r, http.StatusOK, serverToResponse(updated))
}

// ── DELETE /api/v1/mcp/servers/{namespace}/{slug} ─────────────────────────

func (h *MCPHandlers) DeleteServer(w http.ResponseWriter, r *http.Request) {
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

	if err := h.db.DeleteMCPServer(r.Context(), srv.ID); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("MCP server '%s/%s' does not exist", ns, slug), r.URL.Path)
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionMCPServerDeleted, ResourceType: "mcp_server",
		ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── helper ────────────────────────────────────────────────────────────────

func serverToResponse(srv *store.MCPServerRow) map[string]any {
	m := map[string]any{
		"id":           srv.ID,
		"namespace":    srv.Namespace,
		"slug":         srv.Slug,
		"name":         srv.Name,
		"description":  srv.Description,
		"homepage_url": srv.HomepageURL,
		"repo_url":     srv.RepoURL,
		"license":      srv.License,
		"visibility":   string(srv.Visibility),
		"status":       string(srv.Status),
		"created_at":   srv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":   srv.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if lv := srv.LatestVersion; lv != nil {
		m["latest_version"] = map[string]any{
			"version":          lv.Version,
			"runtime":          string(lv.Runtime),
			"protocol_version": lv.ProtocolVersion,
			"packages":         lv.Packages,
			"capabilities":     lv.Capabilities,
			"published_at":     lv.PublishedAt,
		}
	}
	return m
}
