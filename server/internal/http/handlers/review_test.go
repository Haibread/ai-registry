package handlers_test

// Tests for the change-approval workflow HTTP handlers. The store
// layer's transitions and discriminated sentinels are exercised in
// internal/store; these tests focus on the handler glue: URL parsing,
// body validation, error → problem-type mapping, audit-event firing,
// and the wire shape of the review queue.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

func newReviewRouter() *chi.Mux {
	h := handlers.NewReviewHandlers(testDB, testDB)
	r := chi.NewRouter()
	r.Get("/api/v1/review-queue", h.ListReviewQueue)
	r.Route("/api/v1/mcp/servers/{namespace}/{slug}", func(r chi.Router) {
		r.Post("/versions/{version}/submit", h.SubmitMCPVersion)
		r.Post("/versions/{version}/withdraw", h.WithdrawMCPVersion)
		r.Post("/versions/{version}/approve", h.ApproveMCPVersion)
		r.Post("/versions/{version}/reject", h.RejectMCPVersion)
		r.Post("/deletion-request", h.RequestMCPDeletion)
		r.Post("/deletion-request/approve", h.ApproveMCPDeletion)
		r.Post("/deletion-request/reject", h.RejectMCPDeletion)
	})
	r.Route("/api/v1/agents/{namespace}/{slug}", func(r chi.Router) {
		r.Post("/versions/{version}/submit", h.SubmitAgentVersion)
		r.Post("/versions/{version}/withdraw", h.WithdrawAgentVersion)
		r.Post("/versions/{version}/approve", h.ApproveAgentVersion)
		r.Post("/versions/{version}/reject", h.RejectAgentVersion)
		r.Post("/deletion-request", h.RequestAgentDeletion)
		r.Post("/deletion-request/approve", h.ApproveAgentDeletion)
		r.Post("/deletion-request/reject", h.RejectAgentDeletion)
	})
	return r
}

// authedRequest builds a request whose context already carries admin
// claims so the handler's audit logger sees a real actor.
func authedRequest(method, path string, body []byte) *http.Request {
	var r *http.Request
	if body == nil {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, bytes.NewBuffer(body))
		r.Header.Set("Content-Type", "application/json")
	}
	claims := &auth.KeycloakClaims{
		Email:       "admin@example.com",
		RealmAccess: auth.RealmAccess{Roles: []string{"admin"}},
	}
	claims.Subject = "admin-uuid"
	return r.WithContext(auth.ContextWithClaims(r.Context(), claims))
}

