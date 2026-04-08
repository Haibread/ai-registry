package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type bucket struct {
	count     int
	windowStart time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	max     int
	window  time.Duration
	reqCount int
}

// RateLimit returns middleware that limits each unique IP to max requests per window.
// When the limit is exceeded it writes 429 Too Many Requests with a Retry-After header.
// Cleanup of stale entries happens lazily on every 100th request (amortized O(1)).
func RateLimit(max int, window time.Duration) func(http.Handler) http.Handler {
	rl := &rateLimiter{
		buckets: make(map[string]*bucket),
		max:     max,
		window:  window,
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)
			now := time.Now()

			rl.mu.Lock()
			rl.reqCount++
			// Periodic cleanup: every 100th request, remove stale buckets
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
				b = &bucket{windowStart: now}
				rl.buckets[ip] = b
			}

			// Reset if window has elapsed
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

// clientIP extracts the client IP from X-Forwarded-For (first entry) or RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take first entry
		if idx := strings.Index(xff, ","); idx >= 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	// Strip port from RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
