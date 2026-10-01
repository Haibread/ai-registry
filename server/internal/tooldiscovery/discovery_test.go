package tooldiscovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var loopback = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}

func newDiscoverer(t *testing.T, mutate func(*Config)) *Discoverer {
	t.Helper()
	cfg := Config{
		Timeout:          5 * time.Second,
		MaxTools:         100,
		MaxResponseBytes: 1 << 20,
		AllowedPrefixes:  loopback,
		ClientVersion:    "test",
	}
	if mutate != nil {
		mutate(&cfg)
	}
	return New(cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func newMCPServer(nTools, pageSize int) *mcp.Server {
	return newMCPServerSpeaking(nTools, pageSize, nil)
}

func newMCPServerSpeaking(nTools, pageSize int, versions []string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "github-tools", Version: "2.3.0"},
		&mcp.ServerOptions{PageSize: pageSize, SupportedProtocolVersions: versions})
	noop := func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{}, nil
	}
	for i := range nTools {
		s.AddTool(&mcp.Tool{
			Name:        fmt.Sprintf("tool_%02d", i),
			Description: fmt.Sprintf("Tool number %d.", i),
			InputSchema: map[string]any{"type": "object"},
		}, noop)
	}
	return s
}

// serve mounts an MCP server at path, speaking the given transport; every
// other path answers 404.
func serve(t *testing.T, path string, tr Transport, s *mcp.Server) *httptest.Server {
	t.Helper()
	get := func(*http.Request) *mcp.Server { return s }
	var h http.Handler
	if tr == SSE {
		h = mcp.NewSSEHandler(get, nil)
	} else {
		h = mcp.NewStreamableHTTPHandler(get, nil)
	}
	return mount(t, path, h)
}

// serveStateless mounts a stateless streamable HTTP server, the only kind the
// SDK lets speak the 2026-07-28 revision.
func serveStateless(t *testing.T, path string, s *mcp.Server) *httptest.Server {
	t.Helper()
	get := func(*http.Request) *mcp.Server { return s }
	return mount(t, path, mcp.NewStreamableHTTPHandler(get, &mcp.StreamableHTTPOptions{Stateless: true}))
}

func mount(t *testing.T, path string, h http.Handler) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(path, h)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

