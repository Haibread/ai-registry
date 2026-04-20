package handlers

import (
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/http/middleware"
	"github.com/haibread/ai-registry/internal/problem"
	"github.com/haibread/ai-registry/internal/store"
)

// ReportHandlers serves the community issue-report API.
type ReportHandlers struct {
	db           *store.DB
	trustedProxy *net.IPNet
}

// NewReportHandlers creates a ReportHandlers with the given store. When
// trustedProxy is non-nil, X-Forwarded-For is honoured for reporter IP
// extraction from connections originating inside that CIDR. Pass nil when
// the server is directly internet-facing so untrusted clients cannot spoof
// their reporter IP via a forged XFF header.
func NewReportHandlers(db *store.DB, trustedProxy *net.IPNet) *ReportHandlers {
	return &ReportHandlers{db: db, trustedProxy: trustedProxy}
}

// allowed issue types — keep short, structured, and stable. Free-form
// descriptions carry the details.
var allowedIssueTypes = map[string]struct{}{
	"broken":        {},
	"misleading":    {},
	"spam":          {},
	"security":      {},
	"licensing":     {},
	"outdated":      {},
	"duplicate":     {},
	"other":         {},
}

var allowedResourceTypes = map[string]struct{}{
	"mcp_server": {},
	"agent":      {},
}

// CreateReport handles POST /api/v1/reports.
//
// Body: { resource_type, resource_id, issue_type, description }
//
// Public, rate-limited. Any authenticated user or anonymous visitor may file a
// report; the reporter's IP is captured for abuse-handling purposes but never
// returned to non-admins.
func (h *ReportHandlers) CreateReport(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
		IssueType    string `json:"issue_type"`
		Description  string `json:"description"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	body.ResourceType = strings.TrimSpace(body.ResourceType)
	body.ResourceID = strings.TrimSpace(body.ResourceID)
	body.IssueType = strings.TrimSpace(body.IssueType)
	body.Description = strings.TrimSpace(body.Description)

	if _, ok := allowedResourceTypes[body.ResourceType]; !ok {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"resource_type must be 'mcp_server' or 'agent'", r.URL.Path)
		return
	}
	if body.ResourceID == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"resource_id is required", r.URL.Path)
		return
	}
	if _, ok := allowedIssueTypes[body.IssueType]; !ok {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"issue_type must be one of broken, misleading, spam, security, licensing, outdated, duplicate, other", r.URL.Path)
		return
	}
	if len(body.Description) < 5 {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"description must be at least 5 characters", r.URL.Path)
		return
	}
	if len(body.Description) > 4000 {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"description must be at most 4000 characters", r.URL.Path)
		return
	}

	rep, err := h.db.CreateReport(r.Context(), store.CreateReportParams{
		ResourceType: body.ResourceType,
		ResourceID:   body.ResourceID,
		IssueType:    body.IssueType,
		Description:  body.Description,
		ReporterIP:   middleware.ClientIP(r, h.trustedProxy),
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusCreated, reportToResponse(rep, false))
}

// ListReports handles GET /api/v1/reports (admin).
//
// Query params:
//
//	status — optional filter: "pending" | "reviewed" | "dismissed"
//	limit  — optional page size (1–200, default 50)
func (h *ReportHandlers) ListReports(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status != "" && status != string(domain.ReportStatusPending) &&
		status != string(domain.ReportStatusReviewed) &&
		status != string(domain.ReportStatusDismissed) {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"status must be pending, reviewed, or dismissed", r.URL.Path)
		return
	}

	limit := int32(50)
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 200 {
			limit = int32(n)
		}
	}

	items, err := h.db.ListReports(r.Context(), store.ListReportsParams{
		Status: status,
		Limit:  limit,
	})
	if err != nil {
		internalError(w, r, err)
		return
	}

	out := make([]map[string]any, 0, len(items))
	for i := range items {
		out = append(out, reportToResponse(&items[i], true))
	}
	writeJSON(w, r, http.StatusOK, map[string]any{"items": out})
}

// PatchReport handles PATCH /api/v1/reports/{id} (admin).
//
// Body: { status: "reviewed" | "dismissed" | "pending" }
func (h *ReportHandlers) PatchReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"report id is required", r.URL.Path)
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	st := domain.ReportStatus(strings.TrimSpace(body.Status))
	if st != domain.ReportStatusPending && st != domain.ReportStatusReviewed && st != domain.ReportStatusDismissed {
		problem.Write(w, http.StatusUnprocessableEntity, "validation-error",
			"status must be pending, reviewed, or dismissed", r.URL.Path)
		return
	}

	reviewer, _ := auditActor(r.Context())

	if err := h.db.UpdateReportStatus(r.Context(), id, st, reviewer); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			problem.Write(w, http.StatusNotFound, "not-found",
				"report not found", r.URL.Path)
			return
		}
		internalError(w, r, err)
		return
	}

	writeJSON(w, r, http.StatusOK, map[string]string{"status": string(st)})
}

// reportToResponse serialises a report for the API. When includeIP is false
// (i.e. when returned to the submitter of a new report) the reporter IP is
// omitted.
func reportToResponse(r *domain.Report, includeIP bool) map[string]any {
	m := map[string]any{
		"id":            r.ID,
		"resource_type": r.ResourceType,
		"resource_id":   r.ResourceID,
		"issue_type":    r.IssueType,
		"description":   r.Description,
		"status":        string(r.Status),
		"created_at":    r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if includeIP {
		m["reporter_ip"] = r.ReporterIP
		m["reviewed_by"] = r.ReviewedBy
		if r.ReviewedAt != nil {
			m["reviewed_at"] = r.ReviewedAt.Format("2006-01-02T15:04:05Z07:00")
		} else {
			m["reviewed_at"] = nil
		}
	}
	return m
}

