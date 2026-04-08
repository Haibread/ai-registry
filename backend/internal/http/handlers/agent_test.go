package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

// validSkills is a well-formed skills JSON used in agent version tests.
var validSkills = json.RawMessage(`[{"id":"s1","name":"Search","description":"Searches things","tags":["search"]}]`)

func newAgentRouter() *chi.Mux {
	h := handlers.NewAgentHandlers(testDB, testDB, nil)
	r := chi.NewRouter()
	r.Get("/api/v1/agents", h.ListAgents)
	r.Post("/api/v1/agents", h.CreateAgent)
	r.Get("/api/v1/agents/{namespace}/{slug}", h.GetAgent)
	r.Get("/api/v1/agents/{namespace}/{slug}/versions", h.ListVersions)
	r.Post("/api/v1/agents/{namespace}/{slug}/versions", h.CreateVersion)
	r.Get("/api/v1/agents/{namespace}/{slug}/versions/{version}", h.GetVersion)
	r.Post("/api/v1/agents/{namespace}/{slug}/versions/{version}/publish", h.PublishVersion)
	r.Post("/api/v1/agents/{namespace}/{slug}/deprecate", h.DeprecateAgent)
	r.Post("/api/v1/agents/{namespace}/{slug}/visibility", h.SetVisibility)
	return r
}

// adminAgentCtx returns a context with admin claims for agent tests.
func adminAgentCtx() context.Context {
	return auth.ContextWithClaims(context.Background(), &auth.KeycloakClaims{
		RealmAccess: auth.RealmAccess{Roles: []string{"admin"}},
	})
}

// seedAgent creates a publisher + agent row and returns the agent.
func seedAgent(t *testing.T, ns, slug string) {
	t.Helper()
	pubID := seedPublisher(t, ns, ns)
	_, err := testDB.CreateAgent(context.Background(), store.CreateAgentParams{
		PublisherID: pubID,
		Slug:        slug,
		Name:        slug,
	})
	if err != nil {
		t.Fatalf("seedAgent(%q/%q): %v", ns, slug, err)
	}
}

// seedAgentPublic creates a public agent.
func seedAgentPublic(t *testing.T, ns, slug string) {
	t.Helper()
	seedAgent(t, ns, slug)
	ag, err := testDB.GetAgent(context.Background(), ns, slug, false)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if err := testDB.SetAgentVisibility(context.Background(), ag.ID, "public"); err != nil {
		t.Fatalf("SetAgentVisibility: %v", err)
	}
}

// createAgentVersion inserts a draft agent version.
func createAgentVersion(t *testing.T, ns, slug, ver string) {
	t.Helper()
	ag, err := testDB.GetAgent(context.Background(), ns, slug, false)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	_, err = testDB.CreateAgentVersion(context.Background(), store.CreateAgentVersionParams{
		AgentID:     ag.ID,
		Version:     ver,
		EndpointURL: "https://example.com/agent",
		Skills:      validSkills,
	})
	if err != nil {
		t.Fatalf("CreateAgentVersion: %v", err)
	}
}

// ─── ListAgents ─────────────────────────────────────────────────────────────

func TestAgentHandler_ListAgents_EmptyList(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []any `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Items == nil {
		t.Error("items should be empty array, not null")
	}
	if len(body.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(body.Items))
	}
}

func TestAgentHandler_ListAgents_WithAgents(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-pub", "agent-a")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(body.Items))
	}
}

func TestAgentHandler_ListAgents_PublicOnlyFilter(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-vis1", "private-ag")         // private by default
	seedAgentPublic(t, "ag-vis2", "public-ag")

	// Unauthenticated — sees only public
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	var body struct {
		Items []struct {
			Slug string `json:"slug"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 1 {
		t.Errorf("public-only: expected 1 item, got %d", len(body.Items))
	}
	if body.Items[0].Slug != "public-ag" {
		t.Errorf("expected public-ag, got %q", body.Items[0].Slug)
	}

	// Admin — sees all
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents", nil)
	req = req.WithContext(adminAgentCtx())
	rec = httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("admin: expected 2 items, got %d", len(body.Items))
	}
}

// ─── GetAgent ───────────────────────────────────────────────────────────────

