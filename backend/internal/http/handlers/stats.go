package handlers

import (
	"net/http"

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
		writeProblem(w, http.StatusInternalServerError, "internal", "failed to fetch stats", r.URL.Path)
		return
	}
	writeJSON(w, http.StatusOK, counts)
}
