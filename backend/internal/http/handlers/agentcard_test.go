package handlers_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

func newAgentCardRouter() *chi.Mux {
	h := handlers.NewAgentCardHandlers(testDB, slog.Default())
	r := chi.NewRouter()
	r.Get("/agents/{namespace}/{slug}/.well-known/agent-card.json", h.PerAgentCard)
	return r
}

// seedPublishedAgent creates a public, published agent with a version and returns its slug.
func seedPublishedAgent(t *testing.T, ns, slug string) {
	t.Helper()
	pubID := seedPublisher(t, ns, ns)

	ag, err := testDB.CreateAgent(context.Background(), store.CreateAgentParams{
		PublisherID: pubID,
		Slug:        slug,
		Name:        slug,
		Description: "Test agent",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	if err := testDB.SetAgentVisibility(context.Background(), ag.ID, "public"); err != nil {
		t.Fatalf("SetAgentVisibility: %v", err)
	}

	_, err = testDB.CreateAgentVersion(context.Background(), store.CreateAgentVersionParams{
		AgentID:     ag.ID,
		Version:     "1.0.0",
		EndpointURL: "https://example.com/agent",
		Skills:      validSkills,
	})
	if err != nil {
		t.Fatalf("CreateAgentVersion: %v", err)
	}

	if err := testDB.PublishAgentVersion(context.Background(), ag.ID, "1.0.0"); err != nil {
		t.Fatalf("PublishAgentVersion: %v", err)
	}
}

// ─── PerAgentCard ────────────────────────────────────────────────────────────

func TestAgentCardHandler_PerAgentCard_Found(t *testing.T) {
	resetTables(t)
	seedPublishedAgent(t, "card-ns", "card-ag")

	req := httptest.NewRequest(http.MethodGet,
		"/agents/card-ns/card-ag/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newAgentCardRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	var card struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		URL     string `json:"url"`
		Skills  []struct {
			ID string `json:"id"`
		} `json:"skills"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&card); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if card.Name == "" {
		t.Error("expected non-empty name")
	}
	if card.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", card.Version)
	}
	if card.URL == "" {
		t.Error("expected non-empty url")
	}
	if len(card.Skills) == 0 {
		t.Error("expected at least one skill")
	}
}

func TestAgentCardHandler_PerAgentCard_NotFound(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "card-nf-ns", "card-nf-ns")

	req := httptest.NewRequest(http.MethodGet,
		"/agents/card-nf-ns/nonexistent/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newAgentCardRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAgentCardHandler_PerAgentCard_PrivateAgent(t *testing.T) {
	resetTables(t)
	// Private agent (default) — card not accessible
	seedAgent(t, "card-priv-ns", "priv-ag")

	req := httptest.NewRequest(http.MethodGet,
		"/agents/card-priv-ns/priv-ag/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newAgentCardRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAgentCardHandler_PerAgentCard_NoPublishedVersion(t *testing.T) {
	resetTables(t)
	// Public agent but no published version
	seedAgentPublic(t, "card-nover-ns", "nover-ag")

	req := httptest.NewRequest(http.MethodGet,
		"/agents/card-nover-ns/nover-ag/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newAgentCardRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
