package handlers

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// memberEmailParam reads the {email} path segment and URL-decodes it. chi
// returns the raw (still percent-encoded) segment, so an email like
// "a@b.com" arrives as "a%40b.com"; without decoding the server would store
// and match the mangled form. Returns ("", false) when the segment is missing
// or malformed, in which case the caller should answer 422.
func memberEmailParam(r *http.Request) (string, bool) {
	email, err := url.PathUnescape(chi.URLParam(r, "email"))
	if err != nil || email == "" {
		return "", false
	}
	return email, true
}

// GroupHandlers serves the team/group management endpoints.
// All routes are Server-Admin gated at the router.
type GroupHandlers struct {
	db    *store.DB
	audit store.AuditLogger
}

// NewGroupHandlers builds GroupHandlers with the given store and audit logger.
func NewGroupHandlers(db *store.DB, audit store.AuditLogger) *GroupHandlers {
	return &GroupHandlers{db: db, audit: audit}
}

func (h *GroupHandlers) audited(r *http.Request, action domain.AuditAction, id, slug string, meta map[string]any) {
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject,
		ActorEmail:   email,
		Action:       action,
		ResourceType: "group",
		ResourceID:   id,
		ResourceSlug: slug,
		Metadata:     meta,
	})
}

// ListGroups: GET /api/v1/groups
func (h *GroupHandlers) ListGroups(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}
	rows, err := h.db.ListGroups(r.Context(), store.ListGroupsParams{
		Limit:  limit + 1,
		Cursor: r.URL.Query().Get("cursor"),
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
		rows = []store.Group{}
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": rows, "next_cursor": nextCursor})
}

// CreateGroup: POST /api/v1/groups
func (h *GroupHandlers) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Slug == "" || body.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "slug and name are required", r.URL.Path)
		return
	}
	g, err := h.db.CreateGroup(r.Context(), store.CreateGroupParams{
		Slug: body.Slug, Name: body.Name, Description: body.Description,
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict", "a group with that slug already exists", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	h.audited(r, domain.ActionGroupCreated, g.ID, g.Slug, nil)
	writeJSON(w, r, http.StatusCreated, g)
}

// GetGroup: GET /api/v1/groups/{slug}
func (h *GroupHandlers) GetGroup(w http.ResponseWriter, r *http.Request) {
	g, err := h.db.GetGroupBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "group does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, g)
}

// PatchGroup: PATCH /api/v1/groups/{slug}
func (h *GroupHandlers) PatchGroup(w http.ResponseWriter, r *http.Request) {
	g, err := h.db.GetGroupBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "group does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	name := body.Name
	if name == "" {
		name = g.Name // name is required by the column; keep the current one
	}
	updated, err := h.db.UpdateGroup(r.Context(), g.ID, store.UpdateGroupParams{
		Name: name, Description: body.Description,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}
	h.audited(r, domain.ActionGroupUpdated, g.ID, g.Slug, nil)
	writeJSON(w, r, http.StatusOK, updated)
}

// DeleteGroup: DELETE /api/v1/groups/{slug}
func (h *GroupHandlers) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	g, err := h.db.GetGroupBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "group does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.DeleteGroup(r.Context(), g.ID); err != nil {
		internalError(w, r, err)
		return
	}
	h.audited(r, domain.ActionGroupDeleted, g.ID, g.Slug, nil)
	w.WriteHeader(http.StatusNoContent)
}

// ListMembers: GET /api/v1/groups/{slug}/members
func (h *GroupHandlers) ListMembers(w http.ResponseWriter, r *http.Request) {
	g, err := h.db.GetGroupBySlug(r.Context(), chi.URLParam(r, "slug"))
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "group does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	members, err := h.db.ListGroupMembers(r.Context(), g.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if members == nil {
		members = []store.User{}
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": members})
}

// PutMember: PUT /api/v1/groups/{slug}/members/{email}
// Adds the user (by email) to the group, auto-creating an invited user row
// when the email is unknown.
func (h *GroupHandlers) PutMember(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	email, ok := memberEmailParam(r)
	if !ok {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "email is required", r.URL.Path)
		return
	}
	g, err := h.db.GetGroupBySlug(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "group does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	user, err := h.db.GetUserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		// Auto-create an invited user (no subject, no password — cannot log in
		// until a credential is set, but can hold memberships/grants).
		user, err = h.db.CreateUser(r.Context(), store.CreateUserParams{Email: email})
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	if err := h.db.AddGroupMember(r.Context(), g.ID, user.ID); err != nil {
		internalError(w, r, err)
		return
	}
	h.audited(r, domain.ActionGroupMemberAdded, g.ID, g.Slug, map[string]any{"member": user.Email})
	writeJSON(w, r, http.StatusOK, user)
}

// DeleteMember: DELETE /api/v1/groups/{slug}/members/{email}
func (h *GroupHandlers) DeleteMember(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	email, ok := memberEmailParam(r)
	if !ok {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "email is required", r.URL.Path)
		return
	}
	g, err := h.db.GetGroupBySlug(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "group does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	user, err := h.db.GetUserByEmail(r.Context(), email)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "no such member", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.RemoveGroupMember(r.Context(), g.ID, user.ID); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "no such member", r.URL.Path)
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}
	h.audited(r, domain.ActionGroupMemberRemoved, g.ID, g.Slug, map[string]any{"member": user.Email})
	w.WriteHeader(http.StatusNoContent)
}
