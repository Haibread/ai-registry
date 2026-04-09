// Package handlers contains HTTP handler functions.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/problem"
)

// Pinger is the minimal interface required by the readiness handler to check
// database connectivity. *pgxpool.Pool satisfies this interface.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Healthz handles GET /healthz (liveness probe).
// It always returns 200 OK; if the process is alive, the service is live.
func Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz returns a handler for GET /readyz (readiness probe).
// It returns 200 when the database is reachable, 503 otherwise.
func Readyz(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			slog.ErrorContext(r.Context(), "readyz: database ping failed", slog.String("err", err.Error()))
			writeJSON(w, r, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
			return
		}
		writeJSON(w, r, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.ErrorContext(r.Context(), "writeJSON: failed to encode response",
			slog.String("err", err.Error()))
	}
}

// internalError logs err and writes a generic 500 problem response.
// The raw error is never forwarded to the client to avoid leaking internals.
func internalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.ErrorContext(r.Context(), "internal error", slog.String("err", err.Error()))
	problem.Write(w, http.StatusInternalServerError, "internal", "an internal error occurred", r.URL.Path)
}

// decodeJSON deserialises the request body into v.
// On failure it writes the appropriate problem response and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			problem.Write(w, http.StatusRequestEntityTooLarge, "request-too-large",
				"request body exceeds the 1 MiB limit", r.URL.Path)
			return false
		}
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		return false
	}
	return true
}

// auditActor extracts the subject and email for audit log entries.
// Behind RequireAdmin the claims should always be present; the fallback guards
// against misconfigured routes where the middleware chain is incomplete.
func auditActor(ctx context.Context) (subject, email string) {
	if c, ok := auth.ClaimsFromContext(ctx); ok {
		return c.Subject, c.Email
	}
	slog.WarnContext(ctx, "BUG: admin endpoint reached without auth claims in context")
	return "unknown", ""
}
