package handlers

import (
	"net/http"
	"strconv"

	"github.com/haibread/ai-registry/internal/store"
)

// AuditHandlers serves the audit log API.
type AuditHandlers struct {
	db *store.DB
}

// NewAuditHandlers creates an AuditHandlers with the given store.
func NewAuditHandlers(db *store.DB) *AuditHandlers {
	return &AuditHandlers{db: db}
}

// ListEvents handles GET /api/v1/audit.
//
// Query params:
//
//	resource_type — filter by resource type ("mcp_server", "agent", "publisher")
//	resource_id   — filter by resource ULID
//	actor         — filter by Keycloak subject UUID
//	limit         — page size (1-100, default 50)
//	cursor        — opaque pagination cursor
func (h *AuditHandlers) ListEvents(w http.ResponseWriter, r *http.Request) {
	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = int32(n)
		}
	}

	p := store.ListAuditParams{
		ResourceType: r.URL.Query().Get("resource_type"),
		ResourceID:   r.URL.Query().Get("resource_id"),
		ActorSubject: r.URL.Query().Get("actor"),
		Limit:        limit + 1, // fetch one extra to detect next page
		Cursor:       r.URL.Query().Get("cursor"),
	}

	events, err := h.db.ListAuditEvents(r.Context(), p)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", "failed to list audit events", r.URL.Path)
		return
	}

	type item struct {
		ID           string         `json:"id"`
		ActorSubject string         `json:"actor_subject"`
		ActorEmail   string         `json:"actor_email"`
		Action       string         `json:"action"`
		ResourceType string         `json:"resource_type"`
		ResourceID   string         `json:"resource_id"`
		ResourceNS   string         `json:"resource_ns,omitempty"`
		ResourceSlug string         `json:"resource_slug,omitempty"`
		Metadata     map[string]any `json:"metadata,omitempty"`
		CreatedAt    string         `json:"created_at"`
	}

	var nextCursor string
	if int32(len(events)) > limit {
		events = events[:limit]
		last := events[len(events)-1]
		nextCursor = store.EncodeCursorFromTime(last.CreatedAt, last.ID)
	}

	items := make([]item, 0, len(events))
	for _, e := range events {
		items = append(items, item{
			ID:           e.ID,
			ActorSubject: e.ActorSubject,
			ActorEmail:   e.ActorEmail,
			Action:       string(e.Action),
			ResourceType: e.ResourceType,
			ResourceID:   e.ResourceID,
			ResourceNS:   e.ResourceNS,
			ResourceSlug: e.ResourceSlug,
			Metadata:     e.Metadata,
			CreatedAt:    e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"next_cursor": nextCursor,
	})
}