func TestCandidates(t *testing.T) {
	tests := []struct {
		in      string
		want    []string
		wantErr bool
	}{
		{"https://mcp.acme.dev/github", []string{"https://mcp.acme.dev/github", "https://mcp.acme.dev/github/mcp", "https://mcp.acme.dev/github/sse"}, false},
		{"https://mcp.acme.dev/github/", []string{"https://mcp.acme.dev/github/", "https://mcp.acme.dev/github/mcp", "https://mcp.acme.dev/github/sse"}, false},
		{"https://mcp.acme.dev", []string{"https://mcp.acme.dev", "https://mcp.acme.dev/mcp", "https://mcp.acme.dev/sse"}, false},
		{"https://mcp.acme.dev/mcp", []string{"https://mcp.acme.dev/mcp"}, false},
		{"https://mcp.acme.dev/sse/", []string{"https://mcp.acme.dev/sse/"}, false},
		{"https://mcp.acme.dev/x?tenant=a#frag", []string{"https://mcp.acme.dev/x?tenant=a", "https://mcp.acme.dev/x/mcp?tenant=a", "https://mcp.acme.dev/x/sse?tenant=a"}, false},
		{"ftp://mcp.acme.dev", nil, true},
		{"/relative", nil, true},
		{"https://user:pw@mcp.acme.dev", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Candidates(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTransportOrder(t *testing.T) {
	tests := []struct {
		endpoint string
		hint     Transport
		want     Transport
	}{
		{"https://a.dev/x", StreamableHTTP, StreamableHTTP},
		{"https://a.dev/x", SSE, SSE},
		{"https://a.dev/x/sse", StreamableHTTP, SSE},
		{"https://a.dev/x/mcp", SSE, StreamableHTTP},
	}
	for _, tt := range tests {
		got := transportOrder(tt.endpoint, tt.hint)
		if len(got) != 2 || got[0] != tt.want || got[0] == got[1] {
			t.Errorf("transportOrder(%q, %q) = %v, want %q first", tt.endpoint, tt.hint, got, tt.want)
		}
	}
}

func TestGuardPermits(t *testing.T) {
	g := guard{}
	allowInternal := guard{allowed: []netip.Prefix{netip.MustParsePrefix("10.20.0.0/16")}}
	tests := []struct {
		g    guard
		addr string
		want bool
	}{
		{g, "93.184.216.34", true},
		{g, "2606:4700::1111", true},
		{g, "127.0.0.1", false},
		{g, "::1", false},
		{g, "10.0.4.12", false},
		{g, "172.16.0.1", false},
		{g, "192.168.1.1", false},
		{g, "169.254.169.254", false},
		{g, "fd00::1", false},
		{g, "fe80::1", false},
		{g, "100.64.0.1", false},
		{g, "0.0.0.0", false},
		{g, "::ffff:10.0.0.1", false},
		{g, "64:ff9b::a00:1", false},
		{allowInternal, "10.20.3.4", true},
		{allowInternal, "10.21.3.4", false},
	}
	for _, tt := range tests {
		if got := tt.g.permits(netip.MustParseAddr(tt.addr)); got != tt.want {
			t.Errorf("permits(%s) = %v, want %v", tt.addr, got, tt.want)
		}
	}
}

func TestDiscover_GuessesMCPSuffix(t *testing.T) {
	ts := serve(t, "/github/mcp", StreamableHTTP, newMCPServer(3, 0))

	res, err := newDiscoverer(t, nil).Discover(context.Background(), ts.URL+"/github", StreamableHTTP)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if res.Endpoint.URL != ts.URL+"/github/mcp" || res.Endpoint.Transport != StreamableHTTP {
		t.Errorf("endpoint = %+v", res.Endpoint)
	}
	if res.Endpoint.Status != http.StatusOK || res.Endpoint.Error != "" {
		t.Errorf("successful attempt = %+v", res.Endpoint)
	}
	if res.ServerName != "github-tools" || res.ServerVersion != "2.3.0" || res.ProtocolVersion == "" {
		t.Errorf("server info = %q %q %q", res.ServerName, res.ServerVersion, res.ProtocolVersion)
	}
	if len(res.Tools) != 3 || res.Tools[0].Name != "tool_00" || res.Tools[0].Description != "Tool number 0." {
		t.Errorf("tools = %+v", res.Tools)
	}
	first := res.Attempts[0]
	if first.URL != ts.URL+"/github" || first.Status != http.StatusNotFound || first.Error != "HTTP 404" {
		t.Errorf("first attempt = %+v", first)
	}
}

func TestDiscover_ProbesProtocolVersions(t *testing.T) {
	legacy := slices.DeleteFunc(mcp.SupportedProtocolVersions(), func(v string) bool { return v >= "2026-07-28" })
	tests := []struct {
		name      string
		tr        Transport
		stateless bool
		versions  []string
		want      []string
	}{
		{"stateful server", StreamableHTTP, false, nil, legacy},
		{"stateless server", StreamableHTTP, true, nil, mcp.SupportedProtocolVersions()},
		{"a subset", StreamableHTTP, false, []string{"2025-06-18", "2025-03-26"}, []string{"2025-06-18", "2025-03-26"}},
		{"newest only", StreamableHTTP, true, []string{"2026-07-28"}, []string{"2026-07-28"}},
		{"legacy sse subset", SSE, false, []string{"2025-03-26", "2024-11-05"}, []string{"2025-03-26", "2024-11-05"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newMCPServerSpeaking(1, 0, tt.versions)
			var ts *httptest.Server
			if tt.stateless {
				ts = serveStateless(t, "/mcp", s)
			} else {
				ts = serve(t, "/mcp", tt.tr, s)
			}

			res, err := newDiscoverer(t, nil).Discover(context.Background(), ts.URL+"/mcp", tt.tr)
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			if !slices.Equal(res.SupportedProtocolVersions, tt.want) {
				t.Errorf("supported = %v, want %v", res.SupportedProtocolVersions, tt.want)
			}
		})
	}
}

func TestDiscover_FallsBackToLegacySSE(t *testing.T) {
	ts := serve(t, "/sse", SSE, newMCPServer(2, 0))

	res, err := newDiscoverer(t, nil).Discover(context.Background(), ts.URL, StreamableHTTP)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if res.Endpoint.URL != ts.URL+"/sse" || res.Endpoint.Transport != SSE {
		t.Errorf("endpoint = %+v", res.Endpoint)
	}
	if len(res.Tools) != 2 {
		t.Errorf("got %d tools, want 2", len(res.Tools))
	}
}

func TestDiscover_FollowsPagination(t *testing.T) {
	ts := serve(t, "/mcp", StreamableHTTP, newMCPServer(7, 2))

	res, err := newDiscoverer(t, nil).Discover(context.Background(), ts.URL+"/mcp", StreamableHTTP)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(res.Tools) != 7 {
		t.Errorf("got %d tools, want 7", len(res.Tools))
	}
	if len(res.Attempts) != 1 {
		t.Errorf("got %d attempts, want 1", len(res.Attempts))
	}
}

func TestDiscover_TooManyTools(t *testing.T) {
	ts := serve(t, "/mcp", StreamableHTTP, newMCPServer(5, 2))

	_, err := newDiscoverer(t, func(c *Config) { c.MaxTools = 3 }).Discover(context.Background(), ts.URL+"/mcp", StreamableHTTP)
	if !errors.Is(err, ErrTooManyTools) {
		t.Fatalf("err = %v, want ErrTooManyTools", err)
	}
}

func TestDiscover_NoToolsCapability(t *testing.T) {
	ts := serve(t, "/mcp", StreamableHTTP, newMCPServer(0, 0))

	res, err := newDiscoverer(t, nil).Discover(context.Background(), ts.URL+"/mcp", StreamableHTTP)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if res.Tools == nil || len(res.Tools) != 0 {
		t.Errorf("tools = %#v, want empty non-nil slice", res.Tools)
	}
}

func TestDiscover_Failures(t *testing.T) {
	unauthorized := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(unauthorized.Close)
	notFound := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(notFound.Close)
	notMCP := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "<html>hello</html>")
	}))
	t.Cleanup(notMCP.Close)
	slow := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		// The server only notices a client hang-up once the body is drained.
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
	}))
	t.Cleanup(slow.Close)

	tests := []struct {
		name         string
		url          string
		cfg          func(*Config)
		want         error
		wantAttempts int
	}{
		{"unauthorized stops at once", unauthorized.URL, nil, ErrUnauthorized, 1},
		{"nothing answers", notFound.URL, nil, ErrNoServer, 6},
		{"html page is not an MCP server", notMCP.URL + "/mcp", nil, ErrNoServer, 2},
		{"loopback is blocked by default", notFound.URL, func(c *Config) { c.AllowedPrefixes = nil }, ErrBlockedAddress, 1},
		{"budget runs out", slow.URL, func(c *Config) { c.Timeout = 300 * time.Millisecond }, ErrTimeout, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := newDiscoverer(t, tt.cfg).Discover(context.Background(), tt.url, StreamableHTTP)
			if !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
			if len(res.Attempts) != tt.wantAttempts {
				t.Errorf("got %d attempts, want %d: %+v", len(res.Attempts), tt.wantAttempts, res.Attempts)
			}
		})
	}
}

func TestToDomainTool_NormalisesAnnotations(t *testing.T) {
	no := false
	tests := []struct {
		name string
		in   *mcp.ToolAnnotations
		want string
	}{
		{"nil", nil, ""},
		{"all defaults", &mcp.ToolAnnotations{}, ""},
		{"read only", &mcp.ToolAnnotations{ReadOnlyHint: true}, `{"readOnlyHint":true}`},
		{"explicit non-destructive", &mcp.ToolAnnotations{DestructiveHint: &no, Title: "Merge"}, `{"destructiveHint":false,"title":"Merge"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toDomainTool(&mcp.Tool{Name: "x", Annotations: tt.in, InputSchema: map[string]any{"type": "object"}})
			if err != nil {
				t.Fatal(err)
			}
			if string(got.Annotations) != tt.want {
				t.Errorf("annotations = %s, want %s", got.Annotations, tt.want)
			}
			var schema map[string]any
			if err := json.Unmarshal(got.InputSchema, &schema); err != nil || schema["type"] != "object" {
				t.Errorf("input_schema = %s", got.InputSchema)
			}
		})
	}
}
