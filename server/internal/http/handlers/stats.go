package handlers

import (
	"net/http"

	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// StatsHandlers serves the registry stats endpoint.
type StatsHandlers struct {
	db *store.DB
}

// NewStatsHandlers creates a StatsHandlers with the given store.
func NewStatsHandlers(db *store.DB) *StatsHandlers {
	return &StatsHandlers{db: db}
}

// GetStats handles GET /api/v1/stats.
// Returns total counts for each resource type. Admin-only so that private
// entries are included in the totals.
func (h *StatsHandlers) GetStats(w http.ResponseWriter, r *http.Request) {
	counts, err := h.db.GetRegistryCounts(r.Context())
	if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal", "failed to fetch stats", r.URL.Path)
		return
	}
	writeJSON(w, r, http.StatusOK, counts)
}

// GetPublicStats handles GET /api/v1/public-stats.
// Returns counts scoped to published + public entries. No auth required.
func (h *StatsHandlers) GetPublicStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetPublicStats(r.Context())
	if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal", "failed to fetch public stats", r.URL.Path)
		return
	}
	writeJSON(w, r, http.StatusOK, stats)
}
