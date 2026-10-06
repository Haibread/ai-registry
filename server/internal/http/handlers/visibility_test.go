package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
)

// ─── MCP Server Visibility ───────────────────────────────────────────────────

func TestMCPHandler_SetVisibility_Valid(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "vis-ns", "vis-srv")
	// Going public requires an approved (published) version, so promote the
	// server first; otherwise the public flip is rejected with 409.
	srv, err := testDB.GetMCPServer(context.Background(), "vis-ns", "vis-srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if err := testDB.SetMCPServerStatus(context.Background(), srv.ID, domain.StatusPublished); err != nil {
		t.Fatalf("SetMCPServerStatus: %v", err)
	}

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

// TestMCPHandler_SetVisibility_PublicRequiresApproval verifies an unapproved
// draft cannot be exposed publicly (409), while going private is always allowed.
func TestMCPHandler_SetVisibility_PublicRequiresApproval(t *testing.T) {
	resetTables(t)
	seedMCPServer(t, "vis-draft-ns", "vis-draft-srv") // draft, never published

	post := func(vis string) int {
		req := httptest.NewRequest(http.MethodPost,
			"/api/v1/mcp/servers/vis-draft-ns/vis-draft-srv/visibility",
			bytes.NewBufferString(`{"visibility":"`+vis+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(adminCtx())
		rec := httptest.NewRecorder()
		newMCPRouter().ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post("public"); code != http.StatusConflict {
		t.Errorf("public on an unapproved draft: status = %d, want 409", code)
	}
	if code := post("private"); code != http.StatusOK {
		t.Errorf("private on a draft: status = %d, want 200", code)
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
