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

	p := store.ListMCPServersParams{
		PublicOnly: !auth.IsAdminFromContext(r.Context()),
		Namespace:  r.URL.Query().Get("namespace"),
		Status:     status,
		Visibility: visibility,
		Query:      r.URL.Query().Get("q"),
		Limit:      limit + 1, // fetch one extra to detect next page
		Cursor:     r.URL.Query().Get("cursor"),
	}

	rows, total, err := h.db.ListMCPServers(r.Context(), p)
	if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal", "failed to list servers", r.URL.Path)
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nextCursor = store.EncodeCursor(last.CreatedAt, last.ID)
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		return
	}
	if body.Namespace == "" || body.Slug == "" || body.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"namespace, slug, and name are required", r.URL.Path)
		return
	}

	publisherID, err := h.db.GetPublisherBySlug(r.Context(), body.Namespace)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			fmt.Sprintf("publisher '%s' does not exist", body.Namespace), r.URL.Path)
		return
	}
	if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionMCPServerCreated, ResourceType: "mcp_server",
			ResourceID: srv.ID, ResourceNS: body.Namespace, ResourceSlug: body.Slug,
		})
	}
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	versions, err := h.db.ListMCPServerVersions(r.Context(), srv.ID)
	if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	v, err := h.db.GetMCPServerVersion(r.Context(), srv.ID, ver)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("version '%s' does not exist for %s/%s", ver, ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
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
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionMCPVersionCreated, ResourceType: "mcp_server",
			ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
			Metadata: map[string]any{"version": body.Version},
		})
	}
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionMCPVersionPublished, ResourceType: "mcp_server",
			ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
			Metadata: map[string]any{"version": ver},
		})
	}
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	if err := h.db.DeprecateMCPServer(r.Context(), srv.ID); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("server '%s/%s' is not in published status", ns, slug), r.URL.Path)
		return
	} else if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionMCPServerDeprecated, ResourceType: "mcp_server",
			ResourceID: srv.ID, ResourceNS: ns, ResourceSlug: slug,
		})
	}
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "deprecated"})
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
			"published_at":     lv.PublishedAt,
		}
	}
	return m
}
