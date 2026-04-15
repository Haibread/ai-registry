package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/auth"
	stdhttp "github.com/haibread/ai-registry/internal/http"
)

// TestPublicRateLimitRPM_WiredToMiddleware proves that the RouterDeps field
// PublicRateLimitRPM actually reaches the per-IP bucket. It is easy to break
// this wiring by mistake (e.g. by passing the wrong variable to RateLimit),
// and the bug is invisible at unit-test granularity because the middleware
// has its own solid coverage.
//
// Strategy: build a router with PublicRateLimitRPM=2, fire three sequential
// requests against a public GET on /api/v1, and assert the third one gets
// 429. The handler itself may return 500 because we pass a nil DB — we
// don't care: the rate limiter runs before the handler, and the 429 is
// what proves the limit was read from RouterDeps.
//
// Runs synchronously in-process, no containers, no external deps.
func TestPublicRateLimitRPM_WiredToMiddleware(t *testing.T) {
	const limit = 2

	mux := stdhttp.NewRouterForTest(stdhttp.RouterDeps{
		AuthConf:           auth.Config{OIDCIssuer: "https://example.invalid"},
		PublicRateLimitRPM: limit,
	})

	// /api/v1/changelog is a rate-limited public GET (router.go: publicRL
	// is applied via .With(publicRL).Get("/changelog", ...)). The handler
	// will 500 because deps.DB is nil, but that happens AFTER the rate
	// limiter decides whether to let the request through — so the first
	// two requests get whatever the handler produces, and the third gets
	// 429 from the middleware.
	fire := func() int {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/changelog", nil)
		req.RemoteAddr = "192.0.2.1:1234" // fixed source so all three share a bucket
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	// First `limit` requests: anything except 429 is acceptable. The
	// handler may 500 (nil DB) or 200 (if some edge of the code tolerates
	// nil) — we just need to confirm they are NOT rate-limit rejections.
	for i := 1; i <= limit; i++ {
		if code := fire(); code == http.StatusTooManyRequests {
			t.Fatalf("request %d returned 429 before the bucket was exhausted (limit=%d)", i, limit)
		}
	}

	// Request limit+1: must be 429 — proves the middleware's per-IP bucket
	// was constructed with max=PublicRateLimitRPM.
	if code := fire(); code != http.StatusTooManyRequests {
		t.Fatalf("request %d after the limit returned %d, want 429", limit+1, code)
	}
}

// TestPublicRateLimitRPM_ZeroDefaultsTo1000 guards the documented fallback:
// RouterDeps.PublicRateLimitRPM == 0 must map to a per-IP budget of 1000 rpm,
// not to zero (which would reject every request). We only verify the first
// request succeeds — proving the fallback kicked in — rather than firing
// 1001 requests to observe the actual cutoff (that would be slow and
// redundant with the explicit test above).
func TestPublicRateLimitRPM_ZeroDefaultsTo1000(t *testing.T) {
	mux := stdhttp.NewRouterForTest(stdhttp.RouterDeps{
		AuthConf:           auth.Config{OIDCIssuer: "https://example.invalid"},
		PublicRateLimitRPM: 0, // must NOT be interpreted as "allow zero"
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/changelog", nil)
	req.RemoteAddr = "192.0.2.2:1234"
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("the very first request was rate-limited — PublicRateLimitRPM=0 fallback is broken")
	}
}
