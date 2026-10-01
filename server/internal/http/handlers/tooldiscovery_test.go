package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/store"
	"github.com/haibread/ai-registry/internal/tooldiscovery"
)

type fakeDiscoverer struct {
	res          *tooldiscovery.Result
	err          error
	gotURL       string
	gotTransport tooldiscovery.Transport
}

func (f *fakeDiscoverer) Discover(_ context.Context, rawURL string, tr tooldiscovery.Transport) (*tooldiscovery.Result, error) {
	f.gotURL, f.gotTransport = rawURL, tr
	return f.res, f.err
}

func newDiscoveryRouter(d handlers.ToolDiscoverer) *chi.Mux {
	h := handlers.NewToolDiscoveryHandlers(testDB, d, nil)
	r := chi.NewRouter()
	r.Post("/api/v1/mcp/tool-discoveries", h.Discover)
	return r
}

func postDiscovery(t *testing.T, r http.Handler, ctx context.Context, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/mcp/tool-discoveries", bytes.NewBufferString(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestToolDiscovery_Authorization(t *testing.T) {
	resetTables(t)
	ctx := context.Background()
	pubID := seedPublisher(t, "acme", "Acme")
	seedPublisher(t, "other", "Other")
	editor, err := testDB.CreateUser(ctx, store.CreateUserParams{Email: "editor@acme.test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := testDB.CreateGrant(ctx, store.CreateGrantParams{
		PrincipalType: domain.PrincipalUser, PrincipalID: editor.ID,
		PublisherID: pubID, Role: domain.RoleEditor,
	}); err != nil {
		t.Fatalf("CreateGrant: %v", err)
	}
	editorCtx := auth.ContextWithPrincipal(ctx, &auth.Principal{UserID: editor.ID, Email: editor.Email})
	ok := &tooldiscovery.Result{Tools: []domain.MCPTool{}}

	tests := []struct {
		name       string
		ctx        context.Context
		body       string
		wantStatus int
	}{
		{"anonymous", ctx, `{"namespace":"acme","url":"https://mcp.acme.dev","transport":"sse"}`, http.StatusUnauthorized},
		{"editor on the publisher", editorCtx, `{"namespace":"acme","url":"https://mcp.acme.dev","transport":"sse"}`, http.StatusOK},
		{"editor elsewhere only", editorCtx, `{"namespace":"other","url":"https://mcp.acme.dev","transport":"sse"}`, http.StatusForbidden},
		{"server admin", adminCtx(), `{"namespace":"other","url":"https://mcp.acme.dev","transport":"sse"}`, http.StatusOK},
		{"unknown publisher", adminCtx(), `{"namespace":"ghost","url":"https://mcp.acme.dev","transport":"sse"}`, http.StatusUnprocessableEntity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := postDiscovery(t, newDiscoveryRouter(&fakeDiscoverer{res: ok}), tt.ctx, tt.body)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d; body: %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestToolDiscovery_Validation(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	tests := []struct {
		name string
		body string
	}{
		{"missing url", `{"namespace":"acme","transport":"sse"}`},
		{"stdio transport", `{"namespace":"acme","url":"https://mcp.acme.dev","transport":"stdio"}`},
		{"not http", `{"namespace":"acme","url":"file:///etc/passwd","transport":"sse"}`},
		{"unknown field", `{"namespace":"acme","url":"https://mcp.acme.dev","transport":"sse","authorization":"x"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := postDiscovery(t, newDiscoveryRouter(&fakeDiscoverer{}), adminCtx(), tt.body)
			assertProblemType(t, rec, http.StatusUnprocessableEntity, "validation-error")
		})
	}
}

func TestToolDiscovery_PassesDeclaredEndpoint(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	tests := []struct {
		transport string
		want      tooldiscovery.Transport
	}{
		{"http", tooldiscovery.StreamableHTTP},
		{"streamable_http", tooldiscovery.StreamableHTTP},
		{"sse", tooldiscovery.SSE},
	}
	for _, tt := range tests {
		t.Run(tt.transport, func(t *testing.T) {
			d := &fakeDiscoverer{res: &tooldiscovery.Result{Tools: []domain.MCPTool{}}}
			rec := postDiscovery(t, newDiscoveryRouter(d), adminCtx(),
				fmt.Sprintf(`{"namespace":"acme","url":"https://mcp.acme.dev/x","transport":%q}`, tt.transport))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
			}
			if d.gotURL != "https://mcp.acme.dev/x" || d.gotTransport != tt.want {
				t.Errorf("discoverer got %q %q, want %q", d.gotURL, d.gotTransport, tt.want)
			}
		})
	}
}

func TestToolDiscovery_Disabled(t *testing.T) {
	rec := postDiscovery(t, newDiscoveryRouter(nil), adminCtx(),
		`{"namespace":"acme","url":"https://mcp.acme.dev","transport":"sse"}`)
	assertProblemType(t, rec, http.StatusServiceUnavailable, "tool-discovery-disabled")
}

func TestToolDiscovery_MapsFailures(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")
	tried := &tooldiscovery.Result{Attempts: []tooldiscovery.Attempt{
		{URL: "https://mcp.acme.dev/x", Transport: tooldiscovery.StreamableHTTP, Status: 404, Error: "HTTP 404"},
	}}
	tests := []struct {
		err        error
		wantStatus int
		wantSlug   string
	}{
		{fmt.Errorf("wrapped: %w", tooldiscovery.ErrBlockedAddress), http.StatusUnprocessableEntity, "blocked-address"},
		{tooldiscovery.ErrUnauthorized, http.StatusBadGateway, "upstream-unauthorized"},
		{tooldiscovery.ErrTooManyTools, http.StatusBadGateway, "too-many-tools"},
		{tooldiscovery.ErrTimeout, http.StatusGatewayTimeout, "timeout"},
		{tooldiscovery.ErrNoServer, http.StatusBadGateway, "no-mcp-server"},
	}
	for _, tt := range tests {
		t.Run(tt.wantSlug, func(t *testing.T) {
			rec := postDiscovery(t, newDiscoveryRouter(&fakeDiscoverer{res: tried, err: tt.err}), adminCtx(),
				`{"namespace":"acme","url":"https://mcp.acme.dev/x","transport":"http"}`)
			body := rec.Body.String()
			assertProblemType(t, rec, tt.wantStatus, tt.wantSlug)
			var got struct {
				Detail   string `json:"detail"`
				Attempts []struct {
					URL       string `json:"url"`
					Transport string `json:"transport"`
					Status    int    `json:"status"`
					Error     string `json:"error"`
				} `json:"attempts"`
			}
			if err := json.Unmarshal([]byte(body), &got); err != nil {
				t.Fatalf("decode: %v: %s", err, body)
			}
			if len(got.Attempts) != 1 || got.Attempts[0].URL != "https://mcp.acme.dev/x" ||
				got.Attempts[0].Transport != "streamable_http" || got.Attempts[0].Status != 404 ||
				got.Attempts[0].Error != "HTTP 404" {
				t.Errorf("attempts extension = %+v", got.Attempts)
			}
			want := "No MCP server answered. https://mcp.acme.dev/x (streamable_http): HTTP 404."
			if tt.wantSlug == "no-mcp-server" && got.Detail != want {
				t.Errorf("detail = %q, want %q", got.Detail, want)
			}
		})
	}
}

// TestToolDiscovery_EndToEnd runs the real discoverer against an MCP server
// mounted under /mcp: declaring that path finds it, declaring the bare base
// URL does not.
func TestToolDiscovery_EndToEnd(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "acme", "Acme")

	srv := mcp.NewServer(&mcp.Implementation{Name: "github-tools", Version: "2.3.0"}, nil)
	srv.AddTool(&mcp.Tool{
		Name:        "list_issues",
		Description: "List issues in a repository.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{"repo": map[string]any{"type": "string"}}},
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{}, nil
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil))
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	d := tooldiscovery.New(tooldiscovery.Config{
		Timeout:          5 * time.Second,
		MaxTools:         10,
		MaxResponseBytes: 1 << 20,
		AllowedPrefixes:  []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	rec := postDiscovery(t, newDiscoveryRouter(d), adminCtx(),
		fmt.Sprintf(`{"namespace":"acme","url":%q,"transport":"streamable_http"}`, ts.URL))
	assertProblemType(t, rec, http.StatusBadGateway, "no-mcp-server")

	rec = postDiscovery(t, newDiscoveryRouter(d), adminCtx(),
		fmt.Sprintf(`{"namespace":"acme","url":%q,"transport":"streamable_http"}`, ts.URL+"/mcp"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body: %s", rec.Code, rec.Body.String())
	}
	var got struct {
		Endpoint struct {
			URL       string `json:"url"`
			Transport string `json:"transport"`
		} `json:"endpoint"`
		Attempts []struct {
			URL    string `json:"url"`
			Status int    `json:"status"`
			Error  string `json:"error"`
		} `json:"attempts"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"server_info"`
		ProtocolVersion           string           `json:"protocol_version"`
		SupportedProtocolVersions []string         `json:"supported_protocol_versions"`
		Tools                     []domain.MCPTool `json:"tools"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Endpoint.URL != ts.URL+"/mcp" || got.Endpoint.Transport != "streamable_http" {
		t.Errorf("endpoint = %+v", got.Endpoint)
	}
	if len(got.Attempts) != 1 || got.Attempts[0].URL != ts.URL+"/mcp" || got.Attempts[0].Status != 200 || got.Attempts[0].Error != "" {
		t.Errorf("attempts = %+v", got.Attempts)
	}
	if got.ServerInfo.Name != "github-tools" || got.ServerInfo.Version != "2.3.0" || got.ProtocolVersion == "" {
		t.Errorf("server = %+v %q", got.ServerInfo, got.ProtocolVersion)
	}
	if len(got.Tools) != 1 || got.Tools[0].Name != "list_issues" || string(got.Tools[0].Annotations) != `{"readOnlyHint":true}` {
		t.Fatalf("tools = %+v", got.Tools)
	}
	// The result must be accepted as-is by the version-create validation.
	raw, _ := json.Marshal(got.Tools)
	if err := domain.ValidateTools(raw); err != nil {
		t.Errorf("ValidateTools rejects discovered tools: %v", err)
	}
	if !slices.Contains(got.SupportedProtocolVersions, got.ProtocolVersion) {
		t.Errorf("supported_protocol_versions = %v, missing the negotiated %q", got.SupportedProtocolVersions, got.ProtocolVersion)
	}
	if _, err := domain.ValidateProtocolVersions(got.SupportedProtocolVersions); err != nil {
		t.Errorf("ValidateProtocolVersions rejects discovered revisions: %v", err)
	}
}
