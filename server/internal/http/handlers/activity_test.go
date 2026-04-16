package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
)

// newActivityRouter builds a chi router wired for the public activity
// endpoints on both MCP and agents. Tests exercise the full handler
// behaviour (404 on missing resource, whitelist, scrub, pagination).
func newActivityRouter() *chi.Mux {
	mcpH := handlers.NewMCPHandlers(testDB, testDB, nil)
	agH := handlers.NewAgentHandlers(testDB, testDB, nil)
	r := chi.NewRouter()
	r.Get("/api/v1/mcp/servers/{namespace}/{slug}/activity", mcpH.ListMCPServerActivity)
	r.Get("/api/v1/agents/{namespace}/{slug}/activity", agH.ListAgentActivity)
	return r
}

// logEventFor writes a single audit row targeting the given resource.
// Includes a leaky email/subject and metadata — the handler must scrub
// these out of the public response.
func logEventFor(t *testing.T, resourceType, resourceID string, action domain.AuditAction, extra map[string]any) {
	t.Helper()
	metadata := map[string]any{
		"client_ip":     "10.0.0.5",        // must be scrubbed (not on allowlist)
		"user_agent":    "curl/8.0",        // must be scrubbed
		"internal_note": "do not expose",   // must be scrubbed
	}
	for k, v := range extra {
		metadata[k] = v
	}
	testDB.LogAuditEvent(context.Background(), domain.AuditEvent{
		ActorSubject: "kc-subject-XYZ",
		ActorEmail:   "privileged@internal.example",
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Metadata:     metadata,
	})
}

