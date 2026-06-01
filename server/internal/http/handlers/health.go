// Package handlers contains HTTP handler functions.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/haibread/ai-registry/internal/auth"
	"github.com/haibread/ai-registry/internal/domain"
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

// decodeJSON deserialises the request body into v. Decoding is strict: unknown
// fields are rejected (so a typo'd field name 422s instead of being silently
// dropped) and the body must contain exactly one JSON value with no trailing
// data. On failure it writes the appropriate problem response and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxErr):
			problem.Write(w, http.StatusRequestEntityTooLarge, "request-too-large",
				"request body exceeds the 1 MiB limit", r.URL.Path)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
				"request body contains an "+strings.TrimPrefix(err.Error(), "json: "), r.URL.Path)
		default:
			problem.Write(w, http.StatusUnprocessableEntity, "validation-error", "invalid JSON body", r.URL.Path)
		}
		return false
	}
	// A request body must be a single JSON object; reject any trailing data
	// (a second value, garbage after the object) rather than silently ignoring it.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"request body must contain a single JSON object", r.URL.Path)
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

// privateScopeStore is the store slice canViewPrivate needs: resolve a
// publisher id from its slug, and resolve the caller's effective roles on it.
// *store.DB satisfies it.
type privateScopeStore interface {
	GetPublisherBySlug(ctx context.Context, slug string) (string, error)
	auth.RoleStore
}

// canViewPrivate reports whether the caller may see private / draft resources
// owned by the publisher whose slug is namespace. It is true for a Server Admin
// or any caller holding a role (Viewer and up) on that publisher, and false for
// an anonymous caller or a member of a *different* publisher — those get
// public-only reads. It widens visibility only for the owning
// publisher's own members; it never exposes one publisher's private data to
// another's. A namespace that does not resolve to a publisher yields false, so
// the read falls back to public-only and the handler's own 404 covers it.
//
// Read handlers use it to set publicOnly: a publisher member must be able to
// open the detail / versions / activity of their own not-yet-public entries,
// not just see them in the mine-scoped list.
func canViewPrivate(ctx context.Context, st privateScopeStore, namespace string) bool {
	if auth.IsServerAdminFromContext(ctx) {
		return true
	}
	// Skip the DB round-trips for anonymous reads (the common public path).
	if !auth.IsAuthenticated(ctx) {
		return false
	}
	pubID, err := st.GetPublisherBySlug(ctx, namespace)
	if err != nil {
		return false
	}
	ok, _, err := auth.CheckPublisherRole(ctx, st, pubID, domain.RoleViewer)
	if err != nil {
		return false
	}
	return ok
}
