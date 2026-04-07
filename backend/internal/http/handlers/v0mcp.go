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

// ── GET /v0/servers ───────────────────────────────────────────────────────

func (h *V0MCPHandlers) ListServers(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	rows, err := h.db.ListMCPServers(r.Context(), store.ListMCPServersParams{
		PublicOnly: true,
		Query:      r.URL.Query().Get("q"),
		Limit:      limit + 1,
		Cursor:     r.URL.Query().Get("cursor"),
	})
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "failed to list servers", r.URL.Path)
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
		writeProblem(w, http.StatusNotFound, "not-found", "server not found", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	if srv.Visibility != domain.VisibilityPublic {
		writeProblem(w, http.StatusNotFound, "not-found", "server not found", r.URL.Path)
		return
	}

	ver, _ := h.db.GetLatestPublishedVersion(r.Context(), srv.ID)
	writeJSON(w, http.StatusOK, mcpwire.DetailResponse{
		Server: mcpwire.ToServerDetail(*srv, ver),
	})
}

// ── POST /v0/publish ──────────────────────────────────────────────────────

func (h *V0MCPHandlers) Publish(w http.ResponseWriter, r *http.Request) {
	var body mcpwire.PublishRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		return
	}

	p := body.Server
	if p.Name == "" || p.Version == "" || p.ProtocolVersion == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error",
			"server.name, server.version, and server.protocolVersion are required", r.URL.Path)
		return
	}

	// Parse namespace/slug from the MCP name field.
	parts := strings.SplitN(p.Name, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error",
			"server.name must be in the format 'namespace/slug'", r.URL.Path)
		return
	}
	namespace, slug := parts[0], parts[1]

	if err := domain.ValidatePackages(p.Packages); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}

	// Resolve or create the MCP server record.
	srv, err := h.db.GetMCPServer(r.Context(), namespace, slug, false)
	if errors.Is(err, store.ErrNotFound) {
		// Auto-create the server if publisher exists.
		publisherID, pubErr := h.db.GetPublisherBySlug(r.Context(), namespace)
		if errors.Is(pubErr, store.ErrNotFound) {
			writeProblem(w, http.StatusUnprocessableEntity, "validation-error",
				"publisher '"+namespace+"' does not exist", r.URL.Path)
			return
		}
		if pubErr != nil {
			writeProblem(w, http.StatusInternalServerError, "internal", pubErr.Error(), r.URL.Path)
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
			writeProblem(w, http.StatusInternalServerError, "internal", createErr.Error(), r.URL.Path)
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
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
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
		writeProblem(w, http.StatusConflict, "conflict",
			"version '"+p.Version+"' already exists for "+p.Name, r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	if err := h.db.PublishMCPServerVersion(r.Context(), srv.ID, ver.Version); err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
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

	writeJSON(w, http.StatusCreated, map[string]string{
		"message": "Server " + p.Name + " version " + p.Version + " published successfully.",
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
