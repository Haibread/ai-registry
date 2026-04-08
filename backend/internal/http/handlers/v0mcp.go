package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	mcpwire "github.com/haibread/ai-registry/internal/mcp"
	"github.com/haibread/ai-registry/internal/store"
)

// V0MCPHandlers serves the strict MCP registry wire-format endpoints at /v0/.
type V0MCPHandlers struct {
	db    *store.DB
	audit store.AuditLogger
}

// NewV0MCPHandlers creates a V0MCPHandlers with the given store and audit logger.
func NewV0MCPHandlers(db *store.DB, audit store.AuditLogger) *V0MCPHandlers {
	return &V0MCPHandlers{db: db, audit: audit}
}

// writeV0Error writes a spec-compliant error response for v0 routes.
// Spec: { "error": "<message>" }
func writeV0Error(w http.ResponseWriter, r *http.Request, status int, message string) {
	writeJSON(w, r, status, map[string]string{"error": message})
}

// v0InternalError logs err and returns a generic 500 for v0 routes.
func v0InternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "internal error", slog.String("err", err.Error()))
	writeV0Error(w, r, http.StatusInternalServerError, "an internal error occurred")
}

// ── GET /v0/servers ───────────────────────────────────────────────────────

func (h *V0MCPHandlers) ListServers(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	// Accept both "search" (spec) and "q" (legacy) as the search parameter.
	query := r.URL.Query().Get("search")
	if query == "" {
		query = r.URL.Query().Get("q")
	}

	params := store.ListMCPServersParams{
		PublicOnly: true,
		Query:      query,
		Limit:      limit + 1,
		Cursor:     r.URL.Query().Get("cursor"),
	}

	// updated_since filter (RFC 3339)
	if us := r.URL.Query().Get("updated_since"); us != "" {
		if t, err := time.Parse(time.RFC3339, us); err == nil {
			params.UpdatedSince = &t
		}
	}

	// include_deleted filter
	if r.URL.Query().Get("include_deleted") == "true" {
		params.IncludeDeleted = true
	}

	// version filter: "latest" (default behaviour) or exact semver
	if vf := r.URL.Query().Get("version"); vf != "" {
		params.VersionFilter = vf
	}

	rows, _, err := h.db.ListMCPServers(r.Context(), params)
	if err != nil {
		writeV0Error(w, r, http.StatusInternalServerError, "failed to list servers")
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nextCursor = store.EncodeCursor(last.CreatedAt, last.ID)
	}

	entries := make([]mcpwire.ServerResponse, 0, len(rows))
	for _, row := range rows {
		ver, _ := h.db.GetLatestPublishedVersion(r.Context(), row.ID)
		entries = append(entries, mcpwire.ToServerResponse(row, ver, true))
	}

	writeJSON(w, r, http.StatusOK, mcpwire.ListResponse{
		Servers: entries,
		Metadata: mcpwire.ListMetadata{
			Count:      len(entries),
			NextCursor: nextCursor,
		},
	})
}

// ── GET /v0/servers/{id} ──────────────────────────────────────────────────

// GetServer serves GET /v0/servers/{id} where {id} is the MCP server ULID.
func (h *V0MCPHandlers) GetServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	srv, err := h.db.GetMCPServerByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, r, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		v0InternalError(w, r, err)
		return
	}
	if srv.Visibility != domain.VisibilityPublic {
		writeV0Error(w, r, http.StatusNotFound, "server not found")
		return
	}

	ver, _ := h.db.GetLatestPublishedVersion(r.Context(), srv.ID)
	writeJSON(w, r, http.StatusOK, mcpwire.ToServerResponse(*srv, ver, true))
}

// ── GET /v0/servers/{namespace}/{slug} ───────────────────────────────────

// GetServerByName serves GET /v0/servers/{namespace}/{slug} — lookup by name.
func (h *V0MCPHandlers) GetServerByName(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, true)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, r, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		v0InternalError(w, r, err)
		return
	}

	ver, _ := h.db.GetLatestPublishedVersion(r.Context(), srv.ID)
	writeJSON(w, r, http.StatusOK, mcpwire.ToServerResponse(*srv, ver, true))
}

// ── GET /v0/servers/{namespace}/{slug}/versions ──────────────────────────

