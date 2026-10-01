package tooldiscovery

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"slices"
	"strings"
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

func TestEndpoint(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"https://mcp.acme.dev/github", "https://mcp.acme.dev/github", false},
		{"https://mcp.acme.dev/github/", "https://mcp.acme.dev/github/", false},
		{"https://mcp.acme.dev", "https://mcp.acme.dev", false},
		{"http://mcp.acme.dev/sse", "http://mcp.acme.dev/sse", false},
		{"https://mcp.acme.dev/x?tenant=a#frag", "https://mcp.acme.dev/x?tenant=a", false},
		{"ftp://mcp.acme.dev", "", true},
		{"/relative", "", true},
		{"https://user:pw@mcp.acme.dev", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Endpoint(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
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

func TestDiscover_DeclaredEndpoint(t *testing.T) {
	tests := []struct {
		name string
		path string
		tr   Transport
	}{
		{"streamable http", "/github", StreamableHTTP},
		{"sse", "/github", SSE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := serve(t, tt.path, tt.tr, newMCPServer(3, 0))

			res, err := newDiscoverer(t, nil).Discover(context.Background(), ts.URL+tt.path+"#frag", tt.tr)
			if err != nil {
				t.Fatalf("Discover: %v", err)
			}
			if res.Endpoint.URL != ts.URL+tt.path || res.Endpoint.Transport != tt.tr {
				t.Errorf("endpoint = %+v", res.Endpoint)
			}
			if res.Endpoint.Status != http.StatusOK || res.Endpoint.Error != "" {
				t.Errorf("successful attempt = %+v", res.Endpoint)
			}
			if len(res.Attempts) != 1 || res.Attempts[0] != res.Endpoint {
				t.Errorf("attempts = %+v", res.Attempts)
			}
			if res.ServerName != "github-tools" || res.ServerVersion != "2.3.0" || res.ProtocolVersion == "" {
				t.Errorf("server info = %q %q %q", res.ServerName, res.ServerVersion, res.ProtocolVersion)
			}
			if len(res.Tools) != 3 || res.Tools[0].Name != "tool_00" || res.Tools[0].Description != "Tool number 0." {
				t.Errorf("tools = %+v", res.Tools)
			}
		})
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

// TestDiscover_NoGuessing checks that neither a conventional subpath nor the
// other transport is tried when the declared pair does not answer.
func TestDiscover_NoGuessing(t *testing.T) {
	tests := []struct {
		name     string
		mount    string
		served   Transport
		declared string
		tr       Transport
	}{
		{"server under /mcp, base URL declared", "/mcp", StreamableHTTP, "", StreamableHTTP},
		{"server under /sse, base URL declared", "/sse", SSE, "", SSE},
		{"sse server, streamable http declared", "/x", SSE, "/x", StreamableHTTP},
		{"streamable http server, sse declared", "/x", StreamableHTTP, "/x", SSE},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := serve(t, tt.mount, tt.served, newMCPServer(1, 0))

			res, err := newDiscoverer(t, nil).Discover(context.Background(), ts.URL+tt.declared, tt.tr)
			if !errors.Is(err, ErrNoServer) {
				t.Fatalf("err = %v, want ErrNoServer", err)
			}
			if len(res.Attempts) != 1 {
				t.Fatalf("got %d attempts, want 1: %+v", len(res.Attempts), res.Attempts)
			}
			if a := res.Attempts[0]; a.URL != ts.URL+tt.declared || a.Transport != tt.tr || a.Error == "" {
				t.Errorf("attempt = %+v", a)
			}
		})
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

	selfSigned := httptest.NewUnstartedServer(http.NotFoundHandler())
	selfSigned.Config.ErrorLog = log.New(io.Discard, "", 0)
	selfSigned.StartTLS()
	t.Cleanup(selfSigned.Close)
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	tests := []struct {
		name         string
		url          string
		cfg          func(*Config)
		want         error
		wantAttempts int
		// wantCause is a substring of the first attempt's error.
		wantCause string
	}{
		{"unauthorized stops at once", unauthorized.URL, nil, ErrUnauthorized, 1, "HTTP 401"},
		{"nothing answers", notFound.URL, nil, ErrNoServer, 1, "HTTP 404"},
		{"html page is not an MCP server", notMCP.URL + "/mcp", nil, ErrNoServer, 1, "content type"},
		{"untrusted certificate", selfSigned.URL + "/mcp", nil, ErrNoServer, 1, "tls: "},
		{"connection refused", closed.URL + "/mcp", nil, ErrNoServer, 1, "connection refused"},
		{"loopback is blocked by default", notFound.URL, func(c *Config) { c.AllowedPrefixes = nil }, ErrBlockedAddress, 1, "127.0.0.1"},
		{"budget runs out", slow.URL, func(c *Config) { c.Timeout = 300 * time.Millisecond }, ErrTimeout, 1, "timed out"},
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
			if got := res.Attempts[0].Error; !strings.Contains(got, tt.wantCause) {
				t.Errorf("first attempt error = %q, want it to contain %q", got, tt.wantCause)
			}
		})
	}
}

