package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/agents"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// AgentCardHandlers serves A2A Agent Card endpoints.
type AgentCardHandlers struct {
	db     *store.DB
	logger *slog.Logger
}

// NewAgentCardHandlers creates AgentCardHandlers.
func NewAgentCardHandlers(db *store.DB, logger *slog.Logger) *AgentCardHandlers {
	return &AgentCardHandlers{db: db, logger: logger}
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
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	ver, err := h.db.GetLatestPublishedAgentVersion(r.Context(), agent.ID)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusNotFound, "not-found",
			fmt.Sprintf("agent '%s/%s' has no published version", ns, slug), r.URL.Path)
		return
	}
	if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	card, err := agents.GenerateCard(*agent, ver)
	if err != nil {
		problem.Write(w, http.StatusInternalServerError, "internal",
			fmt.Sprintf("failed to generate agent card: %s", err.Error()), r.URL.Path)
		return
	}

	writeJSON(w, r, http.StatusOK, card)
}

// GlobalAgentCard serves GET /.well-known/agent-card.json for the registry itself.
// Makes the registry a first-class A2A citizen.
//
// PUBLIC_BASE_URL must be set in production. If it is unset, this handler
// returns HTTP 500 so misconfigured deployments fail loudly rather than
// silently advertising a localhost URL to external consumers.
func (h *AgentCardHandlers) GlobalAgentCard(w http.ResponseWriter, r *http.Request) {
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		h.logger.ErrorContext(r.Context(),
			"PUBLIC_BASE_URL is not set; cannot generate a valid global agent card",
		)
		problem.Write(w, http.StatusInternalServerError, "misconfiguration",
			"PUBLIC_BASE_URL environment variable is not set", r.URL.Path)
		return
	}
	writeJSON(w, r, http.StatusOK, agents.RegistryCard(baseURL))
}
