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
	trustedProxy *net.IPNet
}

// RateLimit returns middleware that limits each unique IP to max requests per window.
// When the limit is exceeded it writes 429 Too Many Requests with a Retry-After header.
// Cleanup of stale entries happens lazily on every 100th request (amortised O(1)).
// If metrics is non-nil, each rejection increments registry.ratelimit.hits.
// trustedProxy is passed to ClientIP to derive the per-client key.
func RateLimit(max int, window time.Duration, metrics *observability.Metrics, trustedProxy *net.IPNet) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		buckets:      make(map[string]*bucket),
		max:          max,
		window:       window,
		trustedProxy: trustedProxy,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ClientIP(r, rl.trustedProxy)
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

// ClientIP returns the client IP. X-Forwarded-For is only consulted when the
// direct connection (RemoteAddr) falls within trustedProxy; it is then walked
// from the right, skipping hops inside trustedProxy, and the first address
// outside it is the client. Entries left of that point are client-supplied
// and never used. When trustedProxy is nil, RemoteAddr is always used.
// Exposed so handlers that log IPs share the rate limiter's trust policy.
func ClientIP(r *http.Request, trustedProxy *net.IPNet) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}

	if trustedProxy == nil || !trustedProxy.Contains(net.ParseIP(host)) {
		return host
	}

	var hops []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(v, ",")...)
	}

	client := host
	for i := len(hops) - 1; i >= 0; i-- {
		ip := parseForwardedIP(hops[i])
		if ip == nil {
			// Trusted proxies only append valid addresses, so anything left of
			// garbage is client-supplied: stop at the last trusted hop.
			return client
		}
		if !trustedProxy.Contains(ip) {
			return ip.String()
		}
		client = ip.String()
	}
	// Every hop is trusted: the leftmost one is the client, as with nginx's
	// real_ip_recursive.
	return client
}

func parseForwardedIP(s string) net.IP {
	s = strings.TrimSpace(s)
	if h, _, err := net.SplitHostPort(s); err == nil {
		s = h
	}
	return net.ParseIP(strings.TrimSuffix(strings.TrimPrefix(s, "["), "]"))
}
