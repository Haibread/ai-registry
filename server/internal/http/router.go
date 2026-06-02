// Package http wires together the chi router, middleware, and handlers.
package http

import (
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
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
	CORSOrigins []string
	// TrustedProxy, when non-nil, is the CIDR from which X-Forwarded-For
	// headers are trusted for client IP extraction in rate limiting.
	// Set via TRUSTED_PROXY_CIDR env var. Leave nil when not behind a proxy.
	TrustedProxy *net.IPNet
	// PublicRateLimitRPM is the per-IP request budget for unauthenticated
	// reads on /api/v1, in requests per minute. Zero falls back to 1000.
	PublicRateLimitRPM int
	// PublicBaseURL is the externally reachable URL of this deployment.
	// Threaded into the well-known endpoints (oauth-protected-resource,
	// global agent-card). Empty triggers HTTP 500 in those handlers rather
	// than silently advertising localhost.
	PublicBaseURL string
	// Sessions issues / resolves the registry session cookie. Nil only in
	// route-walk tests.
	Sessions *auth.SessionManager
	// OIDC is the server-side OIDC broker (confidential client). Nil when OIDC
	// login is not configured — then /api/v1/auth/oidc/* report 404.
	OIDC *auth.OIDCBroker
	// LocalLoginEnabled mirrors AUTH_LOCAL_LOGIN_ENABLED; gates POST /auth/login.
	LocalLoginEnabled bool
	// OIDCEnabled reports whether the broker is configured, surfaced to the SPA
	// via /config.json so it knows whether to render the SSO button.
	OIDCEnabled bool
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
	// ── Session authenticator ───────────────────────────────────────────────
	// Resolves the session cookie → Principal. Only attach
	// a principal store when a real DB is present: a nil *store.DB would wrap a
	// typed-nil in the interface, so route-walk tests (nil DB) must not get one.
	var principalStore auth.PrincipalStore
	if deps.DB != nil {
		principalStore = deps.DB
	}
	authn := auth.NewAuthenticator(deps.Sessions, principalStore)

	oidcCookieSecure := true
	if deps.Sessions != nil {
		oidcCookieSecure = deps.Sessions.CookieSecure()
	}

	// ── Handlers ──────────────────────────────────────────────────────────────
	mcpH := handlers.NewMCPHandlers(deps.DB, deps.DB, deps.Metrics)
	agentH := handlers.NewAgentHandlers(deps.DB, deps.DB, deps.Metrics)
	pubH := handlers.NewPublisherHandlers(deps.DB, deps.DB)
	revH := handlers.NewReviewHandlers(deps.DB, deps.DB)
	auditH := handlers.NewAuditHandlers(deps.DB)
	statsH := handlers.NewStatsHandlers(deps.DB)
	cardH := handlers.NewAgentCardHandlers(deps.DB, deps.Logger, deps.PublicBaseURL)
	reportH := handlers.NewReportHandlers(deps.DB, deps.TrustedProxy)
	changelogH := handlers.NewChangelogHandlers(deps.DB)
	authH := handlers.NewAuthHandlers(deps.Sessions, deps.DB, deps.LocalLoginEnabled)
	// Post-logout the IdP redirects back to the SPA origin; derive it from the
	// public base URL (falls back to "/" when unset).
	postLogoutRedirect := "/"
	if deps.PublicBaseURL != "" {
		postLogoutRedirect = strings.TrimRight(deps.PublicBaseURL, "/") + "/"
	}
	oidcH := handlers.NewOIDCAuthHandlers(deps.OIDC, deps.Sessions, deps.DB, oidcCookieSecure, "/", postLogoutRedirect)
	groupH := handlers.NewGroupHandlers(deps.DB, deps.DB)
	userH := handlers.NewUserHandlers(deps.DB, deps.DB)
	grantH := handlers.NewGrantHandlers(deps.DB, deps.DB)
	meH := handlers.NewMeHandlers(deps.DB)

	// resolvePublisherSlug maps the {slug} path param to a publisher id for
	// RequirePublisherRole on the per-publisher grants routes.
	resolvePublisherSlug := func(r *http.Request) (string, error) {
		pub, err := deps.DB.GetPublisher(r.Context(), chi.URLParam(r, "slug"))
		if err != nil {
			return "", err
		}
		return pub.ID, nil
	}
	requirePublisherAdmin := auth.RequirePublisherRole(deps.DB, domain.RoleAdmin, resolvePublisherSlug)
	// Per-publisher dashboard reads are open to any member (Viewer satisfies via
	// the lattice; Server Admin short-circuits) — a non-member gets 403.
	requirePublisherViewer := auth.RequirePublisherRole(deps.DB, domain.RoleViewer, resolvePublisherSlug)

	// Publisher-scoped RBAC guards. A resource's {namespace} path
	// segment IS its owning publisher's slug, so the publisher is resolved from
	// the slug — not from the resource's publisher_id column — which keeps the
	// authorization decision a single publishers lookup. Writes require Editor,
	// per-resource approvals require Reviewer; Admin and Server Admin satisfy
	// either via the lattice.
	resolvePublisherByNamespace := func(r *http.Request) (string, error) {
		pub, err := deps.DB.GetPublisher(r.Context(), chi.URLParam(r, "namespace"))
		if err != nil {
			return "", err
		}
		return pub.ID, nil
	}
	requireMCPServerNS := auth.RequirePublisherRole(deps.DB, domain.RoleEditor, resolvePublisherByNamespace)
	requireAgentNS := auth.RequirePublisherRole(deps.DB, domain.RoleEditor, resolvePublisherByNamespace)
	requireReviewerNS := auth.RequirePublisherRole(deps.DB, domain.RoleReviewer, resolvePublisherByNamespace)

	r := chi.NewRouter()

	// ── Core middleware ───────────────────────────────────────────────────────
	// HSTS is emitted only when cookies are Secure (i.e. the deployment is
	// served over HTTPS); a plain-HTTP dev setup must not advertise it.
	hsts := false
	if deps.Sessions != nil {
		hsts = deps.Sessions.CookieSecure()
	}
	r.Use(middleware.SecurityHeaders(hsts))
	r.Use(middleware.CORS(deps.CORSOrigins))
	// CSRF defense: reject cross-site state-changing requests (Fetch Metadata /
	// Origin enforcement). Must run before any handler that mutates state.
	r.Use(middleware.EnforceSameOrigin(deps.CORSOrigins))
	r.Use(middleware.Recover(deps.Logger))
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(deps.Logger, deps.Metrics))
	r.Use(middleware.MaxBodySize(1 << 20)) // 1 MiB
	r.Use(middleware.RequireJSONBody)
	r.Use(authn.Authenticate) // resolve session cookie when present; never blocks

	// ── System endpoints ──────────────────────────────────────────────────────
	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz(deps.DB))
	// /metrics is intentionally unauthenticated: Prometheus scrapes it in-cluster
	// via the ClusterIP Service, and the ServiceMonitor cannot present the
	// session cookie that RequireAdmin needs. The payload is non-sensitive
	// (request counters, latency histograms, registry-entry gauges) and the
	// shipped Ingress does not route /metrics externally. Restrict pod-to-pod
	// access with a NetworkPolicy if your cluster needs it.
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Get("/openapi.yaml", handlers.OpenAPISpec)
	r.Get("/docs", handlers.SwaggerUI)
	// Public runtime config consumed by the browser SPA (sign-in feature flags).
	r.Get("/config.json", handlers.ConfigJSON(deps.OIDCEnabled, deps.LocalLoginEnabled))

	// ── Well-known endpoints ──────────────────────────────────────────────────
	// Global registry agent card (makes the registry a first-class A2A citizen)
	r.Get("/.well-known/agent-card.json", cardH.GlobalAgentCard)

	// ── Per-agent A2A card (public, outside /api/v1 per A2A spec path) ────────
	r.Get("/agents/{namespace}/{slug}/.well-known/agent-card.json", cardH.PerAgentCard)

	// ── API v1 ────────────────────────────────────────────────────────────────
	publicRLMax := deps.PublicRateLimitRPM
	if publicRLMax <= 0 {
		publicRLMax = 1000
	}
	publicRL := middleware.RateLimit(publicRLMax, time.Minute, deps.Metrics, deps.TrustedProxy)
	r.Route("/api/v1", func(r chi.Router) {

		// Auth front doors: local email+password login and
		// logout, plus the brokered OIDC login/callback (confidential client,
		// server-side). All unauthenticated by design; rate-limited and, for
		// local login, lockout-protected. Each ends in a session cookie.
		r.With(publicRL).Post("/auth/login", authH.Login)
		r.With(publicRL).Post("/auth/logout", authH.Logout)
		// Browser logout: revokes the session and, for an OIDC session, bounces
		// through the IdP's RP-initiated logout so the SSO session ends too.
		r.With(publicRL).Get("/auth/logout", oidcH.Logout)
		r.With(publicRL).Get("/auth/oidc/login", oidcH.Login)
		r.With(publicRL).Get("/auth/oidc/callback", oidcH.Callback)

		// Resolved identity + effective role grants for the authenticated
		// caller. Powers the SPA's role-gated admin UI. 401 when no
		// token; the handler resolves the principal/claims itself.
		r.Get("/me", meH.Me)

		// ── RBAC administration ─────────────────────────────────
		// Groups (Server Admin).
		r.Route("/groups", func(r chi.Router) {
			r.With(auth.RequireAdmin).Get("/", groupH.ListGroups)
			r.With(auth.RequireAdmin).Post("/", groupH.CreateGroup)
			r.With(auth.RequireAdmin).Get("/{slug}", groupH.GetGroup)
			r.With(auth.RequireAdmin).Patch("/{slug}", groupH.PatchGroup)
			r.With(auth.RequireAdmin).Delete("/{slug}", groupH.DeleteGroup)
			r.With(auth.RequireAdmin).Get("/{slug}/members", groupH.ListMembers)
			r.With(auth.RequireAdmin).Put("/{slug}/members/{email}", groupH.PutMember)
			r.With(auth.RequireAdmin).Delete("/{slug}/members/{email}", groupH.DeleteMember)
		})

		// Users (Server Admin), except set-password which is self-or-admin and
		// so is only authenticated — the handler enforces the rule.
		r.Route("/users", func(r chi.Router) {
			r.With(auth.RequireAdmin).Get("/", userH.ListUsers)
			r.With(auth.RequireAdmin).Post("/", userH.CreateUser)
			r.With(auth.RequireAdmin).Get("/{id}", userH.GetUser)
			r.With(auth.RequireAdmin).Patch("/{id}", userH.PatchUser)
			r.Post("/{id}/set-password", userH.SetPassword)
		})

		// Global (all-publishers) role grants (Server Admin).
		r.Route("/grants", func(r chi.Router) {
			r.With(auth.RequireAdmin).Get("/", grantH.ListGlobalGrants)
			r.With(auth.RequireAdmin).Post("/", grantH.CreateGlobalGrant)
			r.With(auth.RequireAdmin).Delete("/{id}", grantH.DeleteGlobalGrant)
		})

		// Review queue: authenticated; the handler resolves the caller's reviewer
		// scope (Server Admin / global Reviewer see every publisher, a
		// per-publisher Reviewer sees only the publishers they review, anyone
		// else gets 403) and filters results per-publisher.
		r.Get("/review-queue", revH.ListReviewQueue)

		// Publishers
		r.Route("/publishers", func(r chi.Router) {
			r.With(publicRL).Get("/", pubH.ListPublishers)
			r.With(auth.RequireAdmin).Post("/", pubH.CreatePublisher)
			r.With(publicRL).Get("/{slug}", pubH.GetPublisher)
			// Editing a publisher's metadata (name/contact) is a publisher
			// Admin action; Server Admin keeps break-glass via the
			// RequirePublisherRole short-circuit. Deletion removes the whole
			// tenant, so it stays Server-Admin-only.
			r.With(requirePublisherAdmin).Patch("/{slug}", pubH.PatchPublisher)
			r.With(auth.RequireAdmin).Delete("/{slug}", pubH.DeletePublisher)

			// Per-publisher admin home reads: scoped stats + activity
			// for any member of the publisher (Viewer+) or a Server Admin. This is
			// how a publisher Editor/Admin sees their own dashboard without the
			// Server-Admin-only global /stats and /audit.
			r.With(requirePublisherViewer).Get("/{slug}/stats", pubH.GetPublisherStats)
			r.With(requirePublisherViewer).Get("/{slug}/activity", pubH.GetPublisherActivity)

			// Per-publisher role grants. Publisher Admin or Server
			// Admin; RequirePublisherRole resolves {slug} → publisher.
			r.Route("/{slug}/grants", func(r chi.Router) {
				r.With(requirePublisherAdmin).Get("/", grantH.ListPublisherGrants)
				r.With(requirePublisherAdmin).Post("/", grantH.CreatePublisherGrant)
				r.With(requirePublisherAdmin).Delete("/{id}", grantH.DeletePublisherGrant)
			})
		})

		// MCP servers
		r.Route("/mcp/servers", func(r chi.Router) {
			r.With(publicRL).Get("/", mcpH.ListServers)
			// Create is publisher-scoped: a publisher Editor may
			// author. The role check is in-handler because the target publisher
			// is in the request body, not the path; CreateServer 401s anonymous
			// callers and 403s non-Editors.
			r.Post("/", mcpH.CreateServer)

			r.Route("/{namespace}/{slug}", func(r chi.Router) {
				r.With(publicRL).Get("/", mcpH.GetServer)
				// Edits require Editor on the owning publisher.
				// Admin override is built in.
				r.With(requireMCPServerNS).Patch("/", mcpH.PatchServer)
				// Legacy direct delete is admin-only: it bypasses the
				// change-approval workflow (deletion-request /
				// deletion-request/approve). Publisher Editors must use
				// the workflow path; admins keep this as a
				// force-delete escape hatch.
				r.With(auth.RequireAdmin).Delete("/", mcpH.DeleteServer)
				r.With(requireMCPServerNS).Post("/deprecate", mcpH.DeprecateServer)
				// Visibility is an Editor/Admin action, but SetVisibility only
				// allows flipping to public once the entry has an approved
				// (published) version — an unreviewed draft cannot be exposed.
				r.With(requireMCPServerNS).Post("/visibility", mcpH.SetVisibility)
				r.With(publicRL).Post("/view", mcpH.RecordView)
				r.With(publicRL).Post("/copy", mcpH.RecordCopy)
				r.With(publicRL).Get("/activity", mcpH.ListMCPServerActivity)

				r.Route("/versions", func(r chi.Router) {
					r.With(publicRL).Get("/", mcpH.ListVersions)
					r.With(requireMCPServerNS).Post("/", mcpH.CreateVersion)
					r.With(publicRL).Get("/{version}", mcpH.GetVersion)
					// Publishing a version takes it live, so it is an approver
					// action: Reviewer (or break-glass Server Admin) only.
					// Editors/Admins reach "published" by submitting for review.
					r.With(requireReviewerNS).Post("/{version}/publish", mcpH.PublishVersion)
					// Change-approval workflow.
					r.With(requireMCPServerNS).Post("/{version}/submit", revH.SubmitMCPVersion)
					r.With(requireMCPServerNS).Post("/{version}/withdraw", revH.WithdrawMCPVersion)
					r.With(requireReviewerNS).Post("/{version}/approve", revH.ApproveMCPVersion)
					r.With(requireReviewerNS).Post("/{version}/reject", revH.RejectMCPVersion)
				})
				// Pending-deletion review flow.
				r.With(requireMCPServerNS).Post("/deletion-request", revH.RequestMCPDeletion)
				r.With(requireReviewerNS).Post("/deletion-request/approve", revH.ApproveMCPDeletion)
				r.With(requireReviewerNS).Post("/deletion-request/reject", revH.RejectMCPDeletion)
			})
		})

		// Agents
		r.Route("/agents", func(r chi.Router) {
			r.With(publicRL).Get("/", agentH.ListAgents)
			// Create is publisher-scoped; see the MCP create note.
			r.Post("/", agentH.CreateAgent)

			r.Route("/{namespace}/{slug}", func(r chi.Router) {
				r.With(publicRL).Get("/", agentH.GetAgent)
				r.With(requireAgentNS).Patch("/", agentH.PatchAgent)
				// Same reasoning as MCP servers: bypass-the-workflow path
				// stays admin-only.
				r.With(auth.RequireAdmin).Delete("/", agentH.DeleteAgent)
				r.With(requireAgentNS).Post("/deprecate", agentH.DeprecateAgent)
				// Editor/Admin; public requires an approved version (see MCP note).
				r.With(requireAgentNS).Post("/visibility", agentH.SetVisibility)
				r.With(publicRL).Post("/view", agentH.RecordView)
				r.With(publicRL).Post("/copy", agentH.RecordCopy)
				r.With(publicRL).Get("/activity", agentH.ListAgentActivity)

				r.Route("/versions", func(r chi.Router) {
					r.With(publicRL).Get("/", agentH.ListVersions)
					r.With(requireAgentNS).Post("/", agentH.CreateVersion)
					r.With(publicRL).Get("/{version}", agentH.GetVersion)
					// Approver action (see the MCP publish note): Reviewer / Server Admin.
					r.With(requireReviewerNS).Post("/{version}/publish", agentH.PublishVersion)
					r.With(requireAgentNS).Patch("/{version}/status", agentH.PatchVersionStatus)
					// Change-approval workflow.
					r.With(requireAgentNS).Post("/{version}/submit", revH.SubmitAgentVersion)
					r.With(requireAgentNS).Post("/{version}/withdraw", revH.WithdrawAgentVersion)
					r.With(requireReviewerNS).Post("/{version}/approve", revH.ApproveAgentVersion)
					r.With(requireReviewerNS).Post("/{version}/reject", revH.RejectAgentVersion)
				})
				// Pending-deletion review flow.
				r.With(requireAgentNS).Post("/deletion-request", revH.RequestAgentDeletion)
				r.With(requireReviewerNS).Post("/deletion-request/approve", revH.ApproveAgentDeletion)
				r.With(requireReviewerNS).Post("/deletion-request/reject", revH.RejectAgentDeletion)
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
