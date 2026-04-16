// Package http wires together the chi router, middleware, and handlers.
package http

import (
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/http/middleware"
	"github.com/haibread/ai-registry/internal/observability"
	"github.com/haibread/ai-registry/internal/store"
)

// RouterDeps bundles the dependencies injected into the router.
type RouterDeps struct {
	Logger      *slog.Logger
	DB          *store.DB
	Metrics     *observability.Metrics
	AuthConf    auth.Config
	CORSOrigins []string
	// TrustedProxy, when non-nil, is the CIDR from which X-Forwarded-For
	// headers are trusted for client IP extraction in rate limiting.
	// Set via TRUSTED_PROXY_CIDR env var. Leave nil when not behind a proxy.
	TrustedProxy *net.IPNet
	// PublicRateLimitRPM is the per-IP request budget for unauthenticated
	// reads on /api/v1, in requests per minute. Zero falls back to 1000.
	PublicRateLimitRPM int
}

// NewRouter builds and returns the fully wrapped HTTP handler: the chi router
// with all middleware and routes, wrapped in otelhttp instrumentation.
func NewRouter(deps RouterDeps) http.Handler {
	mux := buildMux(deps)
	return otelhttp.NewHandler(mux, "",
		otelhttp.WithSpanNameFormatter(func(_ string, req *http.Request) string {
			rctx := chi.RouteContext(req.Context())
			if rctx != nil && rctx.RoutePattern() != "" {
				return req.Method + " " + rctx.RoutePattern()
			}
			return req.Method + " " + req.URL.Path
		}),
	)
}

// NewRouterForTest returns the raw *chi.Mux without the otelhttp wrapper so
// tests can use chi.Walk to enumerate registered routes. Production code
// should always call NewRouter instead.
func NewRouterForTest(deps RouterDeps) *chi.Mux {
	return buildMux(deps)
}