// resolveMCPServerID looks up the ULID of a seeded MCP server for use as
// the audit resource_id.
func resolveMCPServerID(t *testing.T, ns, slug string) string {
	t.Helper()
	srv, err := testDB.GetMCPServer(context.Background(), ns, slug, false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	return srv.ID
}

// resolveAgentID looks up the ULID of a seeded agent.
func resolveAgentID(t *testing.T, ns, slug string) string {
	t.Helper()
	ag, err := testDB.GetAgent(context.Background(), ns, slug, false)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	return ag.ID
}

// ─── MCP activity ───────────────────────────────────────────────────────────

func TestActivity_MCP_404_WhenServerDoesNotExist(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/mcp/servers/nope/missing/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestActivity_MCP_404_WhenServerIsPrivate(t *testing.T) {
	// Private servers are invisible to anonymous callers, and so is their
	// activity. (Admins would see it via the regular /audit endpoint.)
	resetTables(t)
	seedMCPServer(t, "pub-private", "srv-p") // default visibility = private

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/mcp/servers/pub-private/srv-p/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (private server must not be visible)", rec.Code)
	}
}

func TestActivity_MCP_EmptyFeed(t *testing.T) {
	resetTables(t)
	seedMCPServerPublic(t, "pub-empty", "srv-e")

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/mcp/servers/pub-empty/srv-e/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items      []any  `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Items == nil {
		t.Error("items should be empty array, not null")
	}
	if len(body.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(body.Items))
	}
}

func TestActivity_MCP_ReturnsWhitelistedActions(t *testing.T) {
	resetTables(t)
	seedMCPServerPublic(t, "pub-feed", "srv-f")
	srvID := resolveMCPServerID(t, "pub-feed", "srv-f")

	// Whitelisted
	logEventFor(t, "mcp_server", srvID, domain.ActionMCPServerCreated, nil)
	logEventFor(t, "mcp_server", srvID, domain.ActionMCPVersionPublished, map[string]any{"version": "1.0.0"})
	logEventFor(t, "mcp_server", srvID, domain.ActionMCPServerDeprecated, map[string]any{"reason": "replaced"})
	// Not whitelisted — draft creation should NOT appear on the public feed
	logEventFor(t, "mcp_server", srvID, domain.ActionMCPVersionCreated, map[string]any{"version": "1.1.0"})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/mcp/servers/pub-feed/srv-f/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []struct {
			Action   string `json:"action"`
			Version  string `json:"version"`
			Metadata map[string]any
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 3 {
		t.Fatalf("expected 3 whitelisted items, got %d", len(body.Items))
	}
	for _, it := range body.Items {
		if it.Action == string(domain.ActionMCPVersionCreated) {
			t.Errorf("draft version creation leaked into public feed: %q", it.Action)
		}
	}
}

func TestActivity_MCP_PrivacyScrub_NoActorIdentity(t *testing.T) {
	// The response body must not contain the actor's email or subject.
	resetTables(t)
	seedMCPServerPublic(t, "pub-priv", "srv-p")
	srvID := resolveMCPServerID(t, "pub-priv", "srv-p")

	logEventFor(t, "mcp_server", srvID, domain.ActionMCPServerCreated, nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/mcp/servers/pub-priv/srv-p/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	raw := rec.Body.String()
	forbidden := []string{
		"privileged@internal.example",
		"kc-subject-XYZ",
		"actor_email",
		"actor_subject",
		"client_ip",
		"user_agent",
		"internal_note",
	}
	for _, needle := range forbidden {
		if strings.Contains(raw, needle) {
			t.Errorf("response leaked %q: %s", needle, raw)
		}
	}
}

func TestActivity_MCP_PrivacyScrub_MetadataAllowlist(t *testing.T) {
	// Whitelisted metadata keys (version, reason, from, to, visibility,
	// field) MUST round-trip. Everything else is dropped.
	resetTables(t)
	seedMCPServerPublic(t, "pub-md", "srv-md")
	srvID := resolveMCPServerID(t, "pub-md", "srv-md")

	logEventFor(t, "mcp_server", srvID, domain.ActionMCPServerVisibility, map[string]any{
		"from":       "private",
		"to":         "public",
		"visibility": "public",
		"reason":     "approved",
	})

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/mcp/servers/pub-md/srv-md/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	var body struct {
		Items []struct {
			Metadata map[string]any `json:"metadata"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 1 {
		t.Fatalf("got %d items, want 1", len(body.Items))
	}
	md := body.Items[0].Metadata
	for _, want := range []string{"from", "to", "visibility", "reason"} {
		if _, ok := md[want]; !ok {
			t.Errorf("metadata missing allowlisted key %q: %+v", want, md)
		}
	}
}

func TestActivity_MCP_Pagination(t *testing.T) {
	resetTables(t)
	seedMCPServerPublic(t, "pub-page", "srv-page")
	srvID := resolveMCPServerID(t, "pub-page", "srv-page")

	// Seed 5 whitelisted events.
	for i := 0; i < 5; i++ {
		logEventFor(t, "mcp_server", srvID, domain.ActionMCPServerUpdated,
			map[string]any{"field": "description"})
	}

	// Page 1 of 3
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/mcp/servers/pub-page/srv-page/activity?limit=3", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	var page1 struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	json.NewDecoder(rec.Body).Decode(&page1) //nolint:errcheck
	if len(page1.Items) != 3 {
		t.Fatalf("page1 len = %d, want 3", len(page1.Items))
	}
	if page1.NextCursor == "" {
		t.Fatal("expected non-empty next_cursor")
	}

	// Page 2
	req = httptest.NewRequest(http.MethodGet,
		"/api/v1/mcp/servers/pub-page/srv-page/activity?limit=3&cursor="+page1.NextCursor, nil)
	rec = httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	var page2 struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&page2) //nolint:errcheck
	if len(page2.Items) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2.Items))
	}

	// No overlap across pages.
	seen := map[string]bool{}
	for _, it := range page1.Items {
		seen[it.ID] = true
	}
	for _, it := range page2.Items {
		if seen[it.ID] {
			t.Errorf("id %s appeared on both pages", it.ID)
		}
	}
}

