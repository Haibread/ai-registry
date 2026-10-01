package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInternalErrorDetail_LogsCauseAndHidesIt(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
	internalErrorDetail(rec, req, errors.New("pq: relation does not exist"), "failed to fetch stats")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "relation") {
		t.Errorf("response leaks the cause: %s", rec.Body.String())
	}
	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log line is not JSON: %v: %q", err, buf.String())
	}
	if line["level"] != "ERROR" || line["error"] != "pq: relation does not exist" || line["path"] != "/api/v1/stats" {
		t.Errorf("log line = %v", line)
	}
}
