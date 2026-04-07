package handlers

import (
	"errors"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/agents"
	"github.com/haibread/ai-registry/internal/store"
)

// AgentCardHandlers serves A2A Agent Card endpoints.
type AgentCardHandlers struct {
	db *store.DB
}

// NewAgentCardHandlers creates AgentCardHandlers.
func NewAgentCardHandlers(db *store.DB) *AgentCardHandlers {
	return &AgentCardHandlers{db: db}
}

// PerAgentCard serves GET /agents/{namespace}/{slug}/.well-known/agent-card.json
// Returns the A2A-compatible agent card for the named agent's latest published version.
func (h *AgentCardHandlers) PerAgentCard(w http.ResponseWriter, r *http.Request) {
	ns := chi.URLParam(r, "namespace")
	slug := chi.URLParam(r, "slug")

	agent, err := h.db.GetAgent(r.Context(), ns, slug, true) // public only
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"agent '"+ns+"/"+slug+"' does not exist or is not public", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	ver, err := h.db.GetLatestPublishedAgentVersion(r.Context(), agent.ID)
	if errors.Is(err, store.ErrNotFound) {
		writeProblem(w, http.StatusNotFound, "not-found",
			"agent '"+ns+"/"+slug+"' has no published version", r.URL.Path)
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal", err.Error(), r.URL.Path)
		return
	}

	card, err := agents.GenerateCard(*agent, ver)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "internal",
			"failed to generate agent card: "+err.Error(), r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, card)
}

// GlobalAgentCard serves GET /.well-known/agent-card.json for the registry itself.
// Makes the registry a first-class A2A citizen.
func GlobalAgentCard(w http.ResponseWriter, r *http.Request) {
	baseURL := os.Getenv("PUBLIC_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8081"
	}
	writeJSON(w, http.StatusOK, agents.RegistryCard(baseURL))
}