func TestActivity_MCP_ScopedToResource(t *testing.T) {
	// Events for a DIFFERENT server must not appear in this server's feed.
	resetTables(t)
	seedMCPServerPublic(t, "pub-scope-a", "srv-a")
	seedMCPServerPublic(t, "pub-scope-b", "srv-b")
	srvA := resolveMCPServerID(t, "pub-scope-a", "srv-a")
	srvB := resolveMCPServerID(t, "pub-scope-b", "srv-b")

	logEventFor(t, "mcp_server", srvA, domain.ActionMCPServerCreated, nil)
	logEventFor(t, "mcp_server", srvB, domain.ActionMCPServerCreated, nil)
	logEventFor(t, "mcp_server", srvB, domain.ActionMCPServerDeprecated, nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/mcp/servers/pub-scope-a/srv-a/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	var body struct {
		Items []any `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 1 {
		t.Errorf("got %d items for srv-a, want 1 (srv-b's events must not bleed in)", len(body.Items))
	}
}

// ─── Agent activity ─────────────────────────────────────────────────────────

func TestActivity_Agent_404_WhenAgentDoesNotExist(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/agents/nope/missing/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestActivity_Agent_ReturnsWhitelistedActions(t *testing.T) {
	resetTables(t)
	seedAgentPublic(t, "pub-ag", "ag-feed")
	agID := resolveAgentID(t, "pub-ag", "ag-feed")

	logEventFor(t, "agent", agID, domain.ActionAgentCreated, nil)
	logEventFor(t, "agent", agID, domain.ActionAgentVersionPublished,
		map[string]any{"version": "0.1.0"})
	// Not whitelisted
	logEventFor(t, "agent", agID, domain.ActionAgentVersionCreated, nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/agents/pub-ag/ag-feed/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []struct {
			Action    string `json:"action"`
			ActorRole string `json:"actor_role"`
			Version   string `json:"version"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Fatalf("got %d whitelisted items, want 2", len(body.Items))
	}
	for _, it := range body.Items {
		if it.ActorRole != "admin" && it.ActorRole != "publisher" {
			t.Errorf("actor_role = %q, want 'admin' or 'publisher'", it.ActorRole)
		}
	}
}

func TestActivity_Agent_PrivacyScrub(t *testing.T) {
	resetTables(t)
	seedAgentPublic(t, "pub-agsc", "ag-sc")
	agID := resolveAgentID(t, "pub-agsc", "ag-sc")

	logEventFor(t, "agent", agID, domain.ActionAgentCreated, nil)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/agents/pub-agsc/ag-sc/activity", nil)
	rec := httptest.NewRecorder()
	newActivityRouter().ServeHTTP(rec, req)

	raw := rec.Body.String()
	for _, needle := range []string{
		"privileged@internal.example", "kc-subject-XYZ",
		"actor_email", "actor_subject",
	} {
		if strings.Contains(raw, needle) {
			t.Errorf("response leaked %q", needle)
		}
	}
}

// ─── Regression: list method signature ──────────────────────────────────────

func TestActivity_ListAuditParams_WiresResourceFilter(t *testing.T) {
	// Sanity-check that the store layer honours the composite filter the
	// handler depends on. A breakage here means pagination would silently
	// return cross-resource events.
	resetTables(t)
	seedMCPServerPublic(t, "pub-sanity", "srv-s")
	srvID := resolveMCPServerID(t, "pub-sanity", "srv-s")

	logEventFor(t, "mcp_server", srvID, domain.ActionMCPServerCreated, nil)
	logEventFor(t, "mcp_server", "some-other-id", domain.ActionMCPServerCreated, nil)

	events, err := testDB.ListAuditEvents(context.Background(), store.ListAuditParams{
		ResourceType: "mcp_server",
		ResourceID:   srvID,
		Limit:        50,
	})
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("got %d events, want 1 (filter must scope by resource_id)", len(events))
	}
}
