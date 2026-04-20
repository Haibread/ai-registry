package handlers_test

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/http/handlers"
)

func newReportsRouter() *chi.Mux {
	return newReportsRouterWithProxy(nil)
}

func newReportsRouterWithProxy(trustedProxy *net.IPNet) *chi.Mux {
	h := handlers.NewReportHandlers(testDB, trustedProxy)
	r := chi.NewRouter()
	r.Post("/api/v1/reports", h.CreateReport)
	r.Get("/api/v1/reports", h.ListReports)
	r.Patch("/api/v1/reports/{id}", h.PatchReport)
	return r
}

func postReport(t *testing.T, body any) *httptest.ResponseRecorder {
	t.Helper()
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newReportsRouter().ServeHTTP(rec, req)
	return rec
}

func TestReportHandler_Create_Valid(t *testing.T) {
	resetTables(t)

	rec := postReport(t, map[string]any{
		"resource_type": "mcp_server",
		"resource_id":   "01HSRV",
		"issue_type":    "broken",
		"description":   "does not install on any platform",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201. body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "pending" {
		t.Errorf("status = %v, want pending", body["status"])
	}
	// Reporter IP should NOT be returned to the submitter.
	if _, ok := body["reporter_ip"]; ok {
		t.Error("reporter_ip should be hidden in create response")
	}
}

func TestReportHandler_Create_ValidationErrors(t *testing.T) {
	resetTables(t)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"bad resource_type", map[string]any{"resource_type": "widget", "resource_id": "1", "issue_type": "broken", "description": "bad thing happening"}},
		{"missing resource_id", map[string]any{"resource_type": "agent", "resource_id": "", "issue_type": "broken", "description": "bad thing happening"}},
		{"bad issue_type", map[string]any{"resource_type": "agent", "resource_id": "1", "issue_type": "flippity", "description": "bad thing happening"}},
		{"short description", map[string]any{"resource_type": "agent", "resource_id": "1", "issue_type": "broken", "description": "hi"}},
		{"long description", map[string]any{"resource_type": "agent", "resource_id": "1", "issue_type": "broken", "description": strings.Repeat("x", 4001)}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := postReport(t, c.body)
			if rec.Code != http.StatusUnprocessableEntity {
				t.Errorf("status = %d, want 422. body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestReportHandler_List_Empty(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports", nil)
	rec := httptest.NewRecorder()
	newReportsRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Items []any `json:"items"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.Items == nil {
		t.Error("items should be empty array, not nil")
	}
	if len(body.Items) != 0 {
		t.Errorf("expected 0 items, got %d", len(body.Items))
	}
}

func TestReportHandler_List_IncludesReporterIP(t *testing.T) {
	resetTables(t)

	// Seed via the handler so reporter_ip is captured from the request.
	rec := postReport(t, map[string]any{
		"resource_type": "mcp_server",
		"resource_id":   "01HSRV",
		"issue_type":    "broken",
		"description":   "does not install on any platform",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("seed status = %d", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports?status=pending", nil)
	out := httptest.NewRecorder()
	newReportsRouter().ServeHTTP(out, req)

	if out.Code != http.StatusOK {
		t.Fatalf("list status = %d", out.Code)
	}
	var body struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.NewDecoder(out.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(body.Items))
	}
	if _, ok := body.Items[0]["reporter_ip"]; !ok {
		t.Error("list response for admins should include reporter_ip")
	}
}

// TestReportHandler_Create_XFFIgnoredWithoutTrustedProxy pins H3: when the
// server is deployed directly internet-facing (no trusted proxy configured),
// an X-Forwarded-For header from an untrusted client must NOT be used as the
// reporter IP. Otherwise anyone could poison reporter_ip audit data by
// forging the header.
func TestReportHandler_Create_XFFIgnoredWithoutTrustedProxy(t *testing.T) {
	resetTables(t)

	buf, _ := json.Marshal(map[string]any{
		"resource_type": "mcp_server",
		"resource_id":   "01HSRV",
		"issue_type":    "broken",
		"description":   "attempting to spoof reporter_ip via XFF",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.0.1")
	req.RemoteAddr = "203.0.113.5:4444"
	rec := httptest.NewRecorder()
	newReportsRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201. body=%s", rec.Code, rec.Body.String())
	}

	// List as admin and confirm the reporter_ip was NOT taken from the
	// attacker-supplied XFF header.
	list := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports?status=pending", nil)
	newReportsRouter().ServeHTTP(list, listReq)
	var body struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.NewDecoder(list.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(body.Items))
	}
	got, _ := body.Items[0]["reporter_ip"].(string)
	if got == "10.0.0.1" {
		t.Errorf("reporter_ip = %q — XFF must be ignored when no trusted proxy is configured", got)
	}
	if got != "203.0.113.5" {
		t.Errorf("reporter_ip = %q, want RemoteAddr host %q", got, "203.0.113.5")
	}
}

func TestReportHandler_Create_XFFTrustedFromProxy(t *testing.T) {
	resetTables(t)

	// Accept XFF from connections inside 127.0.0.0/8 (the proxy).
	_, proxyCIDR, err := net.ParseCIDR("127.0.0.0/8")
	if err != nil {
		t.Fatalf("parse cidr: %v", err)
	}
	router := newReportsRouterWithProxy(proxyCIDR)

	buf, _ := json.Marshal(map[string]any{
		"resource_type": "agent",
		"resource_id":   "01HAG",
		"issue_type":    "spam",
		"description":   "report posted through the reverse proxy",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reports", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "198.51.100.42, 127.0.0.1")
	req.RemoteAddr = "127.0.0.1:4444"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d. body=%s", rec.Code, rec.Body.String())
	}

	list := httptest.NewRecorder()
	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/reports?status=pending", nil)
	router.ServeHTTP(list, listReq)
	var body struct {
		Items []map[string]any `json:"items"`
	}
	_ = json.NewDecoder(list.Body).Decode(&body)
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(body.Items))
	}
	got, _ := body.Items[0]["reporter_ip"].(string)
	if got != "198.51.100.42" {
		t.Errorf("reporter_ip = %q, want leftmost XFF entry %q when request came from a trusted proxy", got, "198.51.100.42")
	}
}

func TestReportHandler_List_InvalidStatus(t *testing.T) {
	resetTables(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/reports?status=bogus", nil)
	rec := httptest.NewRecorder()
	newReportsRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}

func TestReportHandler_Patch_Valid(t *testing.T) {
	resetTables(t)

	// Seed a pending report
	seed := postReport(t, map[string]any{
		"resource_type": "agent",
		"resource_id":   "01HAG",
		"issue_type":    "misleading",
		"description":   "the description says X but it does Y",
	})
	var created map[string]any
	_ = json.NewDecoder(seed.Body).Decode(&created)
	id := created["id"].(string)

	// PATCH to reviewed
	patchBody, _ := json.Marshal(map[string]any{"status": "reviewed"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/reports/"+id, bytes.NewReader(patchBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newReportsRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("patch status = %d, want 200. body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.NewDecoder(rec.Body).Decode(&out)
	if out["status"] != "reviewed" {
		t.Errorf("status = %v, want reviewed", out["status"])
	}
}

func TestReportHandler_Patch_NotFound(t *testing.T) {
	resetTables(t)

	body, _ := json.Marshal(map[string]any{"status": "dismissed"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/reports/01HNOSUCH", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newReportsRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

func TestReportHandler_Patch_InvalidStatus(t *testing.T) {
	resetTables(t)

	body, _ := json.Marshal(map[string]any{"status": "deleted"})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/reports/01HSOMEID", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	newReportsRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", rec.Code)
	}
}