func TestTransportCause(t *testing.T) {
	dnsErr := &net.DNSError{Err: "no such host", Name: "mcp.acme.invalid", IsNotFound: true}
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			"certificate signed by unknown authority",
			&url.Error{Op: "Post", URL: "https://mcp.acme.dev", Err: &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}},
			"tls: x509: certificate signed by unknown authority",
		},
		{
			"hostname mismatch",
			x509.HostnameError{Certificate: &x509.Certificate{}, Host: "mcp.acme.dev"},
			"tls: x509: certificate is not valid for any names, but wanted to match mcp.acme.dev",
		},
		{
			"plain http on a tls port",
			tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"},
			"tls: first record does not look like a TLS handshake",
		},
		{
			"dns failure keeps the dial context",
			&url.Error{Op: "Post", URL: "https://mcp.acme.invalid", Err: &net.OpError{Op: "dial", Net: "tcp", Err: dnsErr}},
			"dial tcp: lookup mcp.acme.invalid: no such host",
		},
		{"bare dns failure", dnsErr, "lookup mcp.acme.invalid: no such host"},
		{
			"url error loses its request line",
			&url.Error{Op: "Post", URL: "https://mcp.acme.dev", Err: errors.New("proxyconnect tcp: refused")},
			"proxyconnect tcp: refused",
		},
		{"unknown error is kept", errors.New("boom"), "boom"},
		{"long error is cut on a rune boundary", errors.New(strings.Repeat("é", 400)), strings.Repeat("é", maxCauseLen/2) + "…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := transportCause(tt.err); got != tt.want {
				t.Errorf("transportCause() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClassify(t *testing.T) {
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connect: connection refused")}
	expired, cancel := context.WithCancel(context.Background())
	cancel()
	bg := context.Background()
	tests := []struct {
		name      string
		ctx       context.Context
		rec       *recorder
		status    int
		sdkErr    error
		wantCause string
		want      error
	}{
		{"blocked address wins", bg, &recorder{dialErr: ErrBlockedAddress}, 0, nil, ErrBlockedAddress.Error(), ErrBlockedAddress},
		{"401 means authentication", bg, &recorder{}, 401, errors.New("x"), "HTTP 401", ErrUnauthorized},
		{"403 means authentication", bg, &recorder{}, 403, errors.New("x"), "HTTP 403", ErrUnauthorized},
		{"expired budget", expired, &recorder{rtErr: refused}, 0, errors.New("x"), "timed out", ErrTimeout},
		{"transport error beats the sdk string", bg, &recorder{rtErr: refused}, 0, errors.New(`calling "initialize": x`), "dial tcp: connect: connection refused", ErrNoServer},
		{"http status", bg, &recorder{}, 404, errors.New("x"), "HTTP 404", ErrNoServer},
		{"sdk protocol error is kept", bg, &recorder{}, 200, errors.New(`calling "initialize": unsupported protocol version`), `calling "initialize": unsupported protocol version`, ErrNoServer},
		{"nothing known", bg, &recorder{}, 0, nil, "connection failed", ErrNoServer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cause, err := classify(tt.ctx, tt.rec, tt.status, tt.sdkErr)
			if cause != tt.wantCause {
				t.Errorf("cause = %q, want %q", cause, tt.wantCause)
			}
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
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
