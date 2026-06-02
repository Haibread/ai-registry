package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/http/middleware"
)

func TestRecover_PanicYieldsProblemJSON(t *testing.T) {
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic("boom")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/explode", nil)
	rec := httptest.NewRecorder()

	middleware.Recover(nil)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var body struct {
		Type     string `json:"type"`
		Status   int    `json:"status"`
		Instance string `json:"instance"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if body.Status != http.StatusInternalServerError {
		t.Errorf("body.status = %d, want 500", body.Status)
	}
	if body.Type == "" || body.Instance != "/api/v1/explode" {
		t.Errorf("unexpected problem body: %+v", body)
	}
}

func TestRecover_NoPanicPassesThrough(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ok", nil)
	rec := httptest.NewRecorder()

	middleware.Recover(nil)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
}

func TestRecover_AbortHandlerRepanics(t *testing.T) {
	// http.ErrAbortHandler is the sentinel a handler panics with to abort the
	// request without logging a 500; Recover must re-panic it unchanged so the
	// server keeps its abort semantics.
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		panic(http.ErrAbortHandler)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/abort", nil)
	rec := httptest.NewRecorder()

	defer func() {
		rec := recover()
		if rec != http.ErrAbortHandler {
			t.Fatalf("expected re-panicked http.ErrAbortHandler, got %v", rec)
		}
	}()

	middleware.Recover(nil)(next).ServeHTTP(rec, req)
	t.Fatal("expected Recover to re-panic http.ErrAbortHandler")
}
