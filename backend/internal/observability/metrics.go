package observability

import (
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// Metrics holds all OTel metric instruments registered for the service.
// Initialise once via InitMetrics; use the exported fields in handlers and
// middleware.
type Metrics struct {
	// HTTP
	HTTPRequestsTotal   metric.Int64Counter
	HTTPRequestDuration metric.Float64Histogram

	// Registry counters (populated in Phase 2+)
	MCPServersTotal metric.Int64UpDownCounter
	AgentsTotal     metric.Int64UpDownCounter

	// Auth
	AuthFailures metric.Int64Counter

	// Rate limiting
	RateLimitHits metric.Int64Counter
}

// InitMetrics registers all metric instruments with the global MeterProvider.
// It must be called after Setup.
func InitMetrics() (*Metrics, error) {
	m := otel.GetMeterProvider().Meter("ai-registry")

	reqTotal, err := m.Int64Counter(
		"registry.http.requests.total",
		metric.WithDescription("Total HTTP requests received"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating registry.http.requests.total: %w", err)
	}

	reqDuration, err := m.Float64Histogram(
		"registry.http.request.duration",
		metric.WithDescription("HTTP request latency in milliseconds"),
		metric.WithUnit("ms"),
		metric.WithExplicitBucketBoundaries(5, 25, 100, 250, 500, 1000, 5000),
	)
	if err != nil {
		return nil, fmt.Errorf("creating registry.http.request.duration: %w", err)
	}

	mcpServersTotal, err := m.Int64UpDownCounter(
		"registry.mcp.servers.total",
		metric.WithDescription("Live count of MCP server entries"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating registry.mcp.servers.total: %w", err)
	}

	agentsTotal, err := m.Int64UpDownCounter(
		"registry.agents.total",
		metric.WithDescription("Live count of agent entries"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating registry.agents.total: %w", err)
	}

	authFailures, err := m.Int64Counter(
		"registry.auth.failures",
		metric.WithDescription("Authentication and authorisation failures"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating registry.auth.failures: %w", err)
	}

	rateLimitHits, err := m.Int64Counter(
		"registry.ratelimit.hits",
		metric.WithDescription("Rate-limit rejections"),
	)
	if err != nil {
		return nil, fmt.Errorf("creating registry.ratelimit.hits: %w", err)
	}

	return &Metrics{
		HTTPRequestsTotal:   reqTotal,
		HTTPRequestDuration: reqDuration,
		MCPServersTotal:     mcpServersTotal,
		AgentsTotal:         agentsTotal,
		AuthFailures:        authFailures,
		RateLimitHits:       rateLimitHits,
	}, nil
}
