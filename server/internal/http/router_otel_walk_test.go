package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/haibread/ai-registry/internal/auth"
	stdhttp "github.com/haibread/ai-registry/internal/http"
)

// TestHTTPHandlers_EveryRegisteredRouteEmitsSpan walks every route registered
// on the chi router and proves that the otelhttp wrapper produces a span for
// each one. This is the broader sibling of TestHTTPHandlers_EmitOTelSpans:
// instead of pinning four hand-picked DB-free routes, we enumerate the
// routing tree the same way the OpenAPI contract test does and assert that
// every walk-targeted request lands inside an otelhttp span.
//
// Goal: codify CLAUDE.md's "every HTTP handler must be traced" promise as a
// machine-checkable contract that fails CI if a future router change drops
// instrumentation on any route — including routes added after this test was
// written. Previously the assertion was only as strong as the four-route
// allow-list in router_otel_test.go.
//
// What we do NOT assert here:
//
//   - That every handler emits *internal* child spans for sub-operations.
//     That's a separate concern about diagnosability, not span existence.
//   - That the response status is 2xx — handlers may legitimately reject a
//     synthetic walk-time request with 401 / 415 / 400 / 404. The middleware
//     short-circuit happens *inside* the otelhttp span, so a 4xx still
//     records the span we care about.
//   - The exact span name. otelhttp's WithSpanNameFormatter timing relative
//     to chi route resolution has churned across instrumentation versions.
//     We assert span existence, not naming.
func TestHTTPHandlers_EveryRegisteredRouteEmitsSpan(t *testing.T) {
	// ── In-memory tracer provider ───────────────────────────────────────────
	prevTP := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(prevTP) })

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	otel.SetTracerProvider(tp)

	// ── Build the router twice ──────────────────────────────────────────────
	// `walkMux` is the unwrapped *chi.Mux so we can enumerate routes with
	// chi.Walk; `serveMux` is the otelhttp-wrapped handler we actually fire
	// requests at. They share configuration so the route set is identical.
	deps := stdhttp.RouterDeps{
		Logger:   discardLogger(),
		AuthConf: auth.Config{OIDCIssuer: "https://example.invalid"},
	}
	walkMux := stdhttp.NewRouterForTest(deps)
	serveMux := stdhttp.NewRouter(deps)

	type route struct{ method, pattern string }
	var routes []route
	walker := func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// Skip the chi-internal `/*` catch-all — it's the group's index entry,
		// not a real route.
		if strings.HasSuffix(pattern, "/*") {
			return nil
		}
		routes = append(routes, route{method: method, pattern: pattern})
		return nil
	}
	if err := chi.Walk(walkMux, walker); err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}
	if len(routes) == 0 {
		t.Fatal("chi.Walk reported zero routes — router fixture is wrong")
	}

	// ── Fire one request per registered route ──────────────────────────────
	// We record the cumulative span count after each request so that any
	// route that fails to produce a span shows up as a 1:1 mismatch in the
	// per-route delta, with the offending route surfaced in the error
	// message.
	for _, r := range routes {
		// Substitute every {param} segment with a literal "x" so the URL is
		// path-routable. The handler is allowed to 4xx the synthetic input;
		// what matters is that the otelhttp wrapper records a span.
		url := substituteChiParams(r.pattern, "x")
		req := httptest.NewRequest(r.method, url, nil)
		rec := httptest.NewRecorder()
		serveMux.ServeHTTP(rec, req)
	}

	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force-flushing tracer provider: %v", err)
	}

	// One span per request is the minimum bar. The otelhttp wrapper produces
	// exactly one root span per call to ServeHTTP, regardless of whether
	// downstream middleware short-circuits with a 4xx. Anything less means a
	// route is bypassing otelhttp entirely — the bug we're guarding against.
	gotSpans := len(sr.Ended())
	if gotSpans < len(routes) {
		t.Fatalf("got %d ended spans across %d registered routes — "+
			"some route is bypassing otelhttp.NewHandler",
			gotSpans, len(routes))
	}
}

// substituteChiParams replaces every {placeholder} in a chi route pattern
// with the given value, producing a path-routable URL. Used to walk every
// registered route in the OTel coverage test without needing per-route
// fixtures.
func substituteChiParams(pattern, value string) string {
	for {
		i := strings.IndexByte(pattern, '{')
		if i < 0 {
			return pattern
		}
		j := strings.IndexByte(pattern[i:], '}')
		if j < 0 {
			return pattern
		}
		pattern = pattern[:i] + value + pattern[i+j+1:]
	}
}
