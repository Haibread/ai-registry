package store_test

import (
	"context"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

func TestReports_CreateAndList(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	r1, err := sharedDB.CreateReport(ctx, store.CreateReportParams{
		ResourceType: "mcp_server",
		ResourceID:   "01HMCP",
		IssueType:    "broken",
		Description:  "does not install",
		ReporterIP:   "1.1.1.1",
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}
	if r1.ID == "" {
		t.Fatal("expected ID to be set")
	}
	if r1.Status != domain.ReportStatusPending {
		t.Errorf("status = %q, want pending", r1.Status)
	}

	if _, err := sharedDB.CreateReport(ctx, store.CreateReportParams{
		ResourceType: "agent",
		ResourceID:   "01HAGENT",
		IssueType:    "spam",
		Description:  "this is advertising",
		ReporterIP:   "2.2.2.2",
	}); err != nil {
		t.Fatalf("CreateReport 2: %v", err)
	}

	// ListReports — all
	all, err := sharedDB.ListReports(ctx, store.ListReportsParams{Limit: 50})
	if err != nil {
		t.Fatalf("ListReports: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 reports, got %d", len(all))
	}
	// Newest first — the agent one was inserted second.
	if all[0].ResourceType != "agent" {
		t.Errorf("newest first: [0].resource_type = %q, want agent", all[0].ResourceType)
	}

	// ListReports — status filter
	pending, err := sharedDB.ListReports(ctx, store.ListReportsParams{Status: "pending", Limit: 50})
	if err != nil {
		t.Fatalf("ListReports pending: %v", err)
	}
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}

	dismissed, err := sharedDB.ListReports(ctx, store.ListReportsParams{Status: "dismissed", Limit: 50})
	if err != nil {
		t.Fatalf("ListReports dismissed: %v", err)
	}
	if len(dismissed) != 0 {
		t.Errorf("expected 0 dismissed, got %d", len(dismissed))
	}
}

func TestReports_UpdateStatus(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	r, err := sharedDB.CreateReport(ctx, store.CreateReportParams{
		ResourceType: "mcp_server",
		ResourceID:   "01HMCP",
		IssueType:    "other",
		Description:  "initial description",
	})
	if err != nil {
		t.Fatalf("CreateReport: %v", err)
	}

	// Mark reviewed
	if err := sharedDB.UpdateReportStatus(ctx, r.ID, domain.ReportStatusReviewed, "admin-sub"); err != nil {
		t.Fatalf("UpdateReportStatus: %v", err)
	}

	// Fetch via list & assert state
	all, _ := sharedDB.ListReports(ctx, store.ListReportsParams{Limit: 10})
	if len(all) != 1 {
		t.Fatalf("expected 1 report, got %d", len(all))
	}
	got := all[0]
	if got.Status != domain.ReportStatusReviewed {
		t.Errorf("status = %q, want reviewed", got.Status)
	}
	if got.ReviewedBy != "admin-sub" {
		t.Errorf("reviewed_by = %q, want admin-sub", got.ReviewedBy)
	}
	if got.ReviewedAt == nil {
		t.Error("reviewed_at should be set")
	}

	// Reopen — pending clears reviewed_at
	if err := sharedDB.UpdateReportStatus(ctx, r.ID, domain.ReportStatusPending, "admin-sub"); err != nil {
		t.Fatalf("UpdateReportStatus reopen: %v", err)
	}
	all, _ = sharedDB.ListReports(ctx, store.ListReportsParams{Limit: 10})
	if all[0].ReviewedAt != nil {
		t.Error("reviewed_at should be cleared when moving back to pending")
	}
}

func TestReports_UpdateStatus_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	err := sharedDB.UpdateReportStatus(ctx, "01HNOSUCH", domain.ReportStatusReviewed, "admin")
	if err == nil {
		t.Fatal("expected error for nonexistent id")
	}
}

func TestReports_UpdateStatus_Invalid(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	r, _ := sharedDB.CreateReport(ctx, store.CreateReportParams{
		ResourceType: "agent", ResourceID: "01H", IssueType: "other", Description: "x",
	})
	if err := sharedDB.UpdateReportStatus(ctx, r.ID, "bogus", "admin"); err == nil {
		t.Error("expected error for invalid status")
	}
}