// ListServerVersions serves GET /v0/servers/{namespace}/{slug}/versions.
// Returns a ListResponse (ServerList shape) where each item is a ServerResponse.
func (h *V0MCPHandlers) ListServerVersions(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, true)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, r, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		v0InternalError(w, r, err)
		return
	}

	vers, err := h.db.ListMCPServerVersions(r.Context(), srv.ID)
	if err != nil {
		writeV0Error(w, r, http.StatusInternalServerError, "failed to list versions")
		return
	}

	// Find the latest published version.
	var latestVer *domain.MCPServerVersion
	for i := range vers {
		if vers[i].PublishedAt != nil {
			if latestVer == nil || vers[i].PublishedAt.After(*latestVer.PublishedAt) {
				v := vers[i]
				latestVer = &v
			}
		}
	}

	entries := make([]mcpwire.ServerResponse, 0, len(vers))
	for i := range vers {
		v := vers[i]
		if v.PublishedAt == nil {
			continue // only expose published versions
		}
		isLatest := latestVer != nil && v.Version == latestVer.Version
		entries = append(entries, mcpwire.ToServerResponse(*srv, &v, isLatest))
	}

	writeJSON(w, r, http.StatusOK, mcpwire.ListResponse{
		Servers: entries,
		Metadata: mcpwire.ListMetadata{
			Count: len(entries),
		},
	})
}

// ── GET /v0/servers/{namespace}/{slug}/versions/{version} ────────────────

// GetServerVersion serves GET /v0/servers/{namespace}/{slug}/versions/{version}.
// Supports version == "latest" to get the latest published version.
func (h *V0MCPHandlers) GetServerVersion(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	version := chi.URLParam(r, "version")

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, true)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, r, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		v0InternalError(w, r, err)
		return
	}

	var ver *domain.MCPServerVersion
	if version == "latest" {
		ver, err = h.db.GetLatestPublishedVersion(r.Context(), srv.ID)
	} else {
		ver, err = h.db.GetMCPServerVersion(r.Context(), srv.ID, version)
	}
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, r, http.StatusNotFound, "version not found")
		return
	}
	if err != nil {
		v0InternalError(w, r, err)
		return
	}
	if ver.PublishedAt == nil {
		writeV0Error(w, r, http.StatusNotFound, "version not found")
		return
	}

	// Check if this is the latest version.
	latestVer, _ := h.db.GetLatestPublishedVersion(r.Context(), srv.ID)
	isLatest := latestVer != nil && latestVer.Version == ver.Version

	writeJSON(w, r, http.StatusOK, mcpwire.ToServerResponse(*srv, ver, isLatest))
}

// ── PATCH /v0/servers/{namespace}/{slug}/status ──────────────────────────

// PatchServerStatus serves PATCH /v0/servers/{namespace}/{slug}/status.
// Sets status across all published versions.
// Returns 200 AllVersionsStatusResponse.
func (h *V0MCPHandlers) PatchServerStatus(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	var body mcpwire.StatusPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeV0Error(w, r, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	validStatuses := map[string]bool{"active": true, "deprecated": true, "deleted": true}
	if !validStatuses[body.Status] {
		writeV0Error(w, r, http.StatusUnprocessableEntity, "status must be one of: active, deprecated, deleted")
		return
	}

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, r, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		v0InternalError(w, r, err)
		return
	}

	// Map spec status to domain status.
	var domainStatus domain.ServerStatus
	var domainVersionStatus domain.VersionStatus
	switch body.Status {
	case "active":
		domainStatus = domain.StatusPublished
		domainVersionStatus = domain.VersionStatusActive
	case "deprecated":
		domainStatus = domain.StatusDeprecated
		domainVersionStatus = domain.VersionStatusDeprecated
	case "deleted":
		domainStatus = domain.StatusDeleted
		domainVersionStatus = domain.VersionStatusDeleted
	}

	// Update server-level status.
	if err := h.db.SetMCPServerStatus(r.Context(), srv.ID, domainStatus); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeV0Error(w, r, http.StatusNotFound, "server not found")
			return
		}
		writeV0Error(w, r, http.StatusInternalServerError, "failed to update status")
		return
	}

	// Update all published versions' status atomically.
	updatedVersions, err := h.db.SetAllVersionsStatus(r.Context(), srv.ID, domainVersionStatus, body.StatusMessage)
	if err != nil {
		writeV0Error(w, r, http.StatusInternalServerError, "failed to update versions status")
		return
	}

	// Re-fetch server to get updated timestamps.
	updatedSrv, _ := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if updatedSrv == nil {
		updatedSrv = srv
	}

	responses := make([]mcpwire.ServerResponse, 0, len(updatedVersions))
	// Find the latest version.
	var latestVer *domain.MCPServerVersion
	for i := range updatedVersions {
		v := updatedVersions[i]
		if v.PublishedAt != nil && (latestVer == nil || v.PublishedAt.After(*latestVer.PublishedAt)) {
			latestVer = &v
		}
	}
	for i := range updatedVersions {
		v := updatedVersions[i]
		isLatest := latestVer != nil && v.Version == latestVer.Version
		responses = append(responses, mcpwire.ToServerResponse(*updatedSrv, &v, isLatest))
	}

	writeJSON(w, r, http.StatusOK, mcpwire.AllVersionsStatusResponse{
		UpdatedCount: len(updatedVersions),
		Servers:      responses,
	})
}

