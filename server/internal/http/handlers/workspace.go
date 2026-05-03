package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// WorkspaceHandlers serves the workspace CRUD endpoints. See ADR 0001.
type WorkspaceHandlers struct {
	db    *store.DB
	audit store.AuditLogger
}

// NewWorkspaceHandlers creates WorkspaceHandlers with the given store and
// audit logger.
func NewWorkspaceHandlers(db *store.DB, audit store.AuditLogger) *WorkspaceHandlers {
	return &WorkspaceHandlers{db: db, audit: audit}
}

// ── GET /api/v1/publishers/{publisher_slug}/workspaces ───────────────────────

func (h *WorkspaceHandlers) ListWorkspaces(w http.ResponseWriter, r *http.Request) {
	pubSlug := chi.URLParam(r, "publisher_slug")
	pub, err := h.db.GetPublisher(r.Context(), pubSlug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("publisher '%s' does not exist", pubSlug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	rows, err := h.db.ListWorkspaces(r.Context(), store.ListWorkspacesParams{
		PublisherID: pub.ID,
		Limit:       limit + 1,
		Cursor:      r.URL.Query().Get("cursor"),
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nextCursor = store.EncodeCursor(last.CreatedAt, last.ID)
	}
	if rows == nil {
		rows = []store.Workspace{}
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"items":       rows,
		"next_cursor": nextCursor,
	})
}

// ── GET /api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug} ──────

func (h *WorkspaceHandlers) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	pubSlug := chi.URLParam(r, "publisher_slug")
	wsSlug := chi.URLParam(r, "workspace_slug")

	ws, err := h.db.ResolveWorkspace(r.Context(), pubSlug, wsSlug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("workspace '%s/%s' does not exist", pubSlug, wsSlug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, ws)
}

// ── POST /api/v1/publishers/{publisher_slug}/workspaces ──────────────────────

func (h *WorkspaceHandlers) CreateWorkspace(w http.ResponseWriter, r *http.Request) {
	pubSlug := chi.URLParam(r, "publisher_slug")
	pub, err := h.db.GetPublisher(r.Context(), pubSlug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("publisher '%s' does not exist", pubSlug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	var body struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Contact     string `json:"contact"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Slug == "" || body.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"slug and name are required", r.URL.Path)
		return
	}
	if err := domain.ValidateSlug(body.Slug); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			fmt.Sprintf("slug: %s", err), r.URL.Path)
		return
	}

	ws, err := h.db.CreateWorkspace(r.Context(), store.CreateWorkspaceParams{
		PublisherID: pub.ID,
		Slug:        body.Slug,
		Name:        body.Name,
		Description: body.Description,
		Contact:     body.Contact,
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("workspace '%s/%s' already exists", pubSlug, body.Slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionWorkspaceCreated, ResourceType: "workspace",
		ResourceID: ws.ID, ResourceNS: pubSlug, ResourceSlug: ws.Slug,
	})
	writeJSON(w, r, http.StatusCreated, ws)
}

// ── PATCH /api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug} ────

func (h *WorkspaceHandlers) PatchWorkspace(w http.ResponseWriter, r *http.Request) {
	pubSlug := chi.URLParam(r, "publisher_slug")
	wsSlug := chi.URLParam(r, "workspace_slug")

	ws, err := h.db.ResolveWorkspace(r.Context(), pubSlug, wsSlug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("workspace '%s/%s' does not exist", pubSlug, wsSlug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Contact     *string `json:"contact"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	p := store.UpdateWorkspaceParams{
		Name:        ws.Name,
		Description: ws.Description,
		Contact:     ws.Contact,
	}
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.Description != nil {
		p.Description = *body.Description
	}
	if body.Contact != nil {
		p.Contact = *body.Contact
	}
	if p.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"name is required", r.URL.Path)
		return
	}

	updated, err := h.db.UpdateWorkspace(r.Context(), ws.ID, p)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("workspace '%s/%s' does not exist", pubSlug, wsSlug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionWorkspaceUpdated, ResourceType: "workspace",
		ResourceID: ws.ID, ResourceNS: pubSlug, ResourceSlug: ws.Slug,
	})
	writeJSON(w, r, http.StatusOK, updated)
}

// ── GET /api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug}/servers ──

// ListWorkspaceServers returns the MCP servers under a given workspace.
// Public read; admin not required.
func (h *WorkspaceHandlers) ListWorkspaceServers(w http.ResponseWriter, r *http.Request) {
	pubSlug := chi.URLParam(r, "publisher_slug")
	wsSlug := chi.URLParam(r, "workspace_slug")

	ws, err := h.db.ResolveWorkspace(r.Context(), pubSlug, wsSlug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("workspace '%s/%s' does not exist", pubSlug, wsSlug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	rows, _, err := h.db.ListMCPServers(r.Context(), store.ListMCPServersParams{
		WorkspaceID: ws.ID,
		PublicOnly:  false, // workspace-scoped reads are admin-side; surface drafts.
		Limit:       limit + 1,
		Cursor:      r.URL.Query().Get("cursor"),
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nextCursor = store.EncodeCursor(last.CreatedAt, last.ID)
	}
	if rows == nil {
		rows = []store.MCPServerRow{}
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"items":       rows,
		"next_cursor": nextCursor,
	})
}

// ── GET /api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug}/agents ──

// ListWorkspaceAgents returns the agents under a given workspace.
func (h *WorkspaceHandlers) ListWorkspaceAgents(w http.ResponseWriter, r *http.Request) {
	pubSlug := chi.URLParam(r, "publisher_slug")
	wsSlug := chi.URLParam(r, "workspace_slug")

	ws, err := h.db.ResolveWorkspace(r.Context(), pubSlug, wsSlug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("workspace '%s/%s' does not exist", pubSlug, wsSlug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	rows, _, err := h.db.ListAgents(r.Context(), store.ListAgentsParams{
		WorkspaceID: ws.ID,
		PublicOnly:  false,
		Limit:       limit + 1,
		Cursor:      r.URL.Query().Get("cursor"),
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nextCursor = store.EncodeCursor(last.CreatedAt, last.ID)
	}
	if rows == nil {
		rows = []store.AgentRow{}
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"items":       rows,
		"next_cursor": nextCursor,
	})
}

// ── DELETE /api/v1/publishers/{publisher_slug}/workspaces/{workspace_slug} ───

func (h *WorkspaceHandlers) DeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	pubSlug := chi.URLParam(r, "publisher_slug")
	wsSlug := chi.URLParam(r, "workspace_slug")

	ws, err := h.db.ResolveWorkspace(r.Context(), pubSlug, wsSlug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("workspace '%s/%s' does not exist", pubSlug, wsSlug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	switch err := h.db.DeleteWorkspace(r.Context(), ws.ID); {
	case errors.Is(err, store.ErrConflict):
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("workspace '%s/%s' still has resources", pubSlug, wsSlug), r.URL.Path)
		return
	case errors.Is(err, store.ErrNotFound):
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("workspace '%s/%s' does not exist", pubSlug, wsSlug), r.URL.Path)
		return
	case err != nil:
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionWorkspaceDeleted, ResourceType: "workspace",
		ResourceID: ws.ID, ResourceNS: pubSlug, ResourceSlug: ws.Slug,
	})
	w.WriteHeader(http.StatusNoContent)
}
