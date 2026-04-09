package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/haibread/ai-registry/internal/observability"
)

// maxBuckets is the upper bound on tracked IPs. When the map is full, new
// source IPs are rejected with 429 rather than growing the map unboundedly.
// A periodic cleanup (every 100 requests) keeps the count well below this
// ceiling under normal traffic.
const maxBuckets = 100_000

type bucket struct {
	count       int
	windowStart time.Time
}

type rateLimiter struct {
	mu           sync.Mutex
	buckets      map[string]*bucket
	max          int
	window       time.Duration
	reqCount     int
	trustedProxy *net.IPNet // when non-nil, X-Forwarded-For is trusted from this CIDR
}

// RateLimit returns middleware that limits each unique IP to max requests per window.
// When the limit is exceeded it writes 429 Too Many Requests with a Retry-After header.
// Cleanup of stale entries happens lazily on every 100th request (amortised O(1)).
// If metrics is non-nil, each rejection increments registry.ratelimit.hits.
// trustedProxy, when non-nil, is the CIDR of a reverse proxy whose
// X-Forwarded-For header is trusted. When nil, RemoteAddr is always used.
func RateLimit(max int, window time.Duration, metrics *observability.Metrics, trustedProxy *net.IPNet) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		buckets:      make(map[string]*bucket),
		max:          max,
		window:       window,
		trustedProxy: trustedProxy,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r, rl.trustedProxy)
			now := time.Now()

			rl.mu.Lock()
			rl.reqCount++
			// Periodic cleanup: every 100th request, remove stale buckets.
			if rl.reqCount%100 == 0 {
				cutoff := now.Add(-2 * window)
				for k, b := range rl.buckets {
					if b.windowStart.Before(cutoff) {
						delete(rl.buckets, k)
					}
				}
			}

			b, ok := rl.buckets[ip]
			if !ok {
				// Reject new entrants when the map is at capacity to prevent OOM.
				if len(rl.buckets) >= maxBuckets {
					rl.mu.Unlock()
					if metrics != nil {
						metrics.RateLimitHits.Add(r.Context(), 1)
					}
					w.Header().Set("Retry-After", fmt.Sprintf("%d", int(window.Seconds())))
					http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
					return
				}
				b = &bucket{windowStart: now}
				rl.buckets[ip] = b
			}

			// Reset if window has elapsed.
			if now.Sub(b.windowStart) >= window {
				b.count = 0
				b.windowStart = now
			}

			if b.count >= max {
				retryAfter := int(window.Seconds() - now.Sub(b.windowStart).Seconds())
				if retryAfter < 1 {
					retryAfter = 1
				}
				rl.mu.Unlock()
				if metrics != nil {
					metrics.RateLimitHits.Add(r.Context(), 1)
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}

			b.count++
			rl.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// clientIP returns the client IP. X-Forwarded-For is only trusted when the
// direct connection (RemoteAddr) falls within trustedProxy. When trustedProxy
// is nil, RemoteAddr is always used.
func clientIP(r *http.Request, trustedProxy *net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if trustedProxy != nil && trustedProxy.Contains(net.ParseIP(host)) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Take the first (leftmost) entry — the original client IP.
			if idx := strings.Index(xff, ","); idx >= 0 {
				return strings.TrimSpace(xff[:idx])
			}
			return strings.TrimSpace(xff)
		}
	}

	return host
}
