package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}
	writeJSON(w, r, http.StatusOK, pub)
}

// ── POST /api/v1/publishers ───────────────────────────────────────────────────

func (h *PublisherHandlers) CreatePublisher(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Slug    string `json:"slug"`
		Name    string `json:"name"`
		Contact string `json:"contact"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		return
	}
	if body.Slug == "" || body.Name == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"slug and name are required", r.URL.Path)
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	if claims, ok := auth.ClaimsFromContext(r.Context()); ok {
		h.audit.LogAuditEvent(r.Context(), domain.AuditEvent{
			ActorSubject: claims.Subject, ActorEmail: claims.Email,
			Action: domain.ActionPublisherCreated, ResourceType: "publisher",
			ResourceID: pub.ID, ResourceSlug: pub.Slug,
		})
	}
	writeJSON(w, r, http.StatusCreated, pub)
}
