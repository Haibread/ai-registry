package domain

import "time"

// ReportStatus is the lifecycle state of a user-submitted issue report.
type ReportStatus string

const (
	ReportStatusPending   ReportStatus = "pending"
	ReportStatusReviewed  ReportStatus = "reviewed"
	ReportStatusDismissed ReportStatus = "dismissed"
)

// Report is a community-submitted issue report against a registry entry.
type Report struct {
	ID           string
	ResourceType string // "mcp_server" | "agent"
	ResourceID   string
	// ResourceNS / ResourceSlug / ResourceName identify the reported entry in
	// human terms. Populated on admin list reads (joined from the entry
	// tables); empty when the entry no longer exists.
	ResourceNS   string
	ResourceSlug string
	ResourceName string
	IssueType    string
	Description  string
	ReporterIP   string
	Status       ReportStatus
	CreatedAt    time.Time
	ReviewedAt   *time.Time
	ReviewedBy   string
}
