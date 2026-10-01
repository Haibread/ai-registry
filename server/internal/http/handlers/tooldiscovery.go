package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/observability"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
	"github.com/haibread/ai-registry/internal/tooldiscovery"
)

// ToolDiscoverer lists the tools and protocol revisions of a remote MCP server.
type ToolDiscoverer interface {
	Discover(ctx context.Context, rawURL string, tr tooldiscovery.Transport) (*tooldiscovery.Result, error)
}

// ToolDiscoveryHandlers serves POST /api/v1/mcp/tool-discoveries.
type ToolDiscoveryHandlers struct {
	db         *store.DB
	discoverer ToolDiscoverer
	metrics    *observability.Metrics
}

// NewToolDiscoveryHandlers builds the handlers; a nil discoverer makes the
// route answer 503.
func NewToolDiscoveryHandlers(db *store.DB, d ToolDiscoverer, metrics *observability.Metrics) *ToolDiscoveryHandlers {
	return &ToolDiscoveryHandlers{db: db, discoverer: d, metrics: metrics}
}

type toolDiscoveryEndpoint struct {
	URL       string `json:"url"`
	Transport string `json:"transport"`
}

type toolDiscoveryAttempt struct {
	URL       string `json:"url"`
	Transport string `json:"transport"`
	Status    int    `json:"status"`
	Error     string `json:"error,omitempty"`
}

type toolDiscoveryServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type toolDiscoveryResponse struct {
	Endpoint                  toolDiscoveryEndpoint    `json:"endpoint"`
	Attempts                  []toolDiscoveryAttempt   `json:"attempts"`
	ServerInfo                *toolDiscoveryServerInfo `json:"server_info,omitempty"`
	ProtocolVersion           string                   `json:"protocol_version"`
	SupportedProtocolVersions []string                 `json:"supported_protocol_versions"`
	Tools                     []domain.MCPTool         `json:"tools"`
}

