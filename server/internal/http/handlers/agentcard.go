package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/agents"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// AgentCardHandlers serves A2A Agent Card endpoints.
type AgentCardHandlers struct {
	db            *store.DB
	logger        *slog.Logger
	publicBaseURL string
}

// NewAgentCardHandlers creates AgentCardHandlers. publicBaseURL comes from
// the config layer (env + YAML + default per CLAUDE.md); empty triggers a
// 500 in GlobalAgentCard rather than silently advertising localhost.
func NewAgentCardHandlers(db *store.DB, logger *slog.Logger, publicBaseURL string) *AgentCardHandlers {
	return &AgentCardHandlers{db: db, logger: logger, publicBaseURL: publicBaseURL}
}

// PerAgentCard serves GET /agents/{namespace}/{slug}/.well-known/agent-card.json
// Returns the A2A-compatible agent card for the named agent's latest published version.
func (h *AgentCardHandlers) PerAgentCard(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	agent, err := h.db.GetAgent(r.Context(), ns, slug, true) // public only
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' does not exist or is not public", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	ver, err := h.db.GetLatestPublishedAgentVersion(r.Context(), agent.ID)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' has no published version", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}

	card, err := agents.GenerateCard(*agent, ver)
	if err != nil {
		internalError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, card)
}

// GlobalAgentCard serves GET /.well-known/agent-card.json for the registry itself.
// Makes the registry a first-class A2A citizen.
//
// PublicBaseURL (config layer: env + YAML + default) must be set in production.
// If it is empty, this handler returns HTTP 500 so misconfigured deployments
// fail loudly rather than silently advertising a localhost URL to external
// consumers.
func (h *AgentCardHandlers) GlobalAgentCard(w http.ResponseWriter, r *http.Request) {
	if h.publicBaseURL == "" {
		h.logger.ErrorContext(r.Context(),
			"PUBLIC_BASE_URL is not set; cannot generate a valid global agent card",
		)
		problem.Write(w, http.StatusInternalServerError, "misconfiguration",
			"PUBLIC_BASE_URL is not set", r.URL.Path)
		return
	}
	writeJSON(w, r, http.StatusOK, agents.RegistryCard(h.publicBaseURL))
}
