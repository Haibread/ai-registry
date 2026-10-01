// Package tooldiscovery connects to a remote MCP server on a publisher's
// behalf and returns the tools it advertises through tools/list and the
// protocol revisions it accepts, guessing the exact endpoint from the URL the
// publisher declared.
package tooldiscovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/haibread/ai-registry/internal/domain"
)

const tracerName = "ai-registry/tooldiscovery"

// Transport is the wire transport of one connection attempt.
type Transport string

const (
	StreamableHTTP Transport = "streamable_http"
	SSE            Transport = "sse"
)

var (
	// ErrUnauthorized reports that a server exists but refused the anonymous
	// connection (HTTP 401/403).
	ErrUnauthorized = errors.New("server requires authentication")
	// ErrNoServer reports that no candidate answered the MCP handshake.
	ErrNoServer = errors.New("no MCP server answered")
	// ErrTimeout reports that the discovery budget ran out.
	ErrTimeout = errors.New("discovery timed out")
	// ErrTooManyTools reports a tools/list larger than Config.MaxTools.
	ErrTooManyTools = errors.New("too many tools")
)

// Config bounds what a single discovery may do.
type Config struct {
	// Timeout is the budget for the whole discovery, every attempt included.
	Timeout time.Duration
	// MaxTools caps the number of tools read across tools/list pages.
	MaxTools int
	// MaxResponseBytes caps each HTTP response body read from the server.
	MaxResponseBytes int64
	// AllowedPrefixes are exempted from the private-address block, for
	// deployments whose MCP servers live on an internal network.
	AllowedPrefixes []netip.Prefix
	// ClientVersion is reported as clientInfo.version during initialize.
	ClientVersion string
}

// Attempt is one connection attempt against one candidate endpoint.
type Attempt struct {
	URL       string
	Transport Transport
	// Status is the HTTP status of the first successful response, or of the
	// first non-redirect one when the attempt failed; 0 when none arrived.
	Status int
	// Error is empty when the attempt succeeded.
	Error string
}

// Result is what a discovery found. Attempts is filled even on failure.
type Result struct {
	Endpoint        Attempt
	Attempts        []Attempt
	ServerName      string
	ServerVersion   string
	ProtocolVersion string
	// SupportedProtocolVersions lists, newest first, the revisions the server
	// accepted when offered one at a time.
	SupportedProtocolVersions []string
	Tools                     []domain.MCPTool
}

// Discoverer runs discoveries. It is safe for concurrent use.
type Discoverer struct {
	cfg       Config
	transport http.RoundTripper
	logger    *slog.Logger
}

// New builds a Discoverer whose connections refuse internal addresses.
func New(cfg Config, logger *slog.Logger) *Discoverer {
	g := guard{allowed: cfg.AllowedPrefixes}
	dialer := &net.Dialer{Timeout: cfg.Timeout, Control: g.control}
	// Proxy is nil on purpose: through a proxy the guard would vet the proxy's
	// address instead of the MCP server's.
	tr := &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   cfg.Timeout,
		ResponseHeaderTimeout: cfg.Timeout,
		MaxIdleConns:          10,
		IdleConnTimeout:       30 * time.Second,
	}
	return &Discoverer{cfg: cfg, transport: otelhttp.NewTransport(tr), logger: logger}
}

// Discover guesses the MCP endpoint behind rawURL, runs the MCP handshake and
// reads every tools/list page. hint is the transport the publisher declared;
// it is tried first on candidates that do not name a transport themselves.
func (d *Discoverer) Discover(ctx context.Context, rawURL string, hint Transport) (*Result, error) {
	ctx, span := otel.GetTracerProvider().Tracer(tracerName).Start(ctx, "mcp.tool_discovery",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attribute.String("mcp.discovery.declared_url", rawURL)))
	defer span.End()

	res, err := d.discover(ctx, rawURL, hint)
	if res != nil {
		span.SetAttributes(attribute.Int("mcp.discovery.attempts", len(res.Attempts)))
		if err == nil {
			span.SetAttributes(
				attribute.String("mcp.discovery.endpoint", res.Endpoint.URL),
				attribute.String("mcp.discovery.transport", string(res.Endpoint.Transport)),
				attribute.Int("mcp.discovery.tools", len(res.Tools)),
				attribute.StringSlice("mcp.discovery.protocol_versions", res.SupportedProtocolVersions))
		}
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return res, err
}

func (d *Discoverer) discover(ctx context.Context, rawURL string, hint Transport) (*Result, error) {
	ctx, cancel := context.WithTimeout(ctx, d.cfg.Timeout)
	defer cancel()

	cands, err := Candidates(rawURL)
	if err != nil {
		return nil, err
	}
	res := &Result{}
	for _, c := range cands {
		for _, tr := range transportOrder(c, hint) {
			if ctx.Err() != nil {
				return res, ErrTimeout
			}
			att, err := d.attempt(ctx, c, tr, res)
			res.Attempts = append(res.Attempts, att)
			d.logger.DebugContext(ctx, "tool discovery attempt",
				slog.String("url", c), slog.String("transport", string(tr)),
				slog.Int("status", att.Status), slog.String("error", att.Error))
			switch {
			case err == nil:
				res.Endpoint = att
				res.SupportedProtocolVersions = d.probeProtocolVersions(ctx, c, tr)
				return res, nil
			case errors.Is(err, ErrBlockedAddress), errors.Is(err, ErrUnauthorized),
				errors.Is(err, ErrTooManyTools):
				return res, err
			}
		}
	}
	if ctx.Err() != nil {
		return res, ErrTimeout
	}
	return res, ErrNoServer
}

