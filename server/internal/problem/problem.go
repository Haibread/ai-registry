// Package problem provides RFC 7807 "Problem Details for HTTP APIs" helpers.
// All API error responses use this single shared implementation so the wire
// format is consistent across the auth middleware and every HTTP handler.
package problem

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
)

// Detail is the RFC 7807 response body.
type Detail struct {
	Type     string       `json:"type"`
	Title    string       `json:"title"`
	Status   int          `json:"status"`
	Detail   string       `json:"detail,omitempty"`
	Instance string       `json:"instance,omitempty"`
	Errors   []FieldError `json:"errors,omitempty"`
}

// FieldError describes a single field-level validation failure,
// used in 422 Unprocessable Entity responses.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Write encodes a Problem Detail response and sets Content-Type to
// application/problem+json.  slug becomes the type URL path segment, e.g.
// "not-found" → "https://registry/errors/not-found".
func Write(w http.ResponseWriter, status int, slug, detail, instance string) {
	p := Detail{
		Type:     fmt.Sprintf("https://registry/errors/%s", slug),
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.Error("problem: failed to encode response", slog.String("err", err.Error()))
	}
}

// WriteWithErrors is like Write but also includes field-level validation errors.
func WriteWithErrors(w http.ResponseWriter, status int, slug, detail, instance string, errs []FieldError) {
	p := Detail{
		Type:     fmt.Sprintf("https://registry/errors/%s", slug),
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Errors:   errs,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		slog.Error("problem: failed to encode response", slog.String("err", err.Error()))
	}
}