// seedDraftMCPServerVersion creates publisher + MCP server + a draft
// version for the handler tests.
func seedDraftMCPServerVersion(t *testing.T, ns, slug, ver string) {
	t.Helper()
	pubID := seedPublisher(t, ns, ns)
	srv, err := testDB.CreateMCPServer(context.Background(), store.CreateMCPServerParams{
		PublisherID: pubID, Slug: slug, Name: slug,
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	if _, err := testDB.CreateMCPServerVersion(context.Background(), store.CreateMCPServerVersionParams{
		ServerID:        srv.ID,
		Version:         ver,
		Runtime:         domain.RuntimeStdio,
		Packages:        json.RawMessage(`[{"registryType":"npm","identifier":"@scope/p","version":"1.0.0","transport":{"type":"stdio"}}]`),
		Capabilities:    json.RawMessage(`{}`),
		ProtocolVersion: "2024-11-05",
	}); err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}
}

func seedDraftAgentForHandler(t *testing.T, ns, slug, ver string) {
	t.Helper()
	pubID := seedPublisher(t, ns, ns)
	ag, err := testDB.CreateAgent(context.Background(), store.CreateAgentParams{
		PublisherID: pubID, Slug: slug, Name: slug,
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := testDB.CreateAgentVersion(context.Background(), store.CreateAgentVersionParams{
		AgentID:            ag.ID,
		Version:            ver,
		EndpointURL:        "https://agent.example/api",
		Skills:             json.RawMessage(`[{"id":"s1","name":"s","description":"d","tags":["x"]}]`),
		DefaultInputModes:  []string{"text/plain"},
		DefaultOutputModes: []string{"text/plain"},
		ProtocolVersion:    "0.3.0",
	}); err != nil {
		t.Fatalf("CreateAgentVersion: %v", err)
	}
}

// ── MCP handlers ────────────────────────────────────────────────────────

func TestReviewHandler_SubmitApproveMCP(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/submit", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("submit: %d, body: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/approve",
		[]byte(`{"revision":1}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("approve: %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewHandler_SubmitWithdrawMCP(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/submit", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("submit: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/withdraw", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("withdraw: %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewHandler_RejectMCPWithReason(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/submit", nil))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/reject",
		[]byte(`{"revision":1,"reason":"needs more docs"}`)))
	if rec.Code != http.StatusNoContent {
		t.Errorf("reject: %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewHandler_ApproveMCP_Validation(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"missing revision", `{}`, http.StatusUnprocessableEntity},
		{"zero revision", `{"revision":0}`, http.StatusUnprocessableEntity},
		{"bad json", `{nope}`, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, authedRequest(
				http.MethodPost,
				"/api/v1/mcp/servers/acme/weather/versions/1.0.0/approve",
				[]byte(tc.body)))
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestReviewHandler_RejectMCP_Validation(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"missing revision", `{"reason":"x"}`, http.StatusUnprocessableEntity},
		{"missing reason", `{"revision":1}`, http.StatusUnprocessableEntity},
		{"empty reason", `{"revision":1,"reason":""}`, http.StatusUnprocessableEntity},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, authedRequest(
				http.MethodPost,
				"/api/v1/mcp/servers/acme/weather/versions/1.0.0/reject",
				[]byte(tc.body)))
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestReviewHandler_ApproveMCP_DiscriminatedConflicts(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	// Submit then attempt approve with stale revision → revision-mismatch.
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/submit", nil))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/approve",
		[]byte(`{"revision":99}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("stale revision: %d, body: %s", rec.Code, rec.Body.String())
	}
	var p struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got := p.Type; got == "" || got[len(got)-len("review-revision-mismatch"):] != "review-revision-mismatch" {
		t.Errorf("type = %q, want suffix review-revision-mismatch", got)
	}
}

func TestReviewHandler_SubmitMCP_NotFound(t *testing.T) {
	resetTables(t)
	r := newReviewRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/missing/missing/versions/1.0.0/submit", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestReviewHandler_MCPDeletionFlow(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	// Request deletion → 202.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deletion-request", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request: %d, body: %s", rec.Code, rec.Body.String())
	}

	// Reject → 204 + clears the request.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deletion-request/reject",
		[]byte(`{"reason":"too soon"}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reject: %d, body: %s", rec.Code, rec.Body.String())
	}

	// Now request again and approve → soft-delete.
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deletion-request", nil))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deletion-request/approve", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("approve: %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewHandler_MCPDeletion_RejectMissingReason(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deletion-request/reject",
		[]byte(`{}`)))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

// ── Agent handlers (smoke coverage; same shape as MCP) ──────────────────

func TestReviewHandler_SubmitApproveAgent(t *testing.T) {
	resetTables(t)
	seedDraftAgentForHandler(t, "acme", "planner", "0.1.0")
	r := newReviewRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/agents/acme/planner/versions/0.1.0/submit", nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("submit: %d, body: %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/agents/acme/planner/versions/0.1.0/approve",
		[]byte(`{"revision":1}`)))
	if rec.Code != http.StatusNoContent {
		t.Errorf("approve: %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewHandler_AgentVersionRejectAndWithdraw(t *testing.T) {
	resetTables(t)
	seedDraftAgentForHandler(t, "acme", "planner", "0.1.0")
	r := newReviewRouter()

	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/agents/acme/planner/versions/0.1.0/submit", nil))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/agents/acme/planner/versions/0.1.0/reject",
		[]byte(`{"revision":1,"reason":"nope"}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reject: %d", rec.Code)
	}

	// Re-submit → withdraw.
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/agents/acme/planner/versions/0.1.0/submit", nil))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/agents/acme/planner/versions/0.1.0/withdraw", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("withdraw: %d", rec.Code)
	}
}

func TestReviewHandler_AgentDeletionFlow(t *testing.T) {
	resetTables(t)
	seedDraftAgentForHandler(t, "acme", "planner", "0.1.0")
	r := newReviewRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost, "/api/v1/agents/acme/planner/deletion-request", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("request: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost, "/api/v1/agents/acme/planner/deletion-request/approve", nil))
	if rec.Code != http.StatusNoContent {
		t.Errorf("approve: %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestReviewHandler_AgentDeletion_RejectFlow(t *testing.T) {
	resetTables(t)
	seedDraftAgentForHandler(t, "acme", "planner", "0.1.0")
	r := newReviewRouter()

	// Request deletion, then reject with reason → 204.
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost, "/api/v1/agents/acme/planner/deletion-request", nil))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/agents/acme/planner/deletion-request/reject",
		[]byte(`{"reason":"not yet"}`)))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("reject: %d, body: %s", rec.Code, rec.Body.String())
	}

	// Reject without reason → 422.
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/agents/acme/planner/deletion-request/reject",
		[]byte(`{}`)))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("missing reason: %d, want 422", rec.Code)
	}
}

