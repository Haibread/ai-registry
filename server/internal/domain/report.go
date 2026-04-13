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
	IssueType    string
	Description  string
	ReporterIP   string
	Status       ReportStatus
	CreatedAt    time.Time
	ReviewedAt   *time.Time
	ReviewedBy   string
}
