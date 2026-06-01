package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// UserHandlers serves the user/principal management endpoints.
// List/create/get/patch are Server-Admin gated at the router; set-password is
// authenticated and self-or-admin (enforced in the handler).
type UserHandlers struct {
	db    *store.DB
	audit store.AuditLogger
}

// NewUserHandlers builds UserHandlers with the given store and audit logger.
func NewUserHandlers(db *store.DB, audit store.AuditLogger) *UserHandlers {
	return &UserHandlers{db: db, audit: audit}
}

func (h *UserHandlers) audited(r *http.Request, action domain.AuditAction, u *store.User) {
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject,
		ActorEmail:   email,
		Action:       action,
		ResourceType: "user",
		ResourceID:   u.ID,
		Metadata:     map[string]any{"target_email": u.Email},
	})
}

// ListUsers: GET /api/v1/users
func (h *UserHandlers) ListUsers(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}
	rows, err := h.db.ListUsers(r.Context(), store.ListUsersParams{
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
		rows = []store.User{}
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": rows, "next_cursor": nextCursor})
}

// CreateUser: POST /api/v1/users — creates a local or invited user.
func (h *UserHandlers) CreateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email         string `json:"email"`
		DisplayName   string `json:"display_name"`
		Password      string `json:"password"`
		IsServerAdmin bool   `json:"is_server_admin"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Email == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "email is required", r.URL.Path)
		return
	}

	var hash string
	if body.Password != "" {
		var err error
		hash, err = auth.HashPassword(body.Password)
		if err != nil {
			internalError(w, r, err)
			return
		}
	}

	u, err := h.db.CreateUser(r.Context(), store.CreateUserParams{
		Email:         body.Email,
		DisplayName:   body.DisplayName,
		PasswordHash:  hash,
		IsServerAdmin: body.IsServerAdmin,
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict", "a user with that email already exists", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	h.audited(r, domain.ActionUserCreated, u)
	writeJSON(w, r, http.StatusCreated, u)
}

// GetUser: GET /api/v1/users/{id}
func (h *UserHandlers) GetUser(w http.ResponseWriter, r *http.Request) {
	u, err := h.db.GetUserByID(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "user does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, u)
}

// PatchUser: PATCH /api/v1/users/{id} — display name, disabled, is_server_admin.
func (h *UserHandlers) PatchUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		DisplayName   *string `json:"display_name"`
		Disabled      *bool   `json:"disabled"`
		IsServerAdmin *bool   `json:"is_server_admin"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	u, err := h.db.UpdateUser(r.Context(), id, store.UpdateUserParams{
		DisplayName:   body.DisplayName,
		Disabled:      body.Disabled,
		IsServerAdmin: body.IsServerAdmin,
	})
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "user does not exist", r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	h.audited(r, domain.ActionUserUpdated, u)
	writeJSON(w, r, http.StatusOK, u)
}

// SetPassword: POST /api/v1/users/{id}/set-password — self or Server Admin.
// Not RequireAdmin-gated at the router so a user can set their own password;
// the handler enforces the self-or-admin rule.
func (h *UserHandlers) SetPassword(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok || claims == nil {
		problem.Write(w, http.StatusUnauthorized, "unauthorized", "Missing or invalid bearer token", r.URL.Path)
		return
	}
	isAdmin := auth.IsServerAdminFromContext(r.Context())
	isSelf := false
	if p, ok := auth.PrincipalFromContext(r.Context()); ok && p != nil {
		isSelf = p.UserID == id
	}
	if !isAdmin && !isSelf {
		problem.Write(w, http.StatusForbidden, "forbidden",
			"You may only set your own password unless you are a Server Admin.", r.URL.Path)
		return
	}

	var body struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if len(body.Password) < 8 {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"password must be at least 8 characters", r.URL.Path)
		return
	}

	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if err := h.db.SetPasswordHash(r.Context(), id, hash); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found", "user does not exist", r.URL.Path)
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}
	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject,
		ActorEmail:   email,
		Action:       domain.ActionUserPasswordSet,
		ResourceType: "user",
		ResourceID:   id,
		Metadata:     map[string]any{"self_service": isSelf && !isAdmin},
	})
	w.WriteHeader(http.StatusNoContent)
}
