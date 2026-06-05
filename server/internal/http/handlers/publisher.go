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

// PublisherHandlers serves the publisher registry endpoints.
type PublisherHandlers struct {
	db    *store.DB
	audit store.AuditLogger
}

// NewPublisherHandlers creates PublisherHandlers with the given store and audit logger.
func NewPublisherHandlers(db *store.DB, audit store.AuditLogger) *PublisherHandlers {
	return &PublisherHandlers{db: db, audit: audit}
}

// ── GET /api/v1/publishers ────────────────────────────────────────────────────

func (h *PublisherHandlers) ListPublishers(w http.ResponseWriter, r *http.Request) {
	limit := int32(20)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	rows, err := h.db.ListPublishers(r.Context(), store.ListPublishersParams{
		Limit:  limit + 1,
		Cursor: r.URL.Query().Get("cursor"),
	})
	if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal", "failed to list publishers", r.URL.Path)
		return
	}

	var nextCursor string
	if int32(len(rows)) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		nextCursor = store.EncodeCursor(last.CreatedAt, last.ID)
	}

	if rows == nil {
		rows = []store.Publisher{}
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"items":       rows,
		"next_cursor": nextCursor,
	})
}

// ── GET /api/v1/publishers/{slug} ─────────────────────────────────────────────

func (h *PublisherHandlers) GetPublisher(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	pub, err := h.db.GetPublisher(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("publisher '%s' does not exist", slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, pub)
}

// ── GET /api/v1/publishers/{slug}/stats ───────────────────────────────────────

// GetPublisherStats returns the per-publisher rollup that powers the scoped
// admin home: resource counts + status breakdowns, member counts, and the
// pending-review signal. Gated to a publisher member (Viewer and up) or a
// Server Admin by RequirePublisherRole in the router — so unlike the global
// /stats it does not require Server Admin.
func (h *PublisherHandlers) GetPublisherStats(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	pubID, err := h.db.GetPublisherBySlug(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("publisher '%s' does not exist", slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	stats, err := h.db.GetPublisherStats(r.Context(), pubID)
	if err != nil {
		internalError(w, r, err)
		return
	}
	writeJSON(w, r, http.StatusOK, stats)
}

// ── GET /api/v1/publishers/{slug}/activity ────────────────────────────────────

// PublisherActivityEvent is one entry in a publisher's activity feed. Unlike
// the public per-resource feed (which scrubs actor identity), this members-only
// feed names the actor: collaborators on a publisher can see who did what.
type PublisherActivityEvent struct {
	ID           string         `json:"id"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type"`
	ResourceSlug string         `json:"resource_slug"`
	Version      string         `json:"version,omitempty"`
	ActorEmail   string         `json:"actor_email,omitempty"`
	CreatedAt    string         `json:"created_at"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// GetPublisherActivity returns the publisher-scoped audit feed (newest first,
// paginated). Gated to a publisher member (Viewer and up) or Server Admin in
// the router. Filtered by resource_ns = the publisher's slug, so it covers the
// lifecycle of every MCP server and agent under the publisher. Metadata is run
// through the same scrub as the public feed; the actor's email is retained
// because the audience is the publisher's own members.
func (h *PublisherHandlers) GetPublisherActivity(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	limit := int32(25)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	events, err := h.db.ListAuditEvents(r.Context(), store.ListAuditParams{
		ResourceNS: slug,
		Limit:      limit + 1, // fetch one extra to detect the next page
		Cursor:     r.URL.Query().Get("cursor"),
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	var nextCursor string
	if int32(len(events)) > limit {
		events = events[:limit]
		last := events[len(events)-1]
		nextCursor = store.EncodeCursorFromTime(last.CreatedAt, last.ID)
	}

	items := make([]PublisherActivityEvent, 0, len(events))
	for _, e := range events {
		items = append(items, PublisherActivityEvent{
			ID:           e.ID,
			Action:       string(e.Action),
			ResourceType: e.ResourceType,
			ResourceSlug: e.ResourceSlug,
			Version:      extractVersion(e),
			ActorEmail:   e.ActorEmail,
			CreatedAt:    e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			Metadata:     scrubMetadata(e.Metadata),
		})
	}

	writeJSON(w, r, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	})
}

// ── POST /api/v1/publishers ───────────────────────────────────────────────────

func (h *PublisherHandlers) CreatePublisher(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug    string `json:"slug"`
		Name    string `json:"name"`
		Contact string `json:"contact"`
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

	pub, err := h.db.CreatePublisher(r.Context(), store.CreatePublisherParams{
		Slug:    body.Slug,
		Name:    body.Name,
		Contact: body.Contact,
	})
	if errors.Is(err, store.ErrConflict) {
		problem.Write(w, http.StatusConflict, "conflict",
			fmt.Sprintf("publisher '%s' already exists", body.Slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionPublisherCreated, ResourceType: "publisher",
		ResourceID: pub.ID, ResourceSlug: pub.Slug,
	})
	writeJSON(w, r, http.StatusCreated, pub)
}

// ── PATCH /api/v1/publishers/{slug} ──────────────────────────────────────────

func (h *PublisherHandlers) PatchPublisher(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	pub, err := h.db.GetPublisher(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("publisher '%s' does not exist", slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	var body struct {
		Name    *string `json:"name"`
		Contact *string `json:"contact"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	p := store.UpdatePublisherParams{
		Name:    pub.Name,
		Contact: pub.Contact,
	}
	if body.Name != nil {
		p.Name = *body.Name
	}
	if body.Contact != nil {
		p.Contact = *body.Contact
	}

	if p.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"name is required", r.URL.Path)
		return
	}

	updated, err := h.db.UpdatePublisher(r.Context(), pub.ID, p)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("publisher '%s' does not exist", slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionPublisherUpdated, ResourceType: "publisher",
		ResourceID: pub.ID, ResourceSlug: pub.Slug,
	})
	writeJSON(w, r, http.StatusOK, updated)
}

// ── DELETE /api/v1/publishers/{slug} ─────────────────────────────────────────

func (h *PublisherHandlers) DeletePublisher(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	pub, err := h.db.GetPublisher(r.Context(), slug)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("publisher '%s' does not exist", slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	// Deleting a publisher cascades to all of its MCP servers, agents, their
	// versions, and any reports filed against them (see store.DeletePublisher);
	// owned resources never block the delete, so there is no 409 path here.
	if err := h.db.DeletePublisher(r.Context(), pub.ID); errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("publisher '%s' does not exist", slug), r.URL.Path)
		return
	} else if err != nil {
		internalError(w, r, err)
		return
	}

	subject, email := auditActor(r.Context())
	h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
		ActorSubject: subject, ActorEmail: email,
		Action: domain.ActionPublisherDeleted, ResourceType: "publisher",
		ResourceID: pub.ID, ResourceSlug: pub.Slug,
	})
	w.WriteHeader(http.StatusNoContent)
}
