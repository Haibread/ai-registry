package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

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
func writeV0Error(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
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

	rows, err := h.db.ListMCPServers(r.Context(), store.ListMCPServersParams{
		PublicOnly: true,
		Query:      query,
		Limit:      limit + 1,
		Cursor:     r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, "failed to list servers")
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nextCursor = store.EncodeCursor(last.CreatedAt, last.ID)
	}

	entries := make([]mcpwire.ServerEntry, 0, len(rows))
	for _, row := range rows {
		ver, _ := h.db.GetLatestPublishedVersion(r.Context(), row.ID)
		entries = append(entries, mcpwire.ToServerEntry(row, ver, true))
	}

	writeJSON(w, http.StatusOK, mcpwire.ListResponse{
		Servers: entries,
		Metadata: mcpwire.ListMetadata{
			Count:      len(entries),
			NextCursor: nextCursor,
		},
	})
}

// ── GET /v0/servers/{id} ──────────────────────────────────────────────────

// V0GetServer serves GET /v0/servers/{id} where {id} is the MCP server ULID.
func (h *V0MCPHandlers) GetServer(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	srv, err := h.db.GetMCPServerByID(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if srv.Visibility != domain.VisibilityPublic {
		writeV0Error(w, http.StatusNotFound, "server not found")
		return
	}

	ver, _ := h.db.GetLatestPublishedVersion(r.Context(), srv.ID)
	writeJSON(w, http.StatusOK, mcpwire.DetailResponse{
		Server: mcpwire.ToServerDetail(*srv, ver),
	})
}

// ── GET /v0/servers/{namespace}/{slug} ───────────────────────────────────

// GetServerByName serves GET /v0/servers/{namespace}/{slug} — lookup by name.
// This is the spec-preferred lookup method (by "namespace/slug" name).
func (h *V0MCPHandlers) GetServerByName(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, true)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	ver, _ := h.db.GetLatestPublishedVersion(r.Context(), srv.ID)
	writeJSON(w, http.StatusOK, mcpwire.DetailResponse{
		Server: mcpwire.ToServerDetail(*srv, ver),
	})
}

// ── GET /v0/servers/{namespace}/{slug}/versions ──────────────────────────

// ListServerVersions serves GET /v0/servers/{namespace}/{slug}/versions.
func (h *V0MCPHandlers) ListServerVersions(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, true)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	vers, err := h.db.ListMCPServerVersions(r.Context(), srv.ID)
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, "failed to list versions")
		return
	}

	entries := make([]mcpwire.VersionEntry, 0, len(vers))
	for _, v := range vers {
		if v.PublishedAt != nil { // only expose published versions
			entries = append(entries, mcpwire.ToVersionEntry(v))
		}
	}

	writeJSON(w, http.StatusOK, mcpwire.VersionListResponse{Versions: entries})
}

// ── GET /v0/servers/{namespace}/{slug}/versions/{version} ────────────────

// GetServerVersion serves GET /v0/servers/{namespace}/{slug}/versions/{version}.
func (h *V0MCPHandlers) GetServerVersion(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	version := chi.URLParam(r, "version")

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, true)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	ver, err := h.db.GetMCPServerVersion(r.Context(), srv.ID, version)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, http.StatusNotFound, "version not found")
		return
	}
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if ver.PublishedAt == nil {
		writeV0Error(w, http.StatusNotFound, "version not found")
		return
	}

	// Return as a full ServerDetail with the specific version's data.
	d := mcpwire.ToServerDetail(*srv, ver)
	writeJSON(w, http.StatusOK, mcpwire.DetailResponse{Server: d})
}

// ── PATCH /v0/servers/{namespace}/{slug}/status ──────────────────────────