func TestAgentHandler_GetAgent_Found(t *testing.T) {
	resetTables(t)
	seedAgentPublic(t, "ag-get", "my-agent")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/ag-get/my-agent", nil)
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Slug string `json:"slug"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Slug != "my-agent" {
		t.Errorf("slug = %q, want my-agent", body.Slug)
	}
}

func TestAgentHandler_GetAgent_NotFound(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "ag-nf", "ag-nf")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/ag-nf/nope", nil)
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAgentHandler_GetAgent_PrivateHiddenFromPublic(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-priv", "hidden-ag") // private by default

	// Unauthenticated
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/ag-priv/hidden-ag", nil)
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("unauthenticated: status = %d, want 404", rec.Code)
	}

	// Admin can see it
	req = httptest.NewRequest(http.MethodGet, "/api/v1/agents/ag-priv/hidden-ag", nil)
	req = req.WithContext(adminAgentCtx())
	rec = httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("admin: status = %d, want 200", rec.Code)
	}
}

// ─── CreateAgent ────────────────────────────────────────────────────────────

func TestAgentHandler_CreateAgent_Valid(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "ag-create-ns", "Create NS")

	payload := `{"namespace":"ag-create-ns","slug":"new-ag","name":"New Agent"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		ID   string `json:"id"`
		Slug string `json:"slug"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.ID == "" {
		t.Error("expected non-empty id")
	}
	if body.Slug != "new-ag" {
		t.Errorf("slug = %q, want new-ag", body.Slug)
	}
}

func TestAgentHandler_CreateAgent_MissingFields(t *testing.T) {
	resetTables(t)

	tests := []struct {
		name    string
		payload string
	}{
		{"missing namespace", `{"slug":"s","name":"N"}`},
		{"missing slug", `{"namespace":"ns","name":"N"}`},
		{"missing name", `{"namespace":"ns","slug":"s"}`},
		{"invalid JSON", `{bad}`},
	}

	r := newAgentRouter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agents",
				bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422", rec.Code)
			}
		})
	}
}

func TestAgentHandler_CreateAgent_DuplicateSlug(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "ag-dup-ns", "Dup NS")

	payload := `{"namespace":"ag-dup-ns","slug":"dup-ag","name":"Dup"}`
	post := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents",
			bytes.NewBufferString(payload))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		newAgentRouter().ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post(); code != http.StatusCreated {
		t.Fatalf("first create: %d", code)
	}
	if code := post(); code != http.StatusConflict {
		t.Errorf("duplicate: %d, want 409", code)
	}
}

// ─── ListVersions ───────────────────────────────────────────────────────────

func TestAgentHandler_ListVersions_Empty(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-lv", "ag-lv")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/ag-lv/ag-lv/versions", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []any `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 0 {
		t.Errorf("expected 0 versions, got %d", len(body.Items))
	}
}

func TestAgentHandler_ListVersions_WithVersions(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-lvv", "ag-lvv")
	createAgentVersion(t, "ag-lvv", "ag-lvv", "1.0.0")
	createAgentVersion(t, "ag-lvv", "ag-lvv", "2.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/ag-lvv/ag-lvv/versions", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []struct {
			Version string `json:"version"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("expected 2 versions, got %d", len(body.Items))
	}
}

// ─── GetVersion ─────────────────────────────────────────────────────────────

func TestAgentHandler_GetVersion_Found(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-gv", "ag-gv")
	createAgentVersion(t, "ag-gv", "ag-gv", "1.0.0")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/ag-gv/ag-gv/versions/1.0.0", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Version string `json:"version"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", body.Version)
	}
}

func TestAgentHandler_GetVersion_NotFound(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-gvnf", "ag-gvnf")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents/ag-gvnf/ag-gvnf/versions/99.0.0", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ─── CreateVersion ──────────────────────────────────────────────────────────

func TestAgentHandler_CreateVersion_Valid(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-cv", "ag-cv")

	body := map[string]any{
		"version":      "1.0.0",
		"endpoint_url": "https://example.com/agent",
		"skills":       validSkills,
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-cv/ag-cv/versions",
		bytes.NewBuffer(b))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Version string `json:"version"`
	}
	json.NewDecoder(rec.Body).Decode(&resp) //nolint:errcheck
	if resp.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", resp.Version)
	}
}

func TestAgentHandler_CreateVersion_MissingFields(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-cvmf", "ag-cvmf")

	tests := []struct {
		name    string
		payload string
	}{
		{"missing version", `{"endpoint_url":"https://example.com","skills":[{"id":"s1","name":"S","description":"D","tags":[]}]}`},
		{"missing endpoint_url", `{"version":"1.0.0","skills":[{"id":"s1","name":"S","description":"D","tags":[]}]}`},
	}

	r := newAgentRouter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-cvmf/ag-cvmf/versions",
				bytes.NewBufferString(tt.payload))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(adminAgentCtx())
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("%s: status = %d, want 422", tt.name, rec.Code)
			}
		})
	}
}

