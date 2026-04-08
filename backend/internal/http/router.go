// Package http wires together the chi router, middleware, and handlers.
package http

import (
	"log/slog"
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
}

// NewRouter builds and returns the chi router with all middleware and routes.
func NewRouter(deps RouterDeps) http.Handler {
	// ── Auth validator ────────────────────────────────────────────────────────
	jwksCache := auth.NewJWKSCache(deps.AuthConf.JWKSEndpoint(), 0)
	validator := auth.NewValidator(jwksCache, deps.AuthConf.OIDCIssuer)

	// ── Handlers ──────────────────────────────────────────────────────────────
	mcpH := handlers.NewMCPHandlers(deps.DB, deps.DB)
	v0H := handlers.NewV0MCPHandlers(deps.DB, deps.DB)
	agentH := handlers.NewAgentHandlers(deps.DB, deps.DB)
	pubH := handlers.NewPublisherHandlers(deps.DB, deps.DB)
	auditH := handlers.NewAuditHandlers(deps.DB)
	statsH := handlers.NewStatsHandlers(deps.DB)
	cardH := handlers.NewAgentCardHandlers(deps.DB)

	r := chi.NewRouter()

	// ── Core middleware ───────────────────────────────────────────────────────
	r.Use(middleware.CORS(deps.CORSOrigins))
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(deps.Logger))
	r.Use(validator.Authenticate) // parse JWT when present; never blocks

	// ── System endpoints ──────────────────────────────────────────────────────
	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz(deps.DB))
	r.With(auth.RequireAdmin).Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Get("/openapi.yaml", handlers.OpenAPISpec)

	// ── Well-known endpoints ──────────────────────────────────────────────────
	r.Get("/.well-known/oauth-protected-resource", handlers.OAuthProtectedResource)
	// Global registry agent card (makes the registry a first-class A2A citizen)
	r.Get("/.well-known/agent-card.json", handlers.GlobalAgentCard)

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
	publicRL := middleware.RateLimit(100, time.Minute)
	r.Route("/api/v1", func(r chi.Router) {

		// Publishers
		r.Route("/publishers", func(r chi.Router) {
			r.With(publicRL).Get("/", pubH.ListPublishers)
			r.With(auth.RequireAdmin).Post("/", pubH.CreatePublisher)
			r.With(publicRL).Get("/{slug}", pubH.GetPublisher)
		})

		// MCP servers
		r.Route("/mcp/servers", func(r chi.Router) {
			r.With(publicRL).Get("/", mcpH.ListServers)
			r.With(auth.RequireAdmin).Post("/", mcpH.CreateServer)

			r.Route("/{namespace}/{slug}", func(r chi.Router) {
				r.With(publicRL).Get("/", mcpH.GetServer)
				r.With(auth.RequireAdmin).Post("/deprecate", mcpH.DeprecateServer)
				r.With(auth.RequireAdmin).Post("/visibility", mcpH.SetVisibility)

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
				r.With(auth.RequireAdmin).Post("/deprecate", agentH.DeprecateAgent)
				r.With(auth.RequireAdmin).Post("/visibility", agentH.SetVisibility)

				r.Route("/versions", func(r chi.Router) {
					r.With(publicRL).Get("/", agentH.ListVersions)
					r.With(auth.RequireAdmin).Post("/", agentH.CreateVersion)
					r.With(publicRL).Get("/{version}", agentH.GetVersion)
					r.With(auth.RequireAdmin).Post("/{version}/publish", agentH.PublishVersion)
				})
			})
		})

		// Stats (admin — includes private entries)
		r.With(auth.RequireAdmin).Get("/stats", statsH.GetStats)

		// Audit log
		r.With(auth.RequireAdmin).Get("/audit", auditH.ListEvents)
	})

	// ── Wrap with OTel HTTP instrumentation ───────────────────────────────────
	return otelhttp.NewHandler(r, "",
		otelhttp.WithSpanNameFormatter(func(_ string, req *http.Request) string {
			rctx := chi.RouteContext(req.Context())
			if rctx != nil && rctx.RoutePattern() != "" {
				return req.Method + " " + rctx.RoutePattern()
			}
			return req.Method + " " + req.URL.Path
		}),
	)
}
