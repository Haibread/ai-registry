package handlers_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/haibread/ai-registry/internal/http/handlers"
)

// mockPinger implements handlers.Pinger for testing.
type mockPinger struct {
	err error
}

func (m *mockPinger) Ping(_ context.Context) error { return m.err }

func TestHealthz(t *testing.T) {
	tests := []struct {
		name           string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "always returns 200",
			expectedStatus: http.StatusOK,
			expectedBody:   `"status":"ok"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
			rec := httptest.NewRecorder()

			handlers.Healthz(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.expectedBody) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), tt.expectedBody)
			}
		})
	}
}

func TestReadyz(t *testing.T) {
	tests := []struct {
		name           string
		pingErr        error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "healthy database",
			pingErr:        nil,
			expectedStatus: http.StatusOK,
			expectedBody:   `"status":"ok"`,
		},
		{
			name:           "database unreachable",
			pingErr:        errors.New("connection refused"),
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   `"status":"unavailable"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
			rec := httptest.NewRecorder()

			h := handlers.Readyz(&mockPinger{err: tt.pingErr})
			h(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.expectedStatus)
			}
			if !strings.Contains(rec.Body.String(), tt.expectedBody) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), tt.expectedBody)
			}
		})
	}
}
