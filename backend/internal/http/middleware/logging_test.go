package middleware_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/haibread/ai-registry/internal/http/middleware"
)

func TestRequestLogger_LogsRequest(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := middleware.RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/test-path", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logged := buf.String()
	if logged == "" {
		t.Fatal("expected log output, got none")
	}
	if !strings.Contains(logged, "http request") {
		t.Errorf("log line missing 'http request': %s", logged)
	}
	if !strings.Contains(logged, "/test-path") {
		t.Errorf("log line missing path: %s", logged)
	}
	if !strings.Contains(logged, "GET") {
		t.Errorf("log line missing method: %s", logged)
	}
}

func TestRequestLogger_LogsStatusCode(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := middleware.RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logged := buf.String()
	if !strings.Contains(logged, "404") {
		t.Errorf("log line missing status 404: %s", logged)
	}
}

func TestRequestLogger_ImplicitStatus200(t *testing.T) {
	// When the handler writes body without calling WriteHeader, status should be 200.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	handler := middleware.RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No explicit WriteHeader — body write implies 200.
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/implicit", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logged := buf.String()
	if !strings.Contains(logged, "200") {
		t.Errorf("expected status 200 in log: %s", logged)
	}
}

func TestRequestLogger_LogsByteCount(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	responseBody := "hello world"
	handler := middleware.RequestLogger(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(responseBody))
	}))

	req := httptest.NewRequest(http.MethodGet, "/bytes", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logged := buf.String()
	// The byte count for "hello world" = 11
	if !strings.Contains(logged, "11") {
		t.Errorf("log line missing byte count 11: %s", logged)
	}
}

func TestRequestLogger_WithRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Chain RequestID + RequestLogger so the request_id field is populated.
	handler := middleware.RequestID(
		middleware.RequestLogger(logger)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/with-id", nil)
	req.Header.Set("X-Request-ID", "test-id-abc")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	logged := buf.String()
	if !strings.Contains(logged, "test-id-abc") {
		t.Errorf("log line missing request_id: %s", logged)
	}
}
