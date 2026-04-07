// Package http wires together the chi router, middleware, and handlers.
package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/haibread/ai-registry/internal/http/handlers"
	"github.com/haibread/ai-registry/internal/http/middleware"
	"github.com/haibread/ai-registry/internal/observability"
	"github.com/haibread/ai-registry/internal/store"
)

// RouterDeps bundles the dependencies injected into the router.
type RouterDeps struct {
	Logger  *slog.Logger
	DB      *store.DB
	Metrics *observability.Metrics
}

// NewRouter builds and returns the chi router with all middleware and routes
// registered for Phase 1.
func NewRouter(deps RouterDeps) http.Handler {
	r := chi.NewRouter()

	// ── Core middleware ───────────────────────────────────────────────────────
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(deps.Logger))

	// ── System endpoints (no auth) ────────────────────────────────────────────
	r.Get("/healthz", handlers.Healthz)
	r.Get("/readyz", handlers.Readyz(deps.DB.Pool))
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Get("/openapi.yaml", handlers.OpenAPISpec)

	// ── Wrap with OTel HTTP instrumentation ──────────────────────────────────
	// The span name formatter uses chi's route pattern for accurate attribution.
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