func TestReviewHandler_AgentDeletion_NotFound(t *testing.T) {
	resetTables(t)
	r := newReviewRouter()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/agents/missing/missing/deletion-request", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// Exercise the writeReviewProblem branches that the happy-path tests
// don't reach: state-mismatch (approve a Draft) and already-pending
// (request deletion when one is already pending).
func TestReviewHandler_DiscriminatedConflicts_StateAndConflict(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	// Approving a Draft (not pending) → 409 review-state-mismatch.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/approve",
		[]byte(`{"revision":1}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("approve draft: %d, body: %s", rec.Code, rec.Body.String())
	}
	var p struct {
		Type string `json:"type"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&p)
	if p.Type[len(p.Type)-len("review-state-mismatch"):] != "review-state-mismatch" {
		t.Errorf("type = %q, want review-state-mismatch suffix", p.Type)
	}

	// Stacking deletion requests → 409 review-already-pending.
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deletion-request", nil))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deletion-request", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("dup deletion: %d, body: %s", rec.Code, rec.Body.String())
	}
	_ = json.NewDecoder(rec.Body).Decode(&p)
	if p.Type[len(p.Type)-len("review-already-pending"):] != "review-already-pending" {
		t.Errorf("type = %q, want review-already-pending suffix", p.Type)
	}
}

// Exercise the AlreadyPublished discriminator: approve an already-
// published version returns 409 with type ending already-published.
func TestReviewHandler_ApproveMCP_AlreadyPublished(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	r := newReviewRouter()

	// Publish via the workflow: submit + approve.
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/submit", nil))
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/approve",
		[]byte(`{"revision":1}`)))

	// Re-approving the now-published version should hit either
	// state-mismatch (review_state went back to 'none') or
	// already-published. The store returns the latter.
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/approve",
		[]byte(`{"revision":1}`)))
	if rec.Code != http.StatusConflict {
		t.Fatalf("re-approve: %d, body: %s", rec.Code, rec.Body.String())
	}
	var p struct {
		Type string `json:"type"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&p)
	if !(p.Type[len(p.Type)-len("already-published"):] == "already-published" ||
		p.Type[len(p.Type)-len("review-state-mismatch"):] == "review-state-mismatch") {
		t.Errorf("type = %q, want already-published or review-state-mismatch suffix", p.Type)
	}
}

// ── Review queue ────────────────────────────────────────────────────────

func TestReviewHandler_ListReviewQueue(t *testing.T) {
	resetTables(t)
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	seedDraftAgentForHandler(t, "globex", "planner", "0.1.0")

	// Submit both so the queue has two items.
	r := newReviewRouter()
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/submit", nil))
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(
		http.MethodPost,
		"/api/v1/agents/globex/planner/versions/0.1.0/submit", nil))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, authedRequest(http.MethodGet, "/api/v1/review-queue", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("queue: %d, body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []struct {
			Kind          string `json:"kind"`
			PublisherSlug string `json:"publisher_slug"`
			EntrySlug     string `json:"entry_slug"`
			Version       string `json:"version"`
			Revision      int    `json:"revision"`
		} `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Errorf("len = %d, want 2", len(body.Items))
	}
	kinds := map[string]bool{}
	for _, it := range body.Items {
		kinds[it.Kind] = true
		if it.Version == "" {
			t.Errorf("queue item %s/%s missing version", it.PublisherSlug, it.EntrySlug)
		}
		if it.Revision != 1 {
			t.Errorf("queue item revision = %d, want 1", it.Revision)
		}
	}
	if !kinds["mcp_version"] || !kinds["agent_version"] {
		t.Errorf("expected both mcp_version and agent_version, got %+v", kinds)
	}
}

