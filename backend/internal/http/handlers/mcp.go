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

// MCPHandlers holds dependencies for MCP registry HTTP handlers.
type MCPHandlers struct {
	db *store.DB
}

// NewMCPHandlers creates an MCPHandlers with the given store.
func NewMCPHandlers(db *store.DB) *MCPHandlers {
	return &MCPHandlers{db: db}
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

	p := store.ListMCPServersParams{
		PublicOnly: !auth.IsAdminFromContext(r.Context()),
		Namespace:  r.URL.Query().Get("namespace"),
		Query:      r.URL.Query().Get("q"),
		Limit:      limit + 1, // fetch one extra to detect next page
		Cursor:     r.URL.Query().Get("cursor"),
	}

	rows, err := h.db.ListMCPServers(r.Context(), p)
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

	type responseItem struct {
		ID          string `json:"id"`
		Namespace   string `json:"namespace"`
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		HomepageURL string `json:"homepage_url,omitempty"`
		RepoURL     string `json:"repo_url,omitempty"`
		License     string `json:"license,omitempty"`
		Visibility  string `json:"visibility"`
		Status      string `json:"status"`
		CreatedAt   string `json:"created_at"`
		UpdatedAt   string `json:"updated_at"`
	}

	items := make([]responseItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, responseItem{
			ID:          r.ID,
			Namespace:   r.Namespace,
			Slug:        r.Slug,
			Name:        r.Name,
			Description: r.Description,
			HomepageURL: r.HomepageURL,
			RepoURL:     r.RepoURL,
			License:     r.License,
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

// ── GET /api/v1/mcp/servers/{namespace}/{slug} ────────────────────────────

func (h *MCPHandlers) GetServer(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"MCP server '"+ns+"/"+slug+"' does not exist", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, serverToResponse(srv, nil))
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
		writeProblem(w, http.StatusConflict, "conflict",
			"MCP server '"+body.Namespace+"/"+body.Slug+"' already exists", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, http.StatusCreated, srv)
}

// ── GET /api/v1/mcp/servers/{namespace}/{slug}/versions ───────────────────

func (h *MCPHandlers) ListVersions(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"MCP server '"+ns+"/"+slug+"' does not exist", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	versions, err := h.db.ListMCPServerVersions(r.Context(), srv.ID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": versions})
}

// ── GET /api/v1/mcp/servers/{namespace}/{slug}/versions/{version} ─────────

func (h *MCPHandlers) GetVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")
	publicOnly := !auth.IsAdminFromContext(r.Context())

	srv, err := h.db.GetMCPServer(r.Context(), ns, slug, publicOnly)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"MCP server '"+ns+"/"+slug+"' does not exist", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	v, err := h.db.GetMCPServerVersion(r.Context(), srv.ID, ver)
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

// ── POST /api/v1/mcp/servers/{namespace}/{slug}/versions ──────────────────

func (h *MCPHandlers) CreateVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

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
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		return
	}
	if body.Version == "" || body.Runtime == "" || body.ProtocolVersion == "" {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error",
			"version, runtime, and protocol_version are required", r.URL.Path)
		return
	}
	if err := domain.ValidatePackages(body.Packages); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}
	if err := domain.ValidateCapabilities(body.Capabilities); err != nil {
		writeProblem(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
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
		writeProblem(w, http.StatusConflict, "conflict",
			"version '"+body.Version+"' already exists for "+ns+"/"+slug, r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, http.StatusCreated, v)
}

// ── POST /api/v1/mcp/servers/{namespace}/{slug}/versions/{version}:publish ─

func (h *MCPHandlers) PublishVersion(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")
	ver := chi.URLParam(r, "version")

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

	if err := h.db.PublishMCPServerVersion(r.Context(), srv.ID, ver); errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"version '"+ver+"' does not exist", r.URL.Path)
		return
	} else if errors.Is(err, store.ErrImmutable) {
		writeProblem(w, http.StatusConflict, "immutable",
			"version '"+ver+"' is already published", r.URL.Path)
		return
	} else if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "published"})
}

// ── POST /api/v1/mcp/servers/{namespace}/{slug}:deprecate ─────────────────

func (h *MCPHandlers) DeprecateServer(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

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

	if err := h.db.DeprecateMCPServer(r.Context(), srv.ID); errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusConflict, "conflict",
			"server '"+ns+"/"+slug+"' is not in published status", r.URL.Path)
		return
	} else if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deprecated"})
}

// ── helper ────────────────────────────────────────────────────────────────

func serverToResponse(srv *store.MCPServerRow, ver *domain.MCPServerVersion) map[string]any {
	m := map[string]any{
		"id":          srv.ID,
		"namespace":   srv.Namespace,
		"slug":        srv.Slug,
		"name":        srv.Name,
		"description": srv.Description,
		"homepage_url": srv.HomepageURL,
		"repo_url":    srv.RepoURL,
		"license":     srv.License,
		"visibility":  string(srv.Visibility),
		"status":      string(srv.Status),
		"created_at":  srv.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		"updated_at":  srv.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if ver != nil {
		m["latest_version"] = ver
	}
	return m
}