// ── PATCH /v0/servers/{namespace}/{slug}/versions/{version}/status ────────

// PatchVersionStatus serves PATCH /v0/servers/{namespace}/{slug}/versions/{version}/status.
// Returns 200 ServerResponse.
func (h *V0MCPHandlers) PatchVersionStatus(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	version := chi.URLParam(r, "version")

	var body mcpwire.StatusPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeV0Error(w, r, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	validStatuses := map[string]bool{"active": true, "deprecated": true, "deleted": true}
	if !validStatuses[body.Status] {
		writeV0Error(w, r, http.StatusUnprocessableEntity, "status must be one of: active, deprecated, deleted")
		return
	}

	// statusMessage must not be set when status is active.
	if body.Status == "active" && body.StatusMessage != "" {
		writeV0Error(w, r, http.StatusUnprocessableEntity, "statusMessage must not be set when status is active")
		return
	}

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, r, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		v0InternalError(w, r, err)
		return
	}

	if err := h.db.SetMCPVersionStatus(r.Context(), srv.ID, version, domain.VersionStatus(body.Status), body.StatusMessage); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeV0Error(w, r, http.StatusNotFound, "version not found")
			return
		}
		writeV0Error(w, r, http.StatusInternalServerError, "failed to update version status")
		return
	}

	// Re-fetch the version and server to build the response.
	ver, err := h.db.GetMCPServerVersion(r.Context(), srv.ID, version)
	if err != nil {
		writeV0Error(w, r, http.StatusInternalServerError, "failed to fetch updated version")
		return
	}

	latestVer, _ := h.db.GetLatestPublishedVersion(r.Context(), srv.ID)
	isLatest := latestVer != nil && latestVer.Version == ver.Version

	// Re-fetch server to get updated timestamps.
	updatedSrv, _ := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if updatedSrv == nil {
		updatedSrv = srv
	}

	writeJSON(w, r, http.StatusOK, mcpwire.ToServerResponse(*updatedSrv, ver, isLatest))
}

// ── PUT /v0/servers/{namespace}/{slug}/versions/{version} ─────────────────

// UpdateServerVersion is a stub that returns 501.
func (h *V0MCPHandlers) UpdateServerVersion(w http.ResponseWriter, r *http.Request) {
	writeV0Error(w, r, http.StatusNotImplemented, "UpdateServerVersion not yet implemented")
}

// ── DELETE /v0/servers/{namespace}/{slug}/versions/{version} ──────────────

// DeleteServerVersion is a stub that returns 501.
func (h *V0MCPHandlers) DeleteServerVersion(w http.ResponseWriter, r *http.Request) {
	writeV0Error(w, r, http.StatusNotImplemented, "DeleteServerVersion not yet implemented")
}

// ── POST /v0/publish ──────────────────────────────────────────────────────

