package handlers

import (
	"net/http"
	"strconv"

	"github.com/haibread/ai-registry/internal/store"
)

// ChangelogHandlers serves the aggregated public changelog.
type ChangelogHandlers struct {
	db *store.DB
}

// NewChangelogHandlers creates a ChangelogHandlers with the given store.
func NewChangelogHandlers(db *store.DB) *ChangelogHandlers {
	return &ChangelogHandlers{db: db}
}

// GetChangelog handles GET /api/v1/changelog.
//
// Query params:
//
//	limit — page size (1–200, default 50)
//
// Returns recently published versions across both registries, merged and
// sorted newest first. Only public, non-deleted entries are included.
func (h *ChangelogHandlers) GetChangelog(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}

	entries, err := h.db.ListChangelog(r.Context(), limit)
	if err != nil {
		internalError(w, r, err)
		return
	}

	type item struct {
		ResourceType string `json:"resource_type"`
		Namespace    string `json:"namespace"`
		Slug         string `json:"slug"`
		Name         string `json:"name"`
		Version      string `json:"version"`
		PublishedAt  string `json:"published_at"`
	}
	items := make([]item, 0, len(entries))
	for _, e := range entries {
		items = append(items, item{
			ResourceType: e.ResourceType,
			Namespace:    e.Namespace,
			Slug:         e.Slug,
			Name:         e.Name,
			Version:      e.Version,
			PublishedAt:  e.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": items})
}
