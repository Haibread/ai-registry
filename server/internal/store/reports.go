package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/haibread/ai-registry/internal/domain"
)

// CreateReportParams holds the fields needed to create a new report.
type CreateReportParams struct {
	ResourceType string
	ResourceID   string
	IssueType    string
	Description  string
	ReporterIP   string
}

// CreateReport inserts a new pending report.
func (db *DB) CreateReport(ctx context.Context, p CreateReportParams) (*domain.Report, error) {
	ctx, span := startSpan(ctx, "CreateReport")
	defer span.End()

	id := NewULID()
	now := time.Now().UTC()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO reports
		    (id, resource_type, resource_id, issue_type, description, reporter_ip, status, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,'pending',$7)`,
		id, p.ResourceType, p.ResourceID, p.IssueType, p.Description, p.ReporterIP, now,
	)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("creating report: %w", err)
	}
	return &domain.Report{
		ID:           id,
		ResourceType: p.ResourceType,
		ResourceID:   p.ResourceID,
		IssueType:    p.IssueType,
		Description:  p.Description,
		ReporterIP:   p.ReporterIP,
		Status:       domain.ReportStatusPending,
		CreatedAt:    now,
	}, nil
}

// ListReportsParams controls filtering for ListReports.
type ListReportsParams struct {
	Status string // "pending" | "reviewed" | "dismissed" | "" (all)
	Limit  int32
}

// ListReports returns a page of reports, newest first.
func (db *DB) ListReports(ctx context.Context, p ListReportsParams) ([]domain.Report, error) {
	ctx, span := startSpan(ctx, "ListReports")
	defer span.End()

	if p.Limit <= 0 || p.Limit > 200 {
		p.Limit = 50
	}

	// Join the reported entry (and its publisher) so the admin UI can link
	// to the entry detail page and show a human-readable name instead of a
	// bare ULID. LEFT JOINs: a report must survive its target's deletion.
	q := `SELECT r.id, r.resource_type, r.resource_id, r.issue_type, r.description,
		      coalesce(r.reporter_ip, ''), r.status, r.created_at, r.reviewed_at, coalesce(r.reviewed_by, ''),
		      coalesce(pm.slug, pa.slug, '') AS resource_ns,
		      coalesce(m.slug, a.slug, '') AS resource_slug,
		      coalesce(m.name, a.name, '') AS resource_name
		  FROM reports r
		  LEFT JOIN mcp_servers m ON r.resource_type = 'mcp_server' AND m.id = r.resource_id
		  LEFT JOIN publishers pm ON pm.id = m.publisher_id
		  LEFT JOIN agents a ON r.resource_type = 'agent' AND a.id = r.resource_id
		  LEFT JOIN publishers pa ON pa.id = a.publisher_id`
	args := []any{}
	if p.Status != "" {
		q += " WHERE r.status = $1"
		args = append(args, p.Status)
	}
	q += fmt.Sprintf(" ORDER BY r.created_at DESC LIMIT $%d", len(args)+1)
	args = append(args, p.Limit)

	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing reports: %w", err)
	}
	defer rows.Close()

	var result []domain.Report
	for rows.Next() {
		var r domain.Report
		var status string
		if err := rows.Scan(
			&r.ID, &r.ResourceType, &r.ResourceID, &r.IssueType, &r.Description,
			&r.ReporterIP, &status, &r.CreatedAt, &r.ReviewedAt, &r.ReviewedBy,
			&r.ResourceNS, &r.ResourceSlug, &r.ResourceName,
		); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning report row: %w", err)
		}
		r.Status = domain.ReportStatus(status)
		result = append(result, r)
	}
	return result, rows.Err()
}

// UpdateReportStatus marks a report as reviewed or dismissed.
func (db *DB) UpdateReportStatus(ctx context.Context, id string, status domain.ReportStatus, reviewedBy string) error {
	ctx, span := startSpan(ctx, "UpdateReportStatus")
	defer span.End()

	if status != domain.ReportStatusReviewed && status != domain.ReportStatusDismissed && status != domain.ReportStatusPending {
		return fmt.Errorf("invalid report status %q", status)
	}

	var reviewedAt *time.Time
	if status != domain.ReportStatusPending {
		now := time.Now().UTC()
		reviewedAt = &now
	}

	tag, err := db.Pool.Exec(ctx, `
		UPDATE reports
		SET status = $1, reviewed_at = $2, reviewed_by = $3
		WHERE id = $4`,
		string(status), reviewedAt, reviewedBy, id,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		recordErr(span, err)
		return fmt.Errorf("updating report status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