// attempt connects to one candidate with one transport and, on success, fills
// res with the server identity and its tools.
func (d *Discoverer) attempt(ctx context.Context, endpoint string, tr Transport, res *Result) (Attempt, error) {
	att := Attempt{URL: endpoint, Transport: tr}
	rec := &recorder{ctx: ctx, base: d.transport, limit: d.cfg.MaxResponseBytes}
	sess, err := d.connect(ctx, endpoint, tr, rec, nil)
	att.Status = rec.firstStatus()
	if err != nil {
		att.Error, err = classify(ctx, rec, att.Status)
		return att, err
	}
	defer func() { _ = sess.Close() }()
	att.Status = rec.firstOKStatus()

	tools, err := d.listTools(ctx, sess)
	if err != nil {
		if errors.Is(err, ErrTooManyTools) {
			att.Error = err.Error()
			return att, err
		}
		att.Error, err = classify(ctx, rec, att.Status)
		return att, err
	}

	init := sess.InitializeResult()
	if init.ServerInfo != nil {
		res.ServerName = init.ServerInfo.Name
		res.ServerVersion = init.ServerInfo.Version
	}
	res.ProtocolVersion = init.ProtocolVersion
	res.Tools = tools
	return att, nil
}

func (d *Discoverer) connect(ctx context.Context, endpoint string, tr Transport, rec *recorder, opts *mcp.ClientSessionOptions) (*mcp.ClientSession, error) {
	httpClient := &http.Client{Transport: rec}
	var t mcp.Transport
	if tr == SSE {
		t = &mcp.SSEClientTransport{Endpoint: endpoint, HTTPClient: httpClient}
	} else {
		t = &mcp.StreamableClientTransport{
			Endpoint:             endpoint,
			HTTPClient:           httpClient,
			MaxRetries:           -1,
			DisableStandaloneSSE: true,
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "ai-registry", Version: d.cfg.ClientVersion}, nil)
	return client.Connect(ctx, t, opts)
}

// probeProtocolVersions offers the server every revision the SDK speaks, one
// handshake each. MCP negotiation has a server answer with the requested
// revision when it supports it and with another one otherwise.
func (d *Discoverer) probeProtocolVersions(ctx context.Context, endpoint string, tr Transport) []string {
	versions := mcp.SupportedProtocolVersions()
	accepted := make([]bool, len(versions))
	var wg sync.WaitGroup
	for i, v := range versions {
		wg.Go(func() { accepted[i] = d.accepts(ctx, endpoint, tr, v) })
	}
	wg.Wait()
	supported := []string{}
	for i, v := range versions {
		if accepted[i] {
			supported = append(supported, v)
		}
	}
	return supported
}

func (d *Discoverer) accepts(ctx context.Context, endpoint string, tr Transport, version string) bool {
	rec := &recorder{ctx: ctx, base: d.transport, limit: d.cfg.MaxResponseBytes}
	sess, err := d.connect(ctx, endpoint, tr, rec, &mcp.ClientSessionOptions{ProtocolVersion: version})
	if err != nil {
		d.logger.DebugContext(ctx, "protocol version probe failed",
			slog.String("url", endpoint), slog.String("version", version), slog.String("error", err.Error()))
		return false
	}
	defer func() { _ = sess.Close() }()
	negotiated := sess.InitializeResult().ProtocolVersion
	d.logger.DebugContext(ctx, "protocol version probe",
		slog.String("url", endpoint), slog.String("offered", version), slog.String("negotiated", negotiated))
	return negotiated == version
}

func (d *Discoverer) listTools(ctx context.Context, sess *mcp.ClientSession) ([]domain.MCPTool, error) {
	tools := []domain.MCPTool{}
	if init := sess.InitializeResult(); init.Capabilities == nil || init.Capabilities.Tools == nil {
		return tools, nil
	}
	for t, err := range sess.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		if len(tools) == d.cfg.MaxTools {
			return nil, fmt.Errorf("%w: the server lists more than %d", ErrTooManyTools, d.cfg.MaxTools)
		}
		dt, err := toDomainTool(t)
		if err != nil {
			return nil, err
		}
		tools = append(tools, dt)
	}
	return tools, nil
}

// classify turns a failed attempt into a short, user-facing reason and the
// sentinel the caller branches on.
func classify(ctx context.Context, rec *recorder, status int) (string, error) {
	switch {
	case rec.blocked() != nil:
		return rec.blocked().Error(), rec.blocked()
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return fmt.Sprintf("HTTP %d", status), ErrUnauthorized
	case ctx.Err() != nil:
		return "timed out", ErrTimeout
	case status >= 300:
		return fmt.Sprintf("HTTP %d", status), ErrNoServer
	case status == 0:
		return "connection failed", ErrNoServer
	default:
		return "not an MCP server", ErrNoServer
	}
}

