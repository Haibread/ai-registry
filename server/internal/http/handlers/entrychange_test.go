package handlers_test

// Tests for the entry-change approval flow at the handler layer: an Editor's
// visibility/deprecate/patch request enqueues (202) instead of applying, a
// Server Admin keeps the immediate path (200), and the change-request
// approve/reject/withdraw endpoints resolve the entry's pending change.

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

// newEntryChangeRouter wires the MCP enqueue endpoints plus the entry-change
// review endpoints onto one router so a test can drive the whole flow.
func newEntryChangeRouter() *chi.Mux {
	mcp := handlers.NewMCPHandlers(testDB, testDB, nil)
	rev := handlers.NewReviewHandlers(testDB, testDB)
	r := chi.NewRouter()
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/visibility", mcp.SetVisibility)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/deprecate", mcp.DeprecateServer)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/undeprecate", mcp.UndeprecateServer)
	r.Patch("/api/v1/mcp/servers/{namespace}/{slug}", mcp.PatchServer)
	r.Get("/api/v1/review-queue", rev.ListReviewQueue)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/change-request/approve", rev.ApproveMCPChange)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/change-request/reject", rev.RejectMCPChange)
	r.Post("/api/v1/mcp/servers/{namespace}/{slug}/change-request/withdraw", rev.WithdrawMCPChange)
	return r
}

// editorCtx returns a context carrying a non-admin principal (an Editor as far
// as these middleware-free unit routers are concerned).
func editorCtx() context.Context {
	return auth.ContextWithPrincipal(context.Background(),
		&auth.Principal{UserID: "editor-uuid", Email: "editor@example.com"})
}

// seedPublishedMCPForHandler creates a publisher + server flipped to published
// so visibility-to-public and deprecate preconditions hold.
func seedPublishedMCPForHandler(t *testing.T, ns, slug string) string {
	t.Helper()
	pubID := seedPublisher(t, ns, ns)
	srv, err := testDB.CreateMCPServer(context.Background(), store.CreateMCPServerParams{
		PublisherID: pubID, Slug: slug, Name: slug,
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	if err := testDB.SetMCPServerStatus(context.Background(), srv.ID, domain.StatusPublished); err != nil {
		t.Fatalf("SetMCPServerStatus: %v", err)
	}
	return srv.ID
}

func fire(r http.Handler, ctx context.Context, method, path, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req.WithContext(ctx))
	return rec
}

func TestEntryChangeHandler_EditorEnqueuesVisibility(t *testing.T) {
	resetTables(t)
	srvID := seedPublishedMCPForHandler(t, "acme", "weather")
	r := newEntryChangeRouter()

	rec := fire(r, editorCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/visibility", `{"visibility":"public"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body: %s", rec.Code, rec.Body.String())
	}

	// The entry is unchanged until approval.
	var vis string
	if err := testDB.Pool.QueryRow(context.Background(),
		`SELECT visibility FROM mcp_servers WHERE id=$1`, srvID).Scan(&vis); err != nil {
		t.Fatalf("read: %v", err)
	}
	if vis != "private" {
		t.Errorf("visibility = %q, want private (still pending)", vis)
	}

	// And it shows up in the review queue as an mcp_change item.
	qrec := fire(r, adminCtx(), http.MethodGet, "/api/v1/review-queue", "")
	if qrec.Code != http.StatusOK {
		t.Fatalf("queue: %d, body: %s", qrec.Code, qrec.Body.String())
	}
	var q struct {
		Items []struct {
			Kind   string `json:"kind"`
			Action string `json:"action"`
		} `json:"items"`
	}
	if err := json.NewDecoder(qrec.Body).Decode(&q); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(q.Items) != 1 || q.Items[0].Kind != "mcp_change" || q.Items[0].Action != "visibility" {
		t.Fatalf("queue items = %+v, want one mcp_change/visibility", q.Items)
	}
}

func TestEntryChangeHandler_AdminAppliesImmediately(t *testing.T) {
	resetTables(t)
	srvID := seedPublishedMCPForHandler(t, "acme", "weather")
	r := newEntryChangeRouter()

	rec := fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/visibility", `{"visibility":"public"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	var vis string
	if err := testDB.Pool.QueryRow(context.Background(),
		`SELECT visibility FROM mcp_servers WHERE id=$1`, srvID).Scan(&vis); err != nil {
		t.Fatalf("read: %v", err)
	}
	if vis != "public" {
		t.Errorf("visibility = %q, want public (applied immediately)", vis)
	}
}

func TestEntryChangeHandler_EnqueueThenApprove(t *testing.T) {
	resetTables(t)
	srvID := seedPublishedMCPForHandler(t, "acme", "weather")
	r := newEntryChangeRouter()

	if rec := fire(r, editorCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/visibility", `{"visibility":"public"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("enqueue: %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec := fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/change-request/approve", `{"revision":1}`); rec.Code != http.StatusNoContent {
		t.Fatalf("approve: %d, body: %s", rec.Code, rec.Body.String())
	}
	var vis string
	if err := testDB.Pool.QueryRow(context.Background(),
		`SELECT visibility FROM mcp_servers WHERE id=$1`, srvID).Scan(&vis); err != nil {
		t.Fatalf("read: %v", err)
	}
	if vis != "public" {
		t.Errorf("visibility = %q, want public after approve", vis)
	}
}