// Discover connects to the declared remote URL with the declared transport and
// returns its tools and supported protocol revisions. Nothing is stored.
func (h *ToolDiscoveryHandlers) Discover(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAuthenticated(r.Context()) {
		problem.Write(w, http.StatusUnauthorized, "unauthorized",
			"Missing or invalid bearer token", r.URL.Path)
		return
	}
	if h.discoverer == nil {
		problem.Write(w, http.StatusServiceUnavailable, "tool-discovery-disabled",
			"Fetching tools from a server is disabled on this registry.", r.URL.Path)
		return
	}
	var body struct {
		Namespace string `json:"namespace"`
		URL       string `json:"url"`
		Transport string `json:"transport"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Namespace == "" || body.URL == "" || body.Transport == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"namespace, url, and transport are required", r.URL.Path)
		return
	}
	var transport tooldiscovery.Transport
	switch body.Transport {
	case "http", "streamable_http":
		transport = tooldiscovery.StreamableHTTP
	case "sse":
		transport = tooldiscovery.SSE
	default:
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"transport must be one of http, sse, streamable_http", r.URL.Path)
		return
	}
	if err := domain.ValidateHTTPURL("url", body.URL); err != nil {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}

	publisherID, err := h.db.GetPublisherBySlug(r.Context(), body.Namespace)
	if errors.Is(err, store.ErrNotFound) {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			fmt.Sprintf("publisher '%s' does not exist", body.Namespace), r.URL.Path)
		return
	}
	if err != nil {
		internalError(w, r, err)
		return
	}
	// Editor is the role that authors versions, which is what the result is for.
	ok, _, err := auth.CheckPublisherRole(r.Context(), h.db, publisherID, domain.RoleEditor)
	if err != nil {
		internalError(w, r, err)
		return
	}
	if !ok {
		problem.Write(w, http.StatusForbidden, "forbidden",
			"Insufficient role on this publisher: editor required to author resources.", r.URL.Path)
		return
	}

	res, err := h.discoverer.Discover(r.Context(), body.URL, transport)
	h.count(r.Context(), err)
	if err != nil {
		writeDiscoveryError(w, r, res, err)
		return
	}

	out := toolDiscoveryResponse{
		Endpoint:                  toolDiscoveryEndpoint{URL: res.Endpoint.URL, Transport: string(res.Endpoint.Transport)},
		Attempts:                  attemptsToResponse(res.Attempts),
		ProtocolVersion:           res.ProtocolVersion,
		SupportedProtocolVersions: res.SupportedProtocolVersions,
		Tools:                     res.Tools,
	}
	if res.ServerName != "" {
		out.ServerInfo = &toolDiscoveryServerInfo{Name: res.ServerName, Version: res.ServerVersion}
	}
	writeJSON(w, r, http.StatusOK, out)
}

func (h *ToolDiscoveryHandlers) count(ctx context.Context, err error) {
	if h.metrics == nil {
		return
	}
	outcome := "ok"
	switch {
	case err == nil:
	case errors.Is(err, tooldiscovery.ErrBlockedAddress):
		outcome = "blocked_address"
	case errors.Is(err, tooldiscovery.ErrUnauthorized):
		outcome = "upstream_unauthorized"
	case errors.Is(err, tooldiscovery.ErrTimeout):
		outcome = "timeout"
	case errors.Is(err, tooldiscovery.ErrTooManyTools):
		outcome = "too_many_tools"
	case errors.Is(err, tooldiscovery.ErrNoServer):
		outcome = "no_mcp_server"
	default:
		outcome = "invalid_url"
	}
	h.metrics.ToolDiscoveries.Add(ctx, 1, metric.WithAttributes(attribute.String("outcome", outcome)))
}

// toolDiscoveryProblem carries the attempts as an RFC 7807 extension member,
// so the caller sees why each endpoint failed.
type toolDiscoveryProblem struct {
	problem.Detail
	Attempts []toolDiscoveryAttempt `json:"attempts,omitempty"`
}

func writeDiscoveryError(w http.ResponseWriter, r *http.Request, res *tooldiscovery.Result, err error) {
	var status int
	var slug, detail string
	switch {
	case errors.Is(err, tooldiscovery.ErrBlockedAddress):
		status, slug = http.StatusUnprocessableEntity, "blocked-address"
		detail = fmt.Sprintf("The registry does not connect to this address: %s.", err)
	case errors.Is(err, tooldiscovery.ErrUnauthorized):
		status, slug = http.StatusBadGateway, "upstream-unauthorized"
		detail = "This server requires authentication. Only public servers can be fetched."
	case errors.Is(err, tooldiscovery.ErrTooManyTools):
		status, slug, detail = http.StatusBadGateway, "too-many-tools", err.Error()
	case errors.Is(err, tooldiscovery.ErrTimeout):
		status, slug = http.StatusGatewayTimeout, "timeout"
		detail = "The server did not answer in time."
	case errors.Is(err, tooldiscovery.ErrNoServer):
		status, slug = http.StatusBadGateway, "no-mcp-server"
		detail = "No MCP server answered. " + attemptFailures(res) + "."
	default:
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
		return
	}
	body := toolDiscoveryProblem{Detail: problem.New(status, slug, detail, r.URL.Path)}
	if res != nil {
		body.Attempts = attemptsToResponse(res.Attempts)
	}
	problem.WriteBody(w, status, body)
}

// attemptFailures reads "<url> (<transport>): <reason>" for each attempt.
func attemptFailures(res *tooldiscovery.Result) string {
	if res == nil || len(res.Attempts) == 0 {
		return "Nothing was tried"
	}
	parts := make([]string, len(res.Attempts))
	for i, a := range res.Attempts {
		parts[i] = fmt.Sprintf("%s (%s): %s", a.URL, a.Transport, a.Error)
	}
	return strings.Join(parts, "; ")
}

func attemptsToResponse(in []tooldiscovery.Attempt) []toolDiscoveryAttempt {
	out := make([]toolDiscoveryAttempt, len(in))
	for i, a := range in {
		out[i] = toolDiscoveryAttempt{URL: a.URL, Transport: string(a.Transport), Status: a.Status, Error: a.Error}
	}
	return out
}
