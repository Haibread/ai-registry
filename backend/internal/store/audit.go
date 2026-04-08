package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/haibread/ai-registry/internal/domain"
)

// AuditLogger is the interface handlers use to record audit events.
// Implementations must not propagate errors — a failed audit write must
// never cause the primary mutation to fail.
type AuditLogger interface {
	LogAuditEvent(ctx context.Context, e domain.AuditEvent)
}

// ListAuditParams controls filtering and pagination for ListAuditEvents.
type ListAuditParams struct {
	ResourceType string // optional filter
	ResourceID   string // optional filter
	ActorSubject string // optional filter
	Limit        int32
	Cursor       string // created_at cursor (DESC ordering)
}

// LogAuditEvent inserts a single audit record. Errors are logged but not
// returned — callers must not fail their main request if audit logging fails.
func (db *DB) LogAuditEvent(ctx context.Context, e domain.AuditEvent) {
	ctx, span := startSpan(ctx, "LogAuditEvent")
	defer span.End()

	id := NewULID()
	if e.ID != "" {
		id = e.ID
	}

	var metaJSON []byte
	if e.Metadata != nil {
		b, err := json.Marshal(e.Metadata)
		if err == nil {
			metaJSON = b
		}
	}

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO audit_log
			(id, actor_subject, actor_email, action, resource_type, resource_id,
			 resource_ns, resource_slug, metadata, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,NOW())`,
		id,
		e.ActorSubject,
		e.ActorEmail,
		string(e.Action),
		e.ResourceType,
		e.ResourceID,
		e.ResourceNS,
		e.ResourceSlug,
		metaJSON,
	)
	if err != nil {
		recordErr(span, err)
		slog.Error("failed to write audit event",
			"action", e.Action,
			"resource_id", e.ResourceID,
			"error", err,
		)
	}
}

// ListAuditEvents returns a page of audit records, newest first.
func (db *DB) ListAuditEvents(ctx context.Context, p ListAuditParams) ([]domain.AuditEvent, error) {
	ctx, span := startSpan(ctx, "ListAuditEvents")
	defer span.End()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}

	args := []any{}
	argN := 1
	where := "WHERE 1=1"

	if p.ResourceType != "" {
		where += fmt.Sprintf(" AND resource_type = $%d", argN)
		args = append(args, p.ResourceType)
		argN++
	}
	if p.ResourceID != "" {
		where += fmt.Sprintf(" AND resource_id = $%d", argN)
		args = append(args, p.ResourceID)
		argN++
	}
	if p.ActorSubject != "" {
		where += fmt.Sprintf(" AND actor_subject = $%d", argN)
		args = append(args, p.ActorSubject)
		argN++
	}
	if p.Cursor != "" {
		at, id, err := decodeCursor(p.Cursor)
		if err == nil {
			// DESC: next page has older rows
			where += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argN, argN+1)
			args = append(args, at, id)
			argN += 2
		}
	}

	args = append(args, p.Limit)
	q := fmt.Sprintf(`
		SELECT id, actor_subject, actor_email, action,
		       resource_type, resource_id, resource_ns, resource_slug,
		       metadata, created_at
		FROM audit_log
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, where, argN)

	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing audit events: %w", err)
	}
	defer rows.Close()

	var result []domain.AuditEvent
	for rows.Next() {
		var e domain.AuditEvent
		var metaRaw []byte
		var action string
		if err := rows.Scan(
			&e.ID, &e.ActorSubject, &e.ActorEmail, &action,
			&e.ResourceType, &e.ResourceID, &e.ResourceNS, &e.ResourceSlug,
			&metaRaw, &e.CreatedAt,
		); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning audit row: %w", err)
		}
		e.Action = domain.AuditAction(action)
		if len(metaRaw) > 0 {
			_ = json.Unmarshal(metaRaw, &e.Metadata)
		}
		result = append(result, e)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// EncodeCursorFromTime builds a cursor from a timestamp and ID (for audit log
// which uses created_at directly, not a time.Time from a struct).
func EncodeCursorFromTime(t time.Time, id string) string {
	return EncodeCursor(t, id)
}
