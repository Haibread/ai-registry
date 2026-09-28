package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

func assertURLProblem(t *testing.T, rec *httptest.ResponseRecorder, field string) {
	t.Helper()
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var p struct {
		Detail string `json:"detail"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&p); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if !strings.HasPrefix(p.Detail, field+" ") {
		t.Errorf("detail = %q, want it to name %s", p.Detail, field)
	}
}

func TestMCPHandler_CreateServer_RejectsNonHTTPURLs(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "url-ns", "URL NS")
	r := newMCPRouter()

	tests := []struct {
		name, payload, field string
	}{
		{"javascript homepage", `{"namespace":"url-ns","slug":"s1","name":"S","homepage_url":"javascript:alert(1)"}`, "homepage_url"},
		{"data repo", `{"namespace":"url-ns","slug":"s2","name":"S","repo_url":"data:text/html,x"}`, "repo_url"},
		{"relative homepage", `{"namespace":"url-ns","slug":"s3","name":"S","homepage_url":"/about"}`, "homepage_url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertURLProblem(t, fire(r, adminCtx(), http.MethodPost, "/api/v1/mcp/servers", tt.payload), tt.field)
		})
	}

	rec := fire(r, adminCtx(), http.MethodPost, "/api/v1/mcp/servers",
		`{"namespace":"url-ns","slug":"ok","name":"S","homepage_url":"HTTPS://example.com","repo_url":"https://github.com/acme/ok"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("valid urls: status = %d, want 201; body: %s", rec.Code, rec.Body.String())
	}
}

func TestAgentHandler_CreateVersion_RejectsNonHTTPURLs(t *testing.T) {
	resetTables(t)
	seedAgent(t, "ag-url", "ag-url")
	r := newAgentRouter()

	tests := []struct {
		name, payload, field string
	}{
		{"javascript endpoint", `{"version":"1.0.0","endpoint_url":"javascript:alert(1)"}`, "endpoint_url"},
		{"data documentation", `{"version":"1.0.0","endpoint_url":"https://a.example","documentation_url":"data:text/html,x"}`, "documentation_url"},
		{"ftp icon", `{"version":"1.0.0","endpoint_url":"https://a.example","icon_url":"ftp://a.example/i.png"}`, "icon_url"},
		{"javascript provider url", `{"version":"1.0.0","endpoint_url":"https://a.example","provider":{"organization":"Acme","url":"javascript:alert(1)"}}`, "provider.url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertURLProblem(t, fire(r, adminAgentCtx(), http.MethodPost, "/api/v1/agents/ag-url/ag-url/versions", tt.payload), tt.field)
		})
	}
}

func TestEntryChangeHandler_MetadataEditRejectsNonHTTPURLs(t *testing.T) {
	resetTables(t)
	srvID := seedPublishedMCPForHandler(t, "acme", "weather")
	r := newEntryChangeRouter()

	for name, ctx := range map[string]context.Context{"editor enqueue": editorCtx(), "admin apply": adminCtx()} {
		t.Run(name, func(t *testing.T) {
			assertURLProblem(t, fire(r, ctx, http.MethodPatch,
				"/api/v1/mcp/servers/acme/weather", `{"repo_url":"javascript:alert(1)"}`), "repo_url")
		})
	}
	if _, ok, err := testDB.GetPendingEntryChange(context.Background(), domain.EntryResourceMCPServer, srvID); err != nil || ok {
		t.Fatalf("pending change after rejected edit: ok=%v err=%v, want none", ok, err)
	}
}

// A payload enqueued before validation existed must not be applied on approve.
func TestEntryChangeHandler_ApproveRejectsStoredNonHTTPURL(t *testing.T) {
	resetTables(t)
	srvID := seedPublishedMCPForHandler(t, "acme", "weather")
	r := newEntryChangeRouter()

	payload := `{"name":"weather","homepage_url":"javascript:alert(1)"}`
	if _, err := testDB.CreateEntryChangeRequest(context.Background(), domain.EntryResourceMCPServer, srvID,
		domain.EntryChangeMetadataEdit, json.RawMessage(payload), store.Actor{Subject: "editor-uuid"}); err != nil {
		t.Fatalf("CreateEntryChangeRequest: %v", err)
	}
	assertURLProblem(t, fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/change-request/approve", `{"revision":1}`), "homepage_url")

	var homepage string
	if err := testDB.Pool.QueryRow(context.Background(),
		`SELECT homepage_url FROM mcp_servers WHERE id=$1`, srvID).Scan(&homepage); err != nil {
		t.Fatalf("read: %v", err)
	}
	if homepage != "" {
		t.Errorf("homepage_url = %q, want unchanged", homepage)
	}
}

// Rows stored before validation keep their links; editing another field of
// such a row must not be blocked by them.
func TestEntryChangeHandler_MetadataEditKeepsLegacyURL(t *testing.T) {
	resetTables(t)
	srvID := seedPublishedMCPForHandler(t, "acme", "weather")
	if _, err := testDB.Pool.Exec(context.Background(),
		`UPDATE mcp_servers SET homepage_url='javascript:alert(1)' WHERE id=$1`, srvID); err != nil {
		t.Fatalf("seed legacy url: %v", err)
	}
	r := newEntryChangeRouter()

	if rec := fire(r, editorCtx(), http.MethodPatch,
		"/api/v1/mcp/servers/acme/weather", `{"description":"new"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("enqueue: %d, body: %s", rec.Code, rec.Body.String())
	}
	if rec := fire(r, adminCtx(), http.MethodPost,
		"/api/v1/mcp/servers/acme/weather/change-request/approve", `{"revision":1}`); rec.Code != http.StatusNoContent {
		t.Fatalf("approve: %d, body: %s", rec.Code, rec.Body.String())
	}
}