// buildMux constructs the chi router with middleware and routes. It is the
// unwrapped inner of NewRouter, exported to tests via NewRouterForTest.
func buildMux(deps RouterDeps) *chi.Mux {
	// ── Auth validator ────────────────────────────────────────────────────────
	jwksCache := auth.NewJWKSCache(deps.AuthConf.JWKSEndpoint(), 0)
	validator := auth.NewValidator(jwksCache, deps.AuthConf.OIDCIssuer)

	// ── Handlers ──────────────────────────────────────────────────────────────
	mcpH := handlers.NewMCPHandlers(deps.DB, deps.DB, deps.Metrics)
	v0H := handlers.NewV0MCPHandlers(deps.DB, deps.DB)
	agentH := handlers.NewAgentHandlers(deps.DB, deps.DB, deps.Metrics)
	pubH := handlers.NewPublisherHandlers(deps.DB, deps.DB)
	auditH := handlers.NewAuditHandlers(deps.DB)
	statsH := handlers.NewStatsHandlers(deps.DB)
	cardH := handlers.NewAgentCardHandlers(deps.DB, deps.Logger)
	reportH := handlers.NewReportHandlers(deps.DB)
	changelogH := handlers.NewChangelogHandlers(deps.DB)

	r := chi.NewRouter()

	// ── Core middleware ───────────────────────────────────────────────────────
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.CORS(deps.CORSOrigins))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(deps.Logger, deps.Metrics))
	r.Use(middleware.MaxBodySize(1 << 20)) // 1 MiB
	r.Use(middleware.RequireJSONBody)
	r.Use(validator.Authenticate) // parse JWT when present; never blocks

	// ── System endpoints ──────────────────────────────────────────────────────
	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz(deps.DB))
	r.With(auth.RequireAdmin).Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Get("/openapi.yaml", handlers.OpenAPISpec)
	r.Get("/docs", handlers.SwaggerUI)
	// Public runtime config consumed by the browser SPA (OIDC bootstrap).
	r.Get("/config.json", handlers.ConfigJSON(deps.AuthConf.OIDCIssuer, deps.AuthConf.OIDCClientID))

	// ── Well-known endpoints ──────────────────────────────────────────────────
	r.Get("/.well-known/oauth-protected-resource", handlers.OAuthProtectedResource)
	// Global registry agent card (makes the registry a first-class A2A citizen)
	r.Get("/.well-known/agent-card.json", cardH.GlobalAgentCard)

	// ── MCP registry wire-format compat layer ─────────────────────────────────
	r.Route("/v0", func(r chi.Router) {
		r.Get("/servers", v0H.ListServers)
		r.Get("/servers/{id}", v0H.GetServer)

		// Name-based routes (spec-preferred: namespace/slug path)
		r.Route("/servers/{namespace}/{slug}", func(r chi.Router) {
			r.Get("/", v0H.GetServerByName)
			r.With(auth.RequireAdmin).Patch("/status", v0H.PatchServerStatus)
			r.Route("/versions", func(r chi.Router) {
				r.Get("/", v0H.ListServerVersions)
				r.Route("/{version}", func(r chi.Router) {
					r.Get("/", v0H.GetServerVersion)
					r.With(auth.RequireAdmin).Put("/", v0H.UpdateServerVersion)
					r.With(auth.RequireAdmin).Delete("/", v0H.DeleteServerVersion)
					r.With(auth.RequireAdmin).Patch("/status", v0H.PatchVersionStatus)
				})
			})
		})

		r.With(auth.RequireAdmin).Post("/publish", v0H.Publish)
	})

	// ── Per-agent A2A card (public, outside /api/v1 per A2A spec path) ────────
	r.Get("/agents/{namespace}/{slug}/.well-known/agent-card.json", cardH.PerAgentCard)

	// ── API v1 ────────────────────────────────────────────────────────────────
	publicRLMax := deps.PublicRateLimitRPM
	if publicRLMax <= 0 {
		publicRLMax = 1000
	}
	publicRL := middleware.RateLimit(publicRLMax, time.Minute, deps.Metrics, deps.TrustedProxy)
	r.Route("/api/v1", func(r chi.Router) {

		// Publishers
		r.Route("/publishers", func(r chi.Router) {
			r.With(publicRL).Get("/", pubH.ListPublishers)
			r.With(auth.RequireAdmin).Post("/", pubH.CreatePublisher)
			r.With(publicRL).Get("/{slug}", pubH.GetPublisher)
			r.With(auth.RequireAdmin).Patch("/{slug}", pubH.PatchPublisher)
			r.With(auth.RequireAdmin).Delete("/{slug}", pubH.DeletePublisher)
		})

		// MCP servers
		r.Route("/mcp/servers", func(r chi.Router) {
			r.With(publicRL).Get("/", mcpH.ListServers)
			r.With(auth.RequireAdmin).Post("/", mcpH.CreateServer)

			r.Route("/{namespace}/{slug}", func(r chi.Router) {
				r.With(publicRL).Get("/", mcpH.GetServer)
				r.With(auth.RequireAdmin).Patch("/", mcpH.PatchServer)
				r.With(auth.RequireAdmin).Delete("/", mcpH.DeleteServer)
				r.With(auth.RequireAdmin).Post("/deprecate", mcpH.DeprecateServer)
				r.With(auth.RequireAdmin).Post("/visibility", mcpH.SetVisibility)
				r.With(publicRL).Post("/view", mcpH.RecordView)
				r.With(publicRL).Post("/copy", mcpH.RecordCopy)
				r.With(publicRL).Get("/activity", mcpH.ListMCPServerActivity)

				r.Route("/versions", func(r chi.Router) {
					r.With(publicRL).Get("/", mcpH.ListVersions)
					r.With(auth.RequireAdmin).Post("/", mcpH.CreateVersion)
					r.With(publicRL).Get("/{version}", mcpH.GetVersion)
					r.With(auth.RequireAdmin).Post("/{version}/publish", mcpH.PublishVersion)
				})
			})
		})

		// Agents
		r.Route("/agents", func(r chi.Router) {
			r.With(publicRL).Get("/", agentH.ListAgents)
			r.With(auth.RequireAdmin).Post("/", agentH.CreateAgent)

			r.Route("/{namespace}/{slug}", func(r chi.Router) {
				r.With(publicRL).Get("/", agentH.GetAgent)
				r.With(auth.RequireAdmin).Patch("/", agentH.PatchAgent)
				r.With(auth.RequireAdmin).Delete("/", agentH.DeleteAgent)
				r.With(auth.RequireAdmin).Post("/deprecate", agentH.DeprecateAgent)
				r.With(auth.RequireAdmin).Post("/visibility", agentH.SetVisibility)
				r.With(publicRL).Post("/view", agentH.RecordView)
				r.With(publicRL).Post("/copy", agentH.RecordCopy)
				r.With(publicRL).Get("/activity", agentH.ListAgentActivity)

				r.Route("/versions", func(r chi.Router) {
					r.With(publicRL).Get("/", agentH.ListVersions)
					r.With(auth.RequireAdmin).Post("/", agentH.CreateVersion)
					r.With(publicRL).Get("/{version}", agentH.GetVersion)
					r.With(auth.RequireAdmin).Post("/{version}/publish", agentH.PublishVersion)
					r.With(auth.RequireAdmin).Patch("/{version}/status", agentH.PatchVersionStatus)
				})
			})
		})

		// Public stats (published + public only, no auth required)
		r.With(publicRL).Get("/public-stats", statsH.GetPublicStats)

		// Public changelog (aggregated recent version publications)
		r.With(publicRL).Get("/changelog", changelogH.GetChangelog)

		// Stats (admin — includes private entries)
		r.With(auth.RequireAdmin).Get("/stats", statsH.GetStats)

		// Audit log
		r.With(auth.RequireAdmin).Get("/audit", auditH.ListEvents)

		// Community issue reports
		r.Route("/reports", func(r chi.Router) {
			r.With(publicRL).Post("/", reportH.CreateReport)
			r.With(auth.RequireAdmin).Get("/", reportH.ListReports)
			r.With(auth.RequireAdmin).Patch("/{id}", reportH.PatchReport)
		})
	})

	return r
}