func TestEntryChangeHandler_RejectRequiresReason(t *testing.T) {
	resetTables(t)
	seedPublishedMCPForHandler(t, "acme", "weather")
	r := newEntryChangeRouter()

	if rec := fire(r, editorCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deprecate", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("enqueue deprecate: %d, body: %s", rec.Code, rec.Body.String())
	}
	rec := fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/change-request/reject", `{"revision":1}`)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("reject without reason = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
}

func TestEntryChangeHandler_WithdrawClearsPending(t *testing.T) {
	resetTables(t)
	seedPublishedMCPForHandler(t, "acme", "weather")
	r := newEntryChangeRouter()

	if rec := fire(r, editorCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/visibility", `{"visibility":"public"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("enqueue: %d", rec.Code)
	}
	if rec := fire(r, editorCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/change-request/withdraw", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("withdraw: %d, body: %s", rec.Code, rec.Body.String())
	}
	// Withdrawing frees the pending slot, so a fresh enqueue succeeds.
	if rec := fire(r, editorCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deprecate", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("re-enqueue after withdraw: %d, body: %s", rec.Code, rec.Body.String())
	}
}

func TestEntryChangeHandler_DuplicatePendingConflict(t *testing.T) {
	resetTables(t)
	seedPublishedMCPForHandler(t, "acme", "weather")
	r := newEntryChangeRouter()

	if rec := fire(r, editorCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/visibility", `{"visibility":"public"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("first enqueue: %d", rec.Code)
	}
	rec := fire(r, editorCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deprecate", "")
	if rec.Code != http.StatusConflict {
		t.Errorf("second enqueue = %d, want 409; body: %s", rec.Code, rec.Body.String())
	}
}

// ── undeprecate: deprecated → published through the same contract ────────────

func TestEntryChangeHandler_UndeprecateRoundTrip(t *testing.T) {
	resetTables(t)
	srvID := seedPublishedMCPForHandler(t, "acme", "weather")
	r := newEntryChangeRouter()

	// Undeprecating a published entry is a 409 (wrong state) on both paths.
	if rec := fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/undeprecate", ""); rec.Code != http.StatusConflict {
		t.Fatalf("undeprecate published = %d, want 409; body: %s", rec.Code, rec.Body.String())
	}

	// Admin deprecates immediately, then editor requests a republish: 202.
	if rec := fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deprecate", ""); rec.Code != http.StatusOK {
		t.Fatalf("deprecate: %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec := fire(r, editorCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/undeprecate", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("editor undeprecate = %d, want 202; body: %s", rec.Code, rec.Body.String())
	}
	if _, status, _ := readStatusVis(t, srvID); status != "deprecated" {
		t.Errorf("status = %q, want deprecated (still pending)", status)
	}

	// Approval applies the republish.
	if rec := fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/change-request/approve", `{"revision":1}`); rec.Code != http.StatusNoContent {
		t.Fatalf("approve: %d, body: %s", rec.Code, rec.Body.String())
	}
	if _, status, _ := readStatusVis(t, srvID); status != "published" {
		t.Errorf("status = %q, want published after approve", status)
	}

	// Admin path is immediate: deprecate again, undeprecate as admin → 200.
	if rec := fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/deprecate", ""); rec.Code != http.StatusOK {
		t.Fatalf("re-deprecate: %d", rec.Code)
	}
	if rec := fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/undeprecate", ""); rec.Code != http.StatusOK {
		t.Fatalf("admin undeprecate = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if _, status, _ := readStatusVis(t, srvID); status != "published" {
		t.Errorf("status = %q, want published (admin immediate)", status)
	}
}

// readStatusVis reads (visibility, status, name) for an MCP server row.
func readStatusVis(t *testing.T, srvID string) (vis, status, name string) {
	t.Helper()
	if err := testDB.Pool.QueryRow(context.Background(),
		`SELECT visibility, status, name FROM mcp_servers WHERE id=$1`, srvID).Scan(&vis, &status, &name); err != nil {
		t.Fatalf("read server: %v", err)
	}
	return vis, status, name
}

// TestEntryChangeHandler_SelfApprovalForbidden: the Editor who enqueued a
// change cannot approve it; another Reviewer can.
func TestEntryChangeHandler_SelfApprovalForbidden(t *testing.T) {
	cases := []struct {
		name    string
		seed    func(t *testing.T)
		enqueue string
		approve string
	}{
		{
			name:    "mcp",
			seed:    func(t *testing.T) { seedPublishedMCPForHandler(t, "acme", "weather") },
			enqueue: "/api/v1/mcp/servers/acme/weather/visibility",
			approve: "/api/v1/mcp/servers/acme/weather/change-request/approve",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resetTables(t)
			tc.seed(t)
			r := newEntryChangeRouter()

			if rec := fire(r, editorCtx(), http.MethodPost, tc.enqueue, `{"visibility":"public"}`); rec.Code != http.StatusAccepted {
				t.Fatalf("enqueue: %d, body: %s", rec.Code, rec.Body.String())
			}
			assertProblemType(t, fire(r, editorCtx(), http.MethodPost, tc.approve, `{"revision":1}`),
				http.StatusForbidden, "self-approval-forbidden")
			if rec := fire(r, principalCtx("reviewer-uuid", false), http.MethodPost, tc.approve, `{"revision":1}`); rec.Code != http.StatusNoContent {
				t.Fatalf("approve by another reviewer: %d, body: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
