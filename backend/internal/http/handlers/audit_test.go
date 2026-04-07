package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/http/handlers"
)

func newAuditRouter() *chi.Mux {
	h := handlers.NewAuditHandlers(testDB)
	r := chi.NewRouter()
	r.Get("/api/v1/audit", h.ListEvents)
	return r
}

// logAuditEvent inserts a test audit event directly via the store.
func logAuditEvent(t *testing.T, resourceType, resourceID, action string) {
	t.Helper()
	testDB.LogAuditEvent(context.Background(), domain.AuditEvent{
		ActorSubject: "test-subject",
		ActorEmail:   "test@example.com",
		Action:       domain.AuditAction(action),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourceNS:   "test-ns",
		ResourceSlug: "test-slug",
	})
}

// ─── ListEvents ─────────────────────────────────────────────────────────────

func TestAuditHandler_ListEvents_Empty(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	rec := httptest.NewRecorder()
	newAuditRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []any `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if body.Items == nil {
		t.Error("items should be empty array, not null")
	}
	if len(body.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(body.Items))
	}
}

func TestAuditHandler_ListEvents_AfterLoggingEvents(t *testing.T) {
	resetTables(t)

	logAuditEvent(t, "mcp_server", "id-001", "mcp_server.created")
	logAuditEvent(t, "agent", "id-002", "agent.created")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	rec := httptest.NewRecorder()
	newAuditRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []struct {
			ID           string `json:"id"`
			Action       string `json:"action"`
			ResourceType string `json:"resource_type"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(body.Items))
	}
	// Newest first
	if body.Items[0].ResourceType != "agent" {
		t.Errorf("first item resource_type = %q, want agent (newest first)", body.Items[0].ResourceType)
	}
}

func TestAuditHandler_ListEvents_ResourceTypeFilter(t *testing.T) {
	resetTables(t)

	logAuditEvent(t, "mcp_server", "id-mcp1", "mcp_server.created")
	logAuditEvent(t, "mcp_server", "id-mcp2", "mcp_server.deprecated")
	logAuditEvent(t, "agent", "id-ag1", "agent.created")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?resource_type=mcp_server", nil)
	rec := httptest.NewRecorder()
	newAuditRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []struct {
			ResourceType string `json:"resource_type"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	if len(body.Items) != 2 {
		t.Errorf("expected 2 mcp_server events, got %d", len(body.Items))
	}
	for _, item := range body.Items {
		if item.ResourceType != "mcp_server" {
			t.Errorf("unexpected resource_type %q", item.ResourceType)
		}
	}
}

func TestAuditHandler_ListEvents_LimitPagination(t *testing.T) {
	resetTables(t)

	// Seed 5 events
	for i := 0; i < 5; i++ {
		logAuditEvent(t, "mcp_server", "id-page"+string(rune('0'+i)), "mcp_server.created")
	}

	// First page of 3
	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit?limit=3", nil)
	rec := httptest.NewRecorder()
	newAuditRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var page1 struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
		NextCursor string `json:"next_cursor"`
	}
	json.NewDecoder(rec.Body).Decode(&page1) //nolint:errcheck
	if len(page1.Items) != 3 {
		t.Errorf("page1 len = %d, want 3", len(page1.Items))
	}
	if page1.NextCursor == "" {
		t.Error("expected non-empty next_cursor")
	}

	// Second page
	req = httptest.NewRequest(http.MethodGet, "/api/v1/audit?limit=3&cursor="+page1.NextCursor, nil)
	rec = httptest.NewRecorder()
	newAuditRouter().ServeHTTP(rec, req)

	var page2 struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	json.NewDecoder(rec.Body).Decode(&page2) //nolint:errcheck
	if len(page2.Items) != 2 {
		t.Errorf("page2 len = %d, want 2", len(page2.Items))
	}

	// No overlap
	seen := map[string]bool{}
	for _, item := range page1.Items {
		seen[item.ID] = true
	}
	for _, item := range page2.Items {
		if seen[item.ID] {
			t.Errorf("item %s appeared on both pages", item.ID)
		}
	}
}

func TestAuditHandler_ListEvents_ResponseShape(t *testing.T) {
	resetTables(t)
	logAuditEvent(t, "publisher", "id-pub1", "publisher.created")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	rec := httptest.NewRecorder()
	newAuditRouter().ServeHTTP(rec, req)

	var body map[string]json.RawMessage
	json.NewDecoder(rec.Body).Decode(&body) //nolint:errcheck
	for _, key := range []string{"items", "next_cursor"} {
		if _, ok := body[key]; !ok {
			t.Errorf("response missing key %q", key)
		}
	}
}
