package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ─── MCP Server Visibility ───────────────────────────────────────────────────

func TestMCPHandler_SetVisibility_Valid(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "vis-ns", "vis-srv")

	r := newMCPRouter()

	tests := []struct {
		visibility string
	}{
		{"public"},
		{"private"},
	}

	for _, tt := range tests {
		t.Run(tt.visibility, func(t *testing.T) {
			payload := `{"visibility":"` + tt.visibility + `"}`
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/mcp/servers/vis-ns/vis-srv/visibility",
				bytes.NewBufferString(payload))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(adminCtx())
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			var body map[string]string
			json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
			if body["visibility"] != tt.visibility {
				t.Errorf("visibility = %q, want %q", body["visibility"], tt.visibility)
			}
		})
	}
}

func TestMCPHandler_SetVisibility_InvalidValue(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "vis-inv-ns", "vis-inv-srv")

	payload := `{"visibility":"invalid"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/mcp/servers/vis-inv-ns/vis-inv-srv/visibility",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestMCPHandler_SetVisibility_InvalidJSON(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "vis-ijns", "vis-ij-srv")

	payload := `{bad}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/mcp/servers/vis-ijns/vis-ij-srv/visibility",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestMCPHandler_SetVisibility_UnknownServer(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "vis-404-ns", "vis-404-ns")

	payload := `{"visibility":"public"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/mcp/servers/vis-404-ns/nonexistent/visibility",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminCtx())
	rec := httptest.NewRecorder()
	newMCPRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ─── Agent Visibility ────────────────────────────────────────────────────────

func TestAgentHandler_SetVisibility_Valid(t *testing.T) {
	resetTables(t)
	seedAgent(t, "agvis-ns", "agvis-ag")

	r := newAgentRouter()

	tests := []struct {
		visibility string
	}{
		{"public"},
		{"private"},
	}

	for _, tt := range tests {
		t.Run(tt.visibility, func(t *testing.T) {
			payload := `{"visibility":"` + tt.visibility + `"}`
			req := httptest.NewRequest(http.MethodPost,
				"/api/v1/agents/agvis-ns/agvis-ag/visibility",
				bytes.NewBufferString(payload))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(adminAgentCtx())
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
			}
			var body map[string]string
			json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
			if body["visibility"] != tt.visibility {
				t.Errorf("visibility = %q, want %q", body["visibility"], tt.visibility)
			}
		})
	}
}

func TestAgentHandler_SetVisibility_InvalidValue(t *testing.T) {
	resetTables(t)
	seedAgent(t, "agvis-inv-ns", "agvis-inv-ag")

	payload := `{"visibility":"unknown"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/agents/agvis-inv-ns/agvis-inv-ag/visibility",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestAgentHandler_SetVisibility_UnknownAgent(t *testing.T) {
	resetTables(t)
	seedPublisher(t, "agvis-404-ns", "agvis-404-ns")

	payload := `{"visibility":"public"}`
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/agents/agvis-404-ns/nonexistent/visibility",
		bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(adminAgentCtx())
	rec := httptest.NewRecorder()
	newAgentRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