// TestReviewHandler_ListReviewQueue_Unauthenticated verifies the queue rejects
// an anonymous caller with 401 (the route is authenticated-only; the handler
// self-gates — ADR 0006).
func TestReviewHandler_ListReviewQueue_Unauthenticated(t *testing.T) {
	resetTables(t)
	rec := httptest.NewRecorder()
	newReviewRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/review-queue", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

// TestReviewHandler_ListReviewQueue_NonReviewerForbidden verifies an
// authenticated caller who holds no Reviewer role anywhere (here an Editor) gets
// 403 — the queue is an approver tool (ADR 0006).
func TestReviewHandler_ListReviewQueue_NonReviewerForbidden(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	pubID := seedPublisher(t, "acme", "acme")
	user, err := testDB.CreateUser(ctx, store.CreateUserParams{Email: "ed@acme.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := testDB.CreateGrant(ctx, store.CreateGrantParams{
		PrincipalType: domain.PrincipalUser, PrincipalID: user.ID,
		PublisherID: pubID, Role: domain.RoleEditor,
	}); err != nil {
		t.Fatalf("CreateGrant: %v", err)
	}

	reqCtx := auth.ContextWithPrincipal(ctx, &auth.Principal{UserID: user.ID, Email: user.Email})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/review-queue", nil).WithContext(reqCtx)
	rec := httptest.NewRecorder()
	newReviewRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403; body: %s", rec.Code, rec.Body.String())
	}
}

// TestReviewHandler_ListReviewQueue_ScopedToReviewerPublishers verifies a
// per-publisher Reviewer sees only their own publisher's pending items, not
// another publisher's (ADR 0006 reviewer scoping).
func TestReviewHandler_ListReviewQueue_ScopedToReviewerPublishers(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	seedDraftAgentForHandler(t, "globex", "planner", "0.1.0")

	r := newReviewRouter()
	// Submit both (as admin) so the queue has one item per publisher.
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/submit", nil))
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(http.MethodPost,
		"/api/v1/agents/globex/planner/versions/0.1.0/submit", nil))

	// A Reviewer on acme only.
	acmeID, err := testDB.GetPublisherBySlug(ctx, "acme")
	if err != nil {
		t.Fatalf("GetPublisherBySlug: %v", err)
	}
	user, err := testDB.CreateUser(ctx, store.CreateUserParams{Email: "rev@acme.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := testDB.CreateGrant(ctx, store.CreateGrantParams{
		PrincipalType: domain.PrincipalUser, PrincipalID: user.ID,
		PublisherID: acmeID, Role: domain.RoleReviewer,
	}); err != nil {
		t.Fatalf("CreateGrant: %v", err)
	}

	reqCtx := auth.ContextWithPrincipal(ctx, &auth.Principal{UserID: user.ID, Email: user.Email})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/review-queue", nil).WithContext(reqCtx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("queue: %d, body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []struct {
			PublisherSlug string `json:"publisher_slug"`
		} `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 1 {
		t.Fatalf("len = %d, want 1 (only acme's item)", len(body.Items))
	}
	if body.Items[0].PublisherSlug != "acme" {
		t.Errorf("publisher_slug = %q, want acme", body.Items[0].PublisherSlug)
	}
}

// TestReviewHandler_ListReviewQueue_GlobalReviewerSeesAll verifies a holder of a
// GLOBAL Reviewer grant (the shape the seeded reviewer group carries) sees every
// publisher's queue, without being a Server Admin (ADR 0006).
func TestReviewHandler_ListReviewQueue_GlobalReviewerSeesAll(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	seedDraftMCPServerVersion(t, "acme", "weather", "1.0.0")
	seedDraftAgentForHandler(t, "globex", "planner", "0.1.0")

	r := newReviewRouter()
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/versions/1.0.0/submit", nil))
	r.ServeHTTP(httptest.NewRecorder(), authedRequest(http.MethodPost,
		"/api/v1/agents/globex/planner/versions/0.1.0/submit", nil))

	user, err := testDB.CreateUser(ctx, store.CreateUserParams{Email: "global-rev@x.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// PublisherID empty = a global (all-publishers) Reviewer grant.
	if _, err := testDB.CreateGrant(ctx, store.CreateGrantParams{
		PrincipalType: domain.PrincipalUser, PrincipalID: user.ID,
		Role: domain.RoleReviewer,
	}); err != nil {
		t.Fatalf("CreateGrant: %v", err)
	}

	reqCtx := auth.ContextWithPrincipal(ctx, &auth.Principal{UserID: user.ID, Email: user.Email})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/review-queue", nil).WithContext(reqCtx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("queue: %d, body: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Items []json.RawMessage `json:"items"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Items) != 2 {
		t.Errorf("len = %d, want 2 (global reviewer sees all publishers)", len(body.Items))
	}
}
