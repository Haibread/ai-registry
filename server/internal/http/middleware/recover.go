package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/haibread/ai-registry/internal/problem"
)

// Recover is chi middleware that turns a panic in any downstream handler into an
// RFC 7807 application/problem+json 500, keeping the wire format consistent with
// every other error in the system. (chi's built-in Recoverer writes a bare
// text/plain 500, which breaks the problem+json contract clients rely on.)
//
// The panic value and stack are logged at error level. http.ErrAbortHandler is
// re-panicked unchanged so the server's request-abort semantics are preserved.
// A nil logger falls back to slog.Default so route-walk tests that construct the
// router without a logger stay safe.
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				logger.ErrorContext(r.Context(), "panic recovered in HTTP handler",
					slog.Any("panic", rec),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.String("stack", string(debug.Stack())),
				)
				problem.Write(w, http.StatusInternalServerError, "internal",
					"an internal error occurred", r.URL.Path)
			}()
			next.ServeHTTP(w, r)
		})
	}
}
