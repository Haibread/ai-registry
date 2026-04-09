package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/observability"
)

// responseWriter wraps http.ResponseWriter to capture the status code and
// number of bytes written, needed for structured access logging.
type responseWriter struct {
	http.ResponseWriter
	status  int
	written int64
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

// RequestLogger returns a middleware that emits a structured log line for
// every HTTP request and records OTel HTTP metrics (request count, latency,
// auth failures).
func RequestLogger(logger *slog.Logger, metrics *observability.Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w}

			next.ServeHTTP(rw, r)

			status := rw.status
			if status == 0 {
				status = http.StatusOK
			}
			elapsed := time.Since(start)

			// Derive the chi route pattern for the metric label so high-cardinality
			// path params (IDs, slugs) don't explode the metric cardinality.
			route := r.URL.Path
			if rctx := chi.RouteContext(r.Context()); rctx != nil && rctx.RoutePattern() != "" {
				route = rctx.RoutePattern()
			}

			attrs := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.route", route),
				attribute.Int("http.status_code", status),
			}
			attrSet := metric.WithAttributes(attrs...)

			if metrics != nil {
				metrics.HTTPRequestsTotal.Add(r.Context(), 1, attrSet)
				metrics.HTTPRequestDuration.Record(r.Context(), float64(elapsed.Milliseconds()), attrSet)

				// Count auth / authz failures.
				if status == http.StatusUnauthorized || status == http.StatusForbidden {
					metrics.AuthFailures.Add(r.Context(), 1,
						metric.WithAttributes(
							attribute.String("http.method", r.Method),
							attribute.String("http.route", route),
						),
					)
				}
			}

			logger.InfoContext(r.Context(), "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr),
				slog.String("request_id", FromContext(r.Context())),
				slog.Int("status", status),
				slog.Int64("bytes", rw.written),
				slog.Duration("duration", elapsed),
			)
		})
	}
}
