package handlers

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// GrantHandlers serves the role-grant management endpoints.
// Per-publisher grants are gated by publisher Admin (or Server Admin) at the
// router; the global endpoints are Server-Admin only.
type GrantHandlers struct {
	db    *store.DB
	audit store.AuditLogger
}

// NewGrantHandlers builds GrantHandlers with the given store and audit logger.
func NewGrantHandlers(db *store.DB, audit store.AuditLogger) *GrantHandlers {
	return &GrantHandlers{db: db, audit: audit}
}

type grantRequest struct {
	PrincipalType string `json:"principal_type"`
	PrincipalID   string `json:"principal_id"`
	Role          string `json:"role"`
}

// validate checks the request shape and returns a problem detail string when
// invalid (empty when valid).
func (b grantRequest) validate() string {
	if !domain.ValidPrincipalType(b.PrincipalType) {
		return "principal_type must be 'user' or 'group'"
	}
	if b.PrincipalID == "" {
		return "principal_id is required"
	}
	if !domain.ValidRole(b.Role) {
		return "role must be one of viewer, editor, reviewer, admin"
	}
	return ""
}

// principalExists confirms the grant's target user/group exists so a grant is
// never attached to a dangling principal_id (which carries no FK).
func (h *GrantHandlers) principalExists(r *http.Request, b grantRequest) (bool, error) {
	switch domain.PrincipalType(b.PrincipalType) {
	case domain.PrincipalUser:
		_, err := h.db.GetUserByID(r.Context(), b.PrincipalID)
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return err == nil, err
	default: // group
		_, err := h.db.GetGroupByID(r.Context(), b.PrincipalID)
		if errors.Is(err, store.ErrNotFound) {
			return false, nil
		}
		return err == nil, err
	}
}

func (h *GrantHandlers) auditGrant(r *http.Request, action domain.AuditAction, g *store.RoleGrant, publisherSlug string) {
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject,
		ActorEmail:   email,
		Action:       action,
		ResourceType: "role_grant",
		ResourceID:   g.ID,
		ResourceNS:   publisherSlug,
		Metadata: map[string]any{
			"principal_type": string(g.PrincipalType),
			"principal_id":   g.PrincipalID,
			"role":           string(g.Role),
			"publisher":      publisherSlug, // empty for global grants
		},
	})
}

// ── Per-publisher grants ─────────────────────────────────────────────────────

// ListPublisherGrants: GET /api/v1/publishers/{slug}/grants
func (h *GrantHandlers) ListPublisherGrants(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	pub, err := h.db.GetPublisher(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "publisher does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	grants, err := h.db.ListGrantsByPublisher(r.Context(), pub.ID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if grants == nil {
		grants = []store.RoleGrant{}
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": grants})
}

// CreatePublisherGrant: POST /api/v1/publishers/{slug}/grants
func (h *GrantHandlers) CreatePublisherGrant(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	pub, err := h.db.GetPublisher(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "publisher does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	var body grantRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if msg := body.validate(); msg != "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", msg, r.URL.Path)
		return
	}
	ok, err := h.principalExists(r, body)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if !ok {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"principal_id does not reference an existing "+body.PrincipalType, r.URL.Path)
		return
	}

	grant, err := h.db.CreateGrant(r.Context(), store.CreateGrantParams{
		PrincipalType: domain.PrincipalType(body.PrincipalType),
		PrincipalID:   body.PrincipalID,
		PublisherID:   pub.ID,
		Role:          domain.Role(body.Role),
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict", "that grant already exists", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	h.auditGrant(r, domain.ActionRoleGrantCreated, grant, slug)
	writeJSON(w, r, http.StatusCreated, grant)
}

// DeletePublisherGrant: DELETE /api/v1/publishers/{slug}/grants/{id}
func (h *GrantHandlers) DeletePublisherGrant(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	id := chi.URLParam(r, "id")

	pub, err := h.db.GetPublisher(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "publisher does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	grant, err := h.db.GetGrant(r.Context(), id)
	// Scope check: the grant must belong to this publisher. A mismatch is a
	// 404 (the grant is not under this collection) rather than leaking it.
	if errors.Is(err, store.ErrNotFound) || (grant != nil && grant.PublisherID != pub.ID) {
		problem.Write(w, http.StatusNotFound, "not-found", "grant does not exist on this publisher", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.DeleteGrant(r.Context(), id); err != nil {
		internalError(w, r, err)
		return
	}
	h.auditGrant(r, domain.ActionRoleGrantDeleted, grant, slug)
	w.WriteHeader(http.StatusNoContent)
}

// ── Global (all-publishers) grants — Server Admin only ───────────────────────

// ListGlobalGrants: GET /api/v1/grants
func (h *GrantHandlers) ListGlobalGrants(w http.ResponseWriter, r *http.Request) {
	grants, err := h.db.ListGlobalGrants(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	if grants == nil {
		grants = []store.RoleGrant{}
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": grants})
}

// CreateGlobalGrant: POST /api/v1/grants
func (h *GrantHandlers) CreateGlobalGrant(w http.ResponseWriter, r *http.Request) {
	var body grantRequest
	if !decodeJSON(w, r, &body) {
		return
	}
	if msg := body.validate(); msg != "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", msg, r.URL.Path)
		return
	}
	ok, err := h.principalExists(r, body)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if !ok {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"principal_id does not reference an existing "+body.PrincipalType, r.URL.Path)
		return
	}

	grant, err := h.db.CreateGrant(r.Context(), store.CreateGrantParams{
		PrincipalType: domain.PrincipalType(body.PrincipalType),
		PrincipalID:   body.PrincipalID,
		Role:          domain.Role(body.Role),
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict", "that grant already exists", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	h.auditGrant(r, domain.ActionRoleGrantCreated, grant, "")
	writeJSON(w, r, http.StatusCreated, grant)
}

// DeleteGlobalGrant: DELETE /api/v1/grants/{id}
func (h *GrantHandlers) DeleteGlobalGrant(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	grant, err := h.db.GetGrant(r.Context(), id)
	// Only global grants are deletable here — a publisher-scoped grant id is a
	// 404 on this collection.
	if errors.Is(err, store.ErrNotFound) || (grant != nil && grant.PublisherID != "") {
		problem.Write(w, http.StatusNotFound, "not-found", "global grant does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.DeleteGrant(r.Context(), id); err != nil {
		internalError(w, r, err)
		return
	}
	h.auditGrant(r, domain.ActionRoleGrantDeleted, grant, "")
	w.WriteHeader(http.StatusNoContent)
}
