package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/santhosh-tekuri/jsonschema/v5"

	"github.com/haibread/ai-registry/api"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

func newAgentCardRouter() *chi.Mux {
	h := handlers.NewAgentCardHandlers(testDB, slog.Default())
	r := chi.NewRouter()
	r.Get("/agents/{namespace}/{slug}/.well-known/agent-card.json", h.PerAgentCard)
	r.Get("/.well-known/agent-card.json", h.GlobalAgentCard)
	return r
}

// ─── A2A JSON Schema conformance ────────────────────────────────────────────
//
// CLAUDE.md Resolved Decision G pins the A2A Agent Card shape to the
// a2a-project/a2a June 2025 commit. The schema file at
// server/api/a2a-agent-card.schema.json is the machine-checkable version of
// that decision — any regression in internal/agents/card.go that drops a
// required field, mis-types a capability flag, or emits a security scheme
// outside the CLAUDE.md decision-K allow-list will fail validation here.

var (
	a2aSchemaOnce sync.Once
	a2aSchema     *jsonschema.Schema
	a2aSchemaErr  error
)

// loadA2ASchema compiles the embedded A2A Agent Card JSON Schema exactly once
// per test binary. Compilation failures (malformed schema) are surfaced as a
// test failure on first use rather than at package init so a broken schema
// does not wedge unrelated tests.
func loadA2ASchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	a2aSchemaOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		const url = "a2a-agent-card.schema.json"
		if err := compiler.AddResource(url, bytes.NewReader(api.A2AAgentCardSchema)); err != nil {
			a2aSchemaErr = err
			return
		}
		a2aSchema, a2aSchemaErr = compiler.Compile(url)
	})
	if a2aSchemaErr != nil {
		t.Fatalf("compiling embedded A2A schema: %v", a2aSchemaErr)
	}
	return a2aSchema
}

// assertA2AConformant decodes body into a generic map and validates it against
// the pinned A2A Agent Card schema. On failure it reports the schema error
// alongside the body so test output is actionable without re-running with -v.
func assertA2AConformant(t *testing.T, body []byte) {
	t.Helper()
	schema := loadA2ASchema(t)

	var doc any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("A2A card body is not valid JSON: %v\nbody: %s", err, string(body))
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("A2A schema violation:\n%v\nbody: %s", err, string(body))
	}
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

// TestAgentCardHandler_PerAgentCard_A2AConformance drives a freshly-seeded
// published agent through the real handler and asserts the response body
// conforms to the pinned A2A JSON Schema. This is the test that catches the
// class of bug where GenerateCard drops a required field (e.g. defaultInputModes
// becomes nil and marshals as `null`) or silently mis-types a capability.
func TestAgentCardHandler_PerAgentCard_A2AConformance(t *testing.T) {
	resetTables(t)
	seedPublishedAgent(t, "conf-ns", "conf-ag")

	req := httptest.NewRequest(http.MethodGet,
		"/agents/conf-ns/conf-ag/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newAgentCardRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	assertA2AConformant(t, rec.Body.Bytes())
}

// ─── GlobalAgentCard ────────────────────────────────────────────────────────

// TestAgentCardHandler_GlobalAgentCard_A2AConformance proves the registry's
// own agent card (served at /.well-known/agent-card.json) is A2A-compliant
// too. Per CLAUDE.md Resolved Decision H, the global card makes the registry
// a first-class A2A citizen — an invalid card here would break discovery
// clients that crawl the registry via A2A.
func TestAgentCardHandler_GlobalAgentCard_A2AConformance(t *testing.T) {
	resetTables(t)
	t.Setenv("PUBLIC_BASE_URL", "https://registry.example.test")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newAgentCardRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	assertA2AConformant(t, rec.Body.Bytes())

	// Also verify the URL field points at the registry we configured, not a
	// stale default. A card that advertises the wrong URL is actively
	// dangerous: it misroutes every A2A client that caches it.
	var card map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &card); err != nil {
		t.Fatalf("decode: %v", err)
	}
	wantPrefix := "https://registry.example.test/"
	if url, _ := card["url"].(string); !strings.HasPrefix(url, wantPrefix) {
		t.Errorf("global card url = %q, want prefix %q", url, wantPrefix)
	}
}

// TestAgentCardHandler_GlobalAgentCard_MisconfiguredReturns500 guards the
// documented behaviour that an unset PUBLIC_BASE_URL MUST fail loud rather
// than silently advertising a localhost URL to external consumers. The
// handler's comment explicitly promises this — if somebody changes it to fall
// back to "http://localhost:8081" the regression is invisible in prod until
// an external A2A crawler tries to dial it.
func TestAgentCardHandler_GlobalAgentCard_MisconfiguredReturns500(t *testing.T) {
	resetTables(t)
	// t.Setenv with empty string explicitly unsets for the duration of the
	// test, regardless of what the ambient environment looks like.
	t.Setenv("PUBLIC_BASE_URL", "")

	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	newAgentCardRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body: %s", rec.Code, rec.Body.String())
	}
	// Problem+JSON response shape — the handler writes via problem.Write.
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "problem+json") {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var prob map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	// problem.Write puts the slug into the `type` URL and the caller-supplied
	// message into `detail`; `title` is the generic HTTP status text.
	typeField, _ := prob["type"].(string)
	if !strings.Contains(typeField, "misconfiguration") {
		t.Errorf("problem.type = %q, want it to contain 'misconfiguration'", typeField)
	}
	if detail, _ := prob["detail"].(string); !strings.Contains(detail, "PUBLIC_BASE_URL") {
		t.Errorf("problem.detail = %q, want it to mention PUBLIC_BASE_URL", detail)
	}
	if status, _ := prob["status"].(float64); int(status) != http.StatusInternalServerError {
		t.Errorf("problem.status = %v, want 500", status)
	}
}

// ─── Unit-level card generation conformance ─────────────────────────────────

// TestRegistryCard_A2AConformance runs the unit-level RegistryCard constructor
// through the schema without going through the HTTP layer. This is a
// cheap-and-fast smoke test that lives alongside the handler tests because
// the schema loader is here; a failure here is a direct bug in
// internal/agents/card.go:RegistryCard.
func TestRegistryCard_A2AConformance(t *testing.T) {
	// Import cycle avoidance: we reach RegistryCard via JSON-round-trip of
	// the global endpoint in the handler-level test above, so this unit test
	// just re-invokes the handler and asserts the same thing from a different
	// entry point. Keeping it separate makes the failure message narrower
	// when only the shape is wrong (no DB, no routing).
	t.Setenv("PUBLIC_BASE_URL", "https://registry.example.test")

	h := handlers.NewAgentCardHandlers(testDB, slog.Default())
	req := httptest.NewRequest(http.MethodGet, "/.well-known/agent-card.json", nil)
	rec := httptest.NewRecorder()
	h.GlobalAgentCard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	assertA2AConformant(t, rec.Body.Bytes())
}
