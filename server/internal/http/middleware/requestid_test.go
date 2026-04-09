package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/http/middleware"
)

func TestRequestID_GeneratesID(t *testing.T) {
	var capturedID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.FromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	middleware.RequestID(next).ServeHTTP(rec, req)

	if capturedID == "" {
		t.Error("expected a request ID to be generated, got empty string")
	}
	if rec.Header().Get("X-Request-ID") != capturedID {
		t.Errorf("response header X-Request-ID = %q, want %q", rec.Header().Get("X-Request-ID"), capturedID)
	}
}

func TestRequestID_PropagatesExistingID(t *testing.T) {
	const existingID = "test-request-id-123"
	var capturedID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = middleware.FromContext(r.Context())
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", existingID)
	rec := httptest.NewRecorder()

	middleware.RequestID(next).ServeHTTP(rec, req)

	if capturedID != existingID {
		t.Errorf("capturedID = %q, want %q", capturedID, existingID)
	}
	if rec.Header().Get("X-Request-ID") != existingID {
		t.Errorf("response header X-Request-ID = %q, want %q", rec.Header().Get("X-Request-ID"), existingID)
	}
}