func (h *V0MCPHandlers) Publish(w http.ResponseWriter, r *http.Request) {
	// Per spec, body IS the ServerDetail directly (no wrapper object).
	var p mcpwire.PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeV0Error(w, r, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	// Required fields: name, version, protocolVersion, description.
	if p.Name == "" || p.Version == "" || p.ProtocolVersion == "" {
		writeV0Error(w, r, http.StatusUnprocessableEntity,
			"name, version, and protocolVersion are required")
		return
	}
	if p.Description == "" {
		writeV0Error(w, r, http.StatusUnprocessableEntity,
			"description is required (1-100 chars)")
		return
	}

	// Validate name pattern: ^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$
	if err := domain.ValidateServerName(p.Name); err != nil {
		writeV0Error(w, r, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Parse namespace/slug from the MCP name field.
	parts := strings.SplitN(p.Name, "/", 2)
	namespace, slug := parts[0], parts[1]

	if err := domain.ValidatePackages(p.Packages); err != nil {
		writeV0Error(w, r, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Resolve or create the MCP server record.
	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		// Auto-create the server if publisher exists.
		publisherID, pubErr := h.db.GetPublisherBySlug(r.Context(), namespace)
		if errors.Is(pubErr, store.ErrNotFound) {
			writeV0Error(w, r, http.StatusUnprocessableEntity,
				fmt.Sprintf("publisher '%s' does not exist", namespace))
			return
		}
		if pubErr != nil {
			v0InternalError(w, r, pubErr)
			return
		}
		repoURL := ""
		if p.Repository != nil {
			repoURL = p.Repository.URL
		}
		newSrv, createErr := h.db.CreateMCPServer(r.Context(), store.CreateMCPServerParams{
			PublisherID: publisherID,
			Slug:        slug,
			Name:        slug,
			Description: p.Description,
			RepoURL:     repoURL,
		})
		if createErr != nil {
			v0InternalError(w, r, createErr)
			return
		}
		srv = &store.MCPServerRow{MCPServer: *newSrv}
		srv.Namespace = namespace
		if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
			h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
				ActorSubject: claims.Subject, ActorEmail: claims.Email,
				Action: domain.ActionMCPServerCreated, ResourceType: "mcp_server",
				ResourceID: srv.ID, ResourceNS: namespace, ResourceSlug: slug,
			})
		}
	} else if err != nil {
		v0InternalError(w, r, err)
		return
	}

	// Derive runtime from the first package's transport type.
	runtime := deriveRuntime(p.Packages)

	// Create and immediately publish the version.
	ver, err := h.db.CreateMCPServerVersion(r.Context(), store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         p.Version,
		Runtime:         runtime,
		Packages:        p.Packages,
		Capabilities:    p.Capabilities,
		ProtocolVersion: p.ProtocolVersion,
	})
	if errors.Is(err, store.ErrConflict) {
		writeV0Error(w, r, http.StatusConflict,
			"version '"+p.Version+"' already exists for "+p.Name)
		return
	}
	if err != nil {
		v0InternalError(w, r, err)
		return
	}

	if err := h.db.PublishMCPServerVersion(r.Context(), srv.ID, ver.Version); err != nil {
		v0InternalError(w, r, err)
		return
	}
	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionMCPVersionPublished, ResourceType: "mcp_server",
			ResourceID: srv.ID, ResourceNS: namespace, ResourceSlug: slug,
			Metadata: map[string]any{"version": p.Version},
		})
	}

	// Re-fetch the version to get the published_at timestamp.
	publishedVer, _ := h.db.GetMCPServerVersion(r.Context(), srv.ID, ver.Version)

	// Re-fetch the server to get updated status/timestamps.
	updatedSrv, _ := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if updatedSrv == nil {
		updatedSrv = srv
	}

	// Spec: respond with 200 + the ServerResponse shape.
	writeJSON(w, r, http.StatusOK, mcpwire.ToServerResponse(*updatedSrv, publishedVer, true))
}

// deriveRuntime returns a Runtime based on the first package entry's transport type.
func deriveRuntime(packages json.RawMessage) domain.Runtime {
	var entries []struct {
		Transport struct {
			Type string `json:"type"`
		} `json:"transport"`
	}
	if err := json.Unmarshal(packages, &entries); err != nil || len(entries) == 0 {
		return domain.RuntimeStdio
	}
	switch entries[0].Transport.Type {
	case "http":
		return domain.RuntimeHTTP
	case "sse":
		return domain.RuntimeSSE
	case "streamable-http", "streamable_http":
		return domain.RuntimeStreamableHTTP
	default:
		return domain.RuntimeStdio
	}
}
