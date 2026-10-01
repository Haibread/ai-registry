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
	Discover(ctx context.Context, rawURL string, hint tooldiscovery.Transport) (*tooldiscovery.Result, error)
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

// Discover connects to the declared remote URL, guesses the MCP endpoint and
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
	var hint tooldiscovery.Transport
	switch body.Transport {
	case "http", "streamable_http":
		hint = tooldiscovery.StreamableHTTP
	case "sse":
		hint = tooldiscovery.SSE
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

	res, err := h.discoverer.Discover(r.Context(), body.URL, hint)
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

func writeDiscoveryError(w http.ResponseWriter, r *http.Request, res *tooldiscovery.Result, err error) {
	switch {
	case errors.Is(err, tooldiscovery.ErrBlockedAddress):
		problem.Write(w, http.StatusUnprocessableEntity, "blocked-address",
			fmt.Sprintf("The registry does not connect to this address: %s.", err), r.URL.Path)
	case errors.Is(err, tooldiscovery.ErrUnauthorized):
		problem.Write(w, http.StatusBadGateway, "upstream-unauthorized",
			"This server requires authentication. Only public servers can be fetched.", r.URL.Path)
	case errors.Is(err, tooldiscovery.ErrTooManyTools):
		problem.Write(w, http.StatusBadGateway, "too-many-tools", err.Error(), r.URL.Path)
	case errors.Is(err, tooldiscovery.ErrTimeout):
		problem.Write(w, http.StatusGatewayTimeout, "timeout",
			"The server did not answer in time.", r.URL.Path)
	case errors.Is(err, tooldiscovery.ErrNoServer):
		problem.Write(w, http.StatusBadGateway, "no-mcp-server",
			"No MCP server answered. Tried "+triedURLs(res)+".", r.URL.Path)
	default:
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", err.Error(), r.URL.Path)
	}
}

func triedURLs(res *tooldiscovery.Result) string {
	if res == nil {
		return "nothing"
	}
	var urls []string
	seen := map[string]bool{}
	for _, a := range res.Attempts {
		if !seen[a.URL] {
			seen[a.URL] = true
			urls = append(urls, a.URL)
		}
	}
	return strings.Join(urls, ", ")
}

func attemptsToResponse(in []tooldiscovery.Attempt) []toolDiscoveryAttempt {
	out := make([]toolDiscoveryAttempt, len(in))
	for i, a := range in {
		out[i] = toolDiscoveryAttempt{URL: a.URL, Transport: string(a.Transport), Status: a.Status, Error: a.Error}
	}
	return out
}