// Candidates lists the endpoints tried for a declared URL, in order: the URL
// as given, then with the conventional /mcp and /sse path suffixes unless the
// path already ends with one of them.
func Candidates(rawURL string) ([]string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, fmt.Errorf("%q is not an absolute http(s) URL", rawURL)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%q must not embed credentials", rawURL)
	}
	u.Fragment = ""
	out := []string{u.String()}
	path := strings.TrimRight(u.Path, "/")
	switch path[strings.LastIndex(path, "/")+1:] {
	case "mcp", "sse":
		return out, nil
	}
	for _, suffix := range []string{"mcp", "sse"} {
		c := *u
		c.Path = path + "/" + suffix
		c.RawPath = ""
		out = append(out, c.String())
	}
	return out, nil
}

// transportOrder tries the transport a /mcp or /sse path names first, and
// otherwise the declared one; the other transport follows, as the MCP spec's
// backwards-compatibility procedure prescribes.
func transportOrder(endpoint string, hint Transport) []Transport {
	first := hint
	switch {
	case strings.HasSuffix(strings.TrimRight(endpointPath(endpoint), "/"), "/sse"):
		first = SSE
	case strings.HasSuffix(strings.TrimRight(endpointPath(endpoint), "/"), "/mcp"):
		first = StreamableHTTP
	}
	if first == SSE {
		return []Transport{SSE, StreamableHTTP}
	}
	return []Transport{StreamableHTTP, SSE}
}

func endpointPath(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	return u.Path
}

// toDomainTool maps a tools/list entry to the registry's stored shape.
// readOnlyHint and idempotentHint are dropped when false, their spec default,
// so a fetched tool compares equal to a hand-written one that omits them.
func toDomainTool(t *mcp.Tool) (domain.MCPTool, error) {
	out := domain.MCPTool{Name: t.Name, Description: t.Description}
	if t.InputSchema != nil {
		b, err := json.Marshal(t.InputSchema)
		if err != nil {
			return out, fmt.Errorf("tool %q: input schema: %w", t.Name, err)
		}
		out.InputSchema = b
	}
	if a := t.Annotations; a != nil {
		m := map[string]any{}
		if a.Title != "" {
			m["title"] = a.Title
		}
		if a.ReadOnlyHint {
			m["readOnlyHint"] = true
		}
		if a.IdempotentHint {
			m["idempotentHint"] = true
		}
		if a.DestructiveHint != nil {
			m["destructiveHint"] = *a.DestructiveHint
		}
		if a.OpenWorldHint != nil {
			m["openWorldHint"] = *a.OpenWorldHint
		}
		if len(m) > 0 {
			b, err := json.Marshal(m)
			if err != nil {
				return out, fmt.Errorf("tool %q: annotations: %w", t.Name, err)
			}
			out.Annotations = b
		}
	}
	return out, nil
}

// recorder observes the HTTP exchanges of one attempt, since the SDK's errors
// do not carry the status code or the dial failure, and caps response bodies.
// It also ties every request to ctx: the SDK sends the session DELETE on the
// connection's own context, which would otherwise outlive the discovery
// against a server that never answers.
type recorder struct {
	ctx   context.Context
	base  http.RoundTripper
	limit int64

	mu       sync.Mutex
	status   int
	okStatus int
	dialErr  error
}

func (r *recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx, cancel := context.WithCancel(req.Context())
	stop := context.AfterFunc(r.ctx, cancel)
	release := func() { stop(); cancel() }
	resp, err := r.base.RoundTrip(req.WithContext(ctx))
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		release()
		if r.dialErr == nil && errors.Is(err, ErrBlockedAddress) {
			r.dialErr = err
		}
		return nil, err
	}
	if r.status == 0 && (resp.StatusCode < 300 || resp.StatusCode >= 400) {
		r.status = resp.StatusCode
	}
	if r.okStatus == 0 && resp.StatusCode < 300 {
		r.okStatus = resp.StatusCode
	}
	left := r.limit
	if left <= 0 {
		left = math.MaxInt64
	}
	resp.Body = &limitedBody{rc: resp.Body, left: left, release: release}
	return resp, nil
}

func (r *recorder) firstStatus() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *recorder) firstOKStatus() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.okStatus
}

func (r *recorder) blocked() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.dialErr == nil {
		return nil
	}
	var be *blockedError
	if errors.As(r.dialErr, &be) {
		return be
	}
	return ErrBlockedAddress
}

type limitedBody struct {
	rc      io.ReadCloser
	left    int64
	release func()
}

var errBodyTooLarge = errors.New("response body exceeds the discovery size limit")

func (b *limitedBody) Read(p []byte) (int, error) {
	if b.left <= 0 {
		return 0, errBodyTooLarge
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	n, err := b.rc.Read(p)
	b.left -= int64(n)
	return n, err
}

func (b *limitedBody) Close() error {
	err := b.rc.Close()
	b.release()
	return err
}
