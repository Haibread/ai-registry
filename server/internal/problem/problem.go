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

// New builds the Detail for status. slug becomes the type URL path segment,
// e.g. "not-found" → "https://registry/errors/not-found".
func New(status int, slug, detail, instance string) Detail {
	return Detail{
		Type:     fmt.Sprintf("https://registry/errors/%s", slug),
		Title:    http.StatusText(status),
		Status:   status,
		Detail:   detail,
		Instance: instance,
	}
}

// Write encodes a Problem Detail response and sets Content-Type to
// application/problem+json.
func Write(w http.ResponseWriter, status int, slug, detail, instance string) {
	WriteBody(w, status, New(status, slug, detail, instance))
}

// WriteWithErrors is like Write but also includes field-level validation errors.
func WriteWithErrors(w http.ResponseWriter, status int, slug, detail, instance string, errs []FieldError) {
	p := New(status, slug, detail, instance)
	p.Errors = errs
	WriteBody(w, status, p)
}

// WriteBody encodes body as application/problem+json. It is for problems that
// carry RFC 7807 extension members: body is a struct embedding Detail.
func WriteBody(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("problem: failed to encode response", slog.String("error", err.Error()))
	}
}
