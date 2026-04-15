package http_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/haibread/ai-registry/internal/auth"
	stdhttp "github.com/haibread/ai-registry/internal/http"
)

// TestHTTPHandlers_EmitOTelSpans codifies the CLAUDE.md rule
//
//	"every new handler gets a span"
//
// into an enforced contract. It installs a tracetest SpanRecorder as the
// global tracer provider, fires one request at each DB-free public route,
// and asserts that every request produces at least one span carrying the
// stable HTTP semantic-convention attributes (http.request.method and
// http.response.status_code).
//
// The test uses stdhttp.NewRouter (not NewRouterForTest) because the otelhttp
// middleware is precisely what's under test — NewRouterForTest returns the
// raw *chi.Mux with no instrumentation, which would defeat the purpose.
//
// Routes under test are chosen to be DB-free so the test stays hermetic:
//
//   - GET /healthz                            — static 200
//   - GET /config.json                         — reads OIDC bootstrap values
//   - GET /.well-known/oauth-protected-resource — static JSON
//   - GET /openapi.yaml                        — embedded spec bytes
//
// If this test starts failing, the most likely cause is somebody replaced
// otelhttp.NewHandler with a bare mux in NewRouter — the exact bug CLAUDE.md
// warns about. The second-most-likely cause is an otelhttp/semconv upgrade
// that renames the attribute keys; update the allow-list of attribute names
// in that case, don't silence the assertion.
func TestHTTPHandlers_EmitOTelSpans(t *testing.T) {
	// ── Install an in-memory tracer provider globally ───────────────────────
	// otelhttp resolves its tracer from otel.GetTracerProvider() on every
	// request, so setting the global provider is enough — no need to inject
	// anything through RouterDeps. Restore the prior provider on cleanup so
	// this test does not leak into sibling tests that run in the same binary.
	prevTP := otel.GetTracerProvider()
	t.Cleanup(func() { otel.SetTracerProvider(prevTP) })

	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	otel.SetTracerProvider(tp)

	// ── Build the full production router (otelhttp-wrapped) ─────────────────
	// discardLogger is defined in router_ratelimit_test.go — both tests share
	// the same need for a non-nil logger to avoid RequestLogger panics.
	handler := stdhttp.NewRouter(stdhttp.RouterDeps{
		Logger:   discardLogger(),
		AuthConf: auth.Config{OIDCIssuer: "https://example.invalid"},
	})

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/config.json"},
		{http.MethodGet, "/.well-known/oauth-protected-resource"},
		{http.MethodGet, "/openapi.yaml"},
	}

	for _, r := range routes {
		req := httptest.NewRequest(r.method, r.path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		// Non-2xx is fine here — we only care that the middleware ran and
		// recorded a span, not that the handler succeeded.
	}

	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force-flushing tracer provider: %v", err)
	}

	ended := sr.Ended()
	if len(ended) < len(routes) {
		t.Fatalf("got %d ended spans, want at least %d (one per request)", len(ended), len(routes))
	}

	// Accept both stable and legacy HTTP semconv names — otelhttp has been
	// migrating for several releases and the exact key depends on the
	// instrumentation version pinned in go.mod.
	methodKeys := []attribute.Key{"http.request.method", "http.method"}
	statusKeys := []attribute.Key{"http.response.status_code", "http.status_code"}

	for _, route := range routes {
		var match sdktrace.ReadOnlySpan
		for _, span := range ended {
			if spanHasAttrValue(span, methodKeys, route.method) {
				match = span
				break
			}
		}
		if match == nil {
			t.Errorf("no span carries %v=%q for request %s %s",
				methodKeys, route.method, route.method, route.path)
			continue
		}
		if !spanHasAnyKey(match, statusKeys) {
			t.Errorf("span for %s %s is missing a status-code attribute (want one of %v)",
				route.method, route.path, statusKeys)
		}
	}
}

// spanHasAttrValue reports whether span has an attribute whose key is one of
// keys and whose string value equals want.
func spanHasAttrValue(span sdktrace.ReadOnlySpan, keys []attribute.Key, want string) bool {
	for _, kv := range span.Attributes() {
		for _, k := range keys {
			if kv.Key == k && kv.Value.AsString() == want {
				return true
			}
		}
	}
	return false
}

// spanHasAnyKey reports whether span carries any attribute matching one of keys.
func spanHasAnyKey(span sdktrace.ReadOnlySpan, keys []attribute.Key) bool {
	for _, kv := range span.Attributes() {
		for _, k := range keys {
			if kv.Key == k {
				return true
			}
		}
	}
	return false
}
