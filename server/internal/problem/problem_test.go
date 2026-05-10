package problem_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/problem"
)

// All API errors flow through this package per CLAUDE.md ("Errors: API errors
// follow RFC 7807 (`application/problem+json`)"). The package was at 0%
// coverage in the post-Phase 7 audit; these tests pin the wire format so a
// regression in the shared error shape — the kind that quietly breaks every
// admin-UI error toast — is caught before it lands.

func TestWrite_HappyPath(t *testing.T) {
	rec := httptest.NewRecorder()
	problem.Write(rec, http.StatusNotFound, "not-found", "agent does not exist", "/api/v1/agents/x/y")

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}

	var got problem.Detail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Type != "https://registry/errors/not-found" {
		t.Errorf("Type = %q", got.Type)
	}
	if got.Title != "Not Found" {
		t.Errorf("Title = %q, want Not Found", got.Title)
	}
	if got.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", got.Status)
	}
	if got.Detail != "agent does not exist" {
		t.Errorf("Detail = %q", got.Detail)
	}
	if got.Instance != "/api/v1/agents/x/y" {
		t.Errorf("Instance = %q", got.Instance)
	}
	if got.Errors != nil {
		t.Errorf("Errors = %v, want nil for non-validation problem", got.Errors)
	}
}

func TestWrite_OmitsEmptyOptionals(t *testing.T) {
	// Both `detail` and `instance` are tagged omitempty. Empty values must
	// not appear in the wire body — clients that decode strictly will reject
	// surprise fields, and admin-UI toasts get noisier with empty strings.
	rec := httptest.NewRecorder()
	problem.Write(rec, http.StatusBadRequest, "bad-request", "", "")

	body := rec.Body.String()
	for _, key := range []string{`"detail"`, `"instance"`, `"errors"`} {
		if contains(body, key) {
			t.Errorf("body unexpectedly contains %s; got %s", key, body)
		}
	}
}

func TestWrite_SlugIsURLSegment(t *testing.T) {
	// The slug becomes the last path segment of the `type` URL — the rest
	// of the system uses substring matches like
	// `strings.Contains(typeField, "misconfiguration")` to identify the error
	// class. Pin that contract.
	rec := httptest.NewRecorder()
	problem.Write(rec, http.StatusInternalServerError, "misconfiguration", "x", "")

	var got problem.Detail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Type != "https://registry/errors/misconfiguration" {
		t.Errorf("Type = %q, want it to end in /misconfiguration", got.Type)
	}
}

func TestWriteWithErrors_IncludesFieldErrors(t *testing.T) {
	rec := httptest.NewRecorder()
	errs := []problem.FieldError{
		{Field: "name", Message: "must not be empty"},
		{Field: "version", Message: "must be valid semver"},
	}
	problem.WriteWithErrors(rec, http.StatusUnprocessableEntity,
		"validation-failed", "request body has invalid fields", "/api/v1/x", errs)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}

	var got problem.Detail
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Errors) != 2 {
		t.Fatalf("Errors len = %d, want 2", len(got.Errors))
	}
	if got.Errors[0] != errs[0] || got.Errors[1] != errs[1] {
		t.Errorf("Errors = %+v, want %+v", got.Errors, errs)
	}
}

func TestWriteWithErrors_OmitsEmptyErrorList(t *testing.T) {
	// `errors` is tagged omitempty. Sending an empty slice should not emit
	// a `"errors":[]` field — it would confuse RFC 7807 consumers that
	// distinguish "no validation errors" from "no validation errors object
	// at all".
	rec := httptest.NewRecorder()
	problem.WriteWithErrors(rec, http.StatusBadRequest, "bad-request",
		"detail", "/x", nil)

	if contains(rec.Body.String(), `"errors"`) {
		t.Errorf("body unexpectedly contains \"errors\" key; got %s", rec.Body.String())
	}
}

// contains is a tiny helper so the assertions read closer to English.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
