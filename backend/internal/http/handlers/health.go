// Package handlers contains HTTP handler functions.
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
)

// Pinger is the minimal interface required by the readiness handler to check
// database connectivity. *pgxpool.Pool satisfies this interface.
type Pinger interface {
	Ping(ctx context.Context) error
}

// Healthz handles GET /healthz (liveness probe).
// It always returns 200 OK; if the process is alive, the service is live.
func Healthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz returns a handler for GET /readyz (readiness probe).
// It returns 200 when the database is reachable, 503 otherwise.
func Readyz(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "unavailable",
				"error":  err.Error(),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