// PatchServerStatus serves PATCH /v0/servers/{namespace}/{slug}/status.
// Sets status across the server: active (→ published) | deprecated | deleted (→ private).
func (h *V0MCPHandlers) PatchServerStatus(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	var body mcpwire.StatusPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeV0Error(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	// Map spec status values to domain status.
	var domainStatus domain.Status
	switch body.Status {
	case "active":
		domainStatus = domain.StatusPublished
	case "deprecated":
		domainStatus = domain.StatusDeprecated
	case "deleted":
		// We model "deleted" as setting visibility to private.
		srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, false)
		if errors.Is(err, store.ErrNotFound) {
			writeV0Error(w, http.StatusNotFound, "server not found")
			return
		}
		if err != nil {
			writeV0Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.db.SetMCPServerVisibility(r.Context(), srv.ID, domain.VisibilityPrivate); err != nil {
			writeV0Error(w, http.StatusInternalServerError, "failed to delete server")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		writeV0Error(w, http.StatusUnprocessableEntity, "status must be one of: active, deprecated, deleted")
		return
	}

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.db.SetMCPServerStatus(r.Context(), srv.ID, domainStatus); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeV0Error(w, http.StatusNotFound, "server not found")
			return
		}
		writeV0Error(w, http.StatusInternalServerError, "failed to update status")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── PATCH /v0/servers/{namespace}/{slug}/versions/{version}/status ────────

// PatchVersionStatus serves PATCH /v0/servers/{namespace}/{slug}/versions/{version}/status.
// Sets status on a single version: active | deprecated | deleted.
func (h *V0MCPHandlers) PatchVersionStatus(w http.ResponseWriter, r *http.Request) {
	namespace := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	version := chi.URLParam(r, "version")

	var body mcpwire.StatusPatchRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeV0Error(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	validStatuses := map[string]bool{"active": true, "deprecated": true, "deleted": true}
	if !validStatuses[body.Status] {
		writeV0Error(w, http.StatusUnprocessableEntity, "status must be one of: active, deprecated, deleted")
		return
	}

	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		writeV0Error(w, http.StatusNotFound, "server not found")
		return
	}
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.db.SetMCPVersionStatus(r.Context(), srv.ID, version, domain.VersionStatus(body.Status)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeV0Error(w, http.StatusNotFound, "version not found")
			return
		}
		writeV0Error(w, http.StatusInternalServerError, "failed to update version status")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ── POST /v0/publish ──────────────────────────────────────────────────────

func (h *V0MCPHandlers) Publish(w http.ResponseWriter, r *http.Request) {
	// Per spec, body IS the ServerDetail directly (no wrapper object).
	var p mcpwire.PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeV0Error(w, http.StatusUnprocessableEntity, "invalid JSON body")
		return
	}

	// Required fields: name, version, protocolVersion, description.
	if p.Name == "" || p.Version == "" || p.ProtocolVersion == "" {
		writeV0Error(w, http.StatusUnprocessableEntity,
			"name, version, and protocolVersion are required")
		return
	}
	if p.Description == "" {
		writeV0Error(w, http.StatusUnprocessableEntity,
			"description is required (1-100 chars)")
		return
	}

	// Validate name pattern: ^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$
	if err := domain.ValidateServerName(p.Name); err != nil {
		writeV0Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Parse namespace/slug from the MCP name field.
	parts := strings.SplitN(p.Name, "/", 2)
	namespace, slug := parts[0], parts[1]

	if err := domain.ValidatePackages(p.Packages); err != nil {
		writeV0Error(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	// Resolve or create the MCP server record.
	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		// Auto-create the server if publisher exists.
		publisherID, pubErr := h.db.GetPublisherBySlug(r.Context(), namespace)
		if errors.Is(pubErr, store.ErrNotFound) {
			writeV0Error(w, http.StatusUnprocessableEntity,
				"publisher '"+namespace+"' does not exist")
			return
		}
		if pubErr != nil {
			writeV0Error(w, http.StatusInternalServerError, pubErr.Error())
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
			writeV0Error(w, http.StatusInternalServerError, createErr.Error())
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
		writeV0Error(w, http.StatusInternalServerError, err.Error())
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
		writeV0Error(w, http.StatusConflict,
			"version '"+p.Version+"' already exists for "+p.Name)
		return
	}
	if err != nil {
		writeV0Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.db.PublishMCPServerVersion(r.Context(), srv.ID, ver.Version); err != nil {
		writeV0Error(w, http.StatusInternalServerError, err.Error())
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
	writeJSON(w, http.StatusOK, mcpwire.ServerResponse{
		Server: mcpwire.ToServerDetail(*updatedSrv, publishedVer),
	})
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
