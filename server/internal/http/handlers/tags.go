package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// validateVersionTags normalizes the tag slugs ticked on a new version and
// writes a 422 problem when any of them is not an active instance tag. The
// returned bool reports whether the request may proceed.
func validateVersionTags(w http.ResponseWriter, r *http.Request, db *store.DB, tags []string) ([]string, bool) {
	normalized := domain.NormalizeVersionTags(tags)
	if len(normalized) == 0 {
		return nil, true
	}
	missing, err := db.MissingActiveTagSlugs(r.Context(), normalized)
	if err != nil {
		internalError(w, r, err)
		return nil, false
	}
	if len(missing) > 0 {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			fmt.Sprintf("unknown or inactive tags: %s (the vocabulary is at GET /api/v1/tags)",
				strings.Join(missing, ", ")), r.URL.Path)
		return nil, false
	}
	return normalized, true
}

// TagHandlers holds dependencies for the instance-tag vocabulary endpoints.
type TagHandlers struct {
	db    *store.DB
	audit store.AuditLogger
}

// NewTagHandlers creates a TagHandlers with the given store and audit logger.
func NewTagHandlers(db *store.DB, audit store.AuditLogger) *TagHandlers {
	return &TagHandlers{db: db, audit: audit}
}

// ── GET /api/v1/tags ───────────────────────────────────────────────────────

func (h *TagHandlers) List(w http.ResponseWriter, r *http.Request) {
	tags, err := h.db.ListInstanceTags(r.Context())
	if err != nil {
		internalError(w, r, err)
		return
	}
	if tags == nil {
		tags = []domain.InstanceTag{}
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": tags})
}

// ── POST /api/v1/tags ──────────────────────────────────────────────────────

func (h *TagHandlers) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Color       string `json:"color"`
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
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}
	if body.Color == "" {
		body.Color = "gray"
	}
	if err := domain.ValidateTagColor(body.Color); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}

	t, err := h.db.CreateInstanceTag(r.Context(), store.CreateInstanceTagParams{
		Slug:        body.Slug,
		Name:        body.Name,
		Description: body.Description,
		Color:       body.Color,
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("tag '%s' already exists", body.Slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionInstanceTagCreated, ResourceType: "instance_tag",
		ResourceID: t.ID, ResourceSlug: t.Slug,
		Metadata: map[string]any{"name": t.Name, "color": t.Color},
	})
	writeJSON(w, r, http.StatusCreated, t)
}

// ── PATCH /api/v1/tags/{slug} ──────────────────────────────────────────────

func (h *TagHandlers) Update(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Color       *string `json:"color"`
		Active      *bool   `json:"active"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Name != nil && *body.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"name must not be empty", r.URL.Path)
		return
	}
	if body.Color != nil {
		if err := domain.ValidateTagColor(*body.Color); err != nil {
			problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
			return
		}
	}

	t, err := h.db.UpdateInstanceTag(r.Context(), slug, store.UpdateInstanceTagParams{
		Name:        body.Name,
		Description: body.Description,
		Color:       body.Color,
		Active:      body.Active,
	})
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("tag '%s' does not exist", slug), r.URL.Path)
		return
	}
	if errors.Is(err, store.ErrManagedTag) {
		problem.Write(w, http.StatusConflict, "managed-tag",
			fmt.Sprintf("tag '%s' is defined in the server configuration (INSTANCE_TAGS / instance_tags) and cannot be edited via the API; change it there", slug),
			r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionInstanceTagUpdated, ResourceType: "instance_tag",
		ResourceID: t.ID, ResourceSlug: t.Slug,
		Metadata: map[string]any{"active": t.Active},
	})
	writeJSON(w, r, http.StatusOK, t)
}

// ── DELETE /api/v1/tags/{slug} ─────────────────────────────────────────────

func (h *TagHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	err := h.db.DeleteInstanceTag(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("tag '%s' does not exist", slug), r.URL.Path)
		return
	}
	if errors.Is(err, store.ErrManagedTag) {
		problem.Write(w, http.StatusConflict, "managed-tag",
			fmt.Sprintf("tag '%s' is defined in the server configuration (INSTANCE_TAGS / instance_tags) and cannot be deleted via the API; remove it there", slug),
			r.URL.Path)
		return
	}
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "tag-in-use",
			fmt.Sprintf("tag '%s' is carried by at least one version and cannot be deleted; deactivate it instead", slug),
			r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionInstanceTagDeleted, ResourceType: "instance_tag",
		ResourceSlug: slug,
	})
	w.WriteHeader(http.StatusNoContent)
}