func TestAgentHandler_CreateVersion_InvalidSkills(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-cvbs", "ag-cvbs")

	// Skills missing required fields
	payload := `{"version":"1.0.0","endpoint_url":"https://example.com","skills":[{"id":"","name":"","description":"","tags":[]}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-cvbs/ag-cvbs/versions",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestAgentHandler_CreateVersion_InvalidAuthentication(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-cvba", "ag-cvba")

	// Auth with invalid scheme
	payload := `{"version":"1.0.0","endpoint_url":"https://example.com","skills":[{"id":"s1","name":"S","description":"D","tags":[]}],"authentication":[{"scheme":"UnknownScheme"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-cvba/ag-cvba/versions",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

// ─── PublishVersion ─────────────────────────────────────────────────────────

func TestAgentHandler_PublishVersion_Success(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-pv", "ag-pv")
	createAgentVersion(t, "ag-pv", "ag-pv", "1.0.0")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-pv/ag-pv/versions/1.0.0/publish", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body["status"] != "published" {
		t.Errorf("status = %q, want published", body["status"])
	}
}

func TestAgentHandler_PublishVersion_NotFound(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-pvnf", "ag-pvnf")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-pvnf/ag-pvnf/versions/99.0.0/publish", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestAgentHandler_PublishVersion_AlreadyPublished(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-pvap", "ag-pvap")
	createAgentVersion(t, "ag-pvap", "ag-pvap", "1.0.0")

	r := newAgentRouter()

	publish := func() int {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-pvap/ag-pvap/versions/1.0.0/publish", nil)
		req = req.WithContext(adminAgentCtx())
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := publish(); code != http.StatusOK {
		t.Fatalf("first publish: %d", code)
	}
	if code := publish(); code != http.StatusConflict {
		t.Errorf("second publish: %d, want 409", code)
	}
}

// ─── DeprecateAgent ─────────────────────────────────────────────────────────

func TestAgentHandler_DeprecateAgent_Success(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-dep", "ag-dep")
	createAgentVersion(t, "ag-dep", "ag-dep", "1.0.0")

	r := newAgentRouter()

	// Publish first
	pubReq := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-dep/ag-dep/versions/1.0.0/publish", nil)
	pubReq = pubReq.WithContext(adminAgentCtx())
	pubRec := httptest.NewRecorder()
	r.ServeHTTP(pubRec, pubReq)
	if pubRec.Code != http.StatusOK {
		t.Fatalf("publish: status = %d", pubRec.Code)
	}

	// Deprecate
	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-dep/ag-dep/deprecate", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body map[string]string
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body["status"] != "deprecated" {
		t.Errorf("status = %q, want deprecated", body["status"])
	}
}

func TestAgentHandler_DeprecateAgent_NotPublished(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-depnp", "ag-depnp") // draft, not published

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agents/ag-depnp/ag-depnp/deprecate", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
}

// ─── Status / Visibility filter tests ──────────────────────────────────────

func TestAgentHandler_List_FilterByStatus(t *testing.T) {
	resetTables(t)

	seedAgent(t, "agfst-ns1", "agfst-draft")
	seedAgentPublished(t, "agfst-ns2", "agfst-pub-1")
	seedAgentPublished(t, "agfst-ns3", "agfst-pub-2")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents?status=published", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []struct {
			Slug   string `json:"slug"`
			Status string `json:"status"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("status=published: got %d items, want 2", len(body.Items))
	}
	for _, item := range body.Items {
		if item.Status != "published" {
			t.Errorf("expected status=published, got %q for slug %q", item.Status, item.Slug)
		}
	}
}

func TestAgentHandler_List_FilterByVisibility(t *testing.T) {
	resetTables(t)

	seedAgent(t, "agfvis-ns1", "agfvis-private")
	seedAgentPublic(t, "agfvis-ns2", "agfvis-public")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents?visibility=private", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []struct {
			Slug       string `json:"slug"`
			Visibility string `json:"visibility"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 1 {
		t.Errorf("visibility=private: got %d items, want 1", len(body.Items))
	}
	if len(body.Items) > 0 && body.Items[0].Slug != "agfvis-private" {
		t.Errorf("visibility=private: slug=%q, want agfvis-private", body.Items[0].Slug)
	}
}

func TestAgentHandler_List_InvalidFilterIgnored(t *testing.T) {
	resetTables(t)
	seedAgent(t, "aginv-ns1", "aginv-ag-1")
	seedAgent(t, "aginv-ns2", "aginv-ag-2")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents?status=badvalue&visibility=garbage", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []any `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("invalid filters ignored: got %d items, want 2", len(body.Items))
	}
}

// seedAgentPublished creates an agent and promotes its status to published via SQL.
func seedAgentPublished(t *testing.T, ns, slug string) {
	t.Helper()
	seedAgent(t, ns, slug)
	ag, err := testDB.GetAgent(context.Background(), ns, slug, false)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if _, err := testDB.Pool.Exec(context.Background(), "UPDATE agents SET status=$1 WHERE id=$2", "published", ag.ID); err != nil {
		t.Fatalf("seedAgentPublished: %v", err)
	}
}

func TestAgentHandler_ListAgents_TotalCount(t *testing.T) {
	resetTables(t)
	// Seed 3 agents visible to admin.
	for i := range 3 {
		seedAgent(t, "atc-pub", fmt.Sprintf("atc-ag-%d", i))
	}

	// Ask for page of 2 — items should have 2, but total_count should be 3.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/agents?limit=2", nil)
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items      []any `json:"items"`
		TotalCount int   `json:"total_count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Errorf("items length: got %d, want 2", len(body.Items))
	}
	if body.TotalCount != 3 {
		t.Errorf("total_count: got %d, want 3", body.TotalCount)
	}
}
