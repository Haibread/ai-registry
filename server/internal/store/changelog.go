package store

import (
	"context"
	"fmt"
	"time"
)

// ChangelogEntry represents one recently-published version (from either the
// MCP or agent registry) in the aggregated public changelog feed.
type ChangelogEntry struct {
	ResourceType string    // "mcp_server" | "agent"
	Namespace    string    // publisher slug
	Slug         string    // resource slug
	Name         string    // human-readable resource name
	Version      string    // semver
	PublishedAt  time.Time // when the version transitioned to published
}

// ListChangelog returns the most recently published versions across both
// registries, merged and sorted by published_at descending. Only public,
// non-deprecated resources are included.
//
// limit is clamped to [1, 200]; 50 is the default when limit <= 0.
func (db *DB) ListChangelog(ctx context.Context, limit int) ([]ChangelogEntry, error) {
	ctx, span := startSpan(ctx, "ListChangelog")
	defer span.End()

	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}

	rows, err := db.Pool.Query(ctx, `
		(
			SELECT 'mcp_server' AS resource_type,
			       pub.slug      AS namespace,
			       s.slug        AS slug,
			       s.name        AS name,
			       v.version     AS version,
			       v.published_at
			FROM mcp_server_versions v
			JOIN mcp_servers s ON s.id = v.server_id
			JOIN publishers pub ON pub.id = s.publisher_id
			WHERE v.published_at IS NOT NULL
			  AND s.visibility = 'public'
			  AND s.status <> 'deleted'
		)
		UNION ALL
		(
			SELECT 'agent'       AS resource_type,
			       pub.slug      AS namespace,
			       a.slug        AS slug,
			       a.name        AS name,
			       v.version     AS version,
			       v.published_at
			FROM agent_versions v
			JOIN agents a ON a.id = v.agent_id
			JOIN publishers pub ON pub.id = a.publisher_id
			WHERE v.published_at IS NOT NULL
			  AND a.visibility = 'public'
			  AND a.status <> 'deleted'
		)
		ORDER BY published_at DESC
		LIMIT $1
	`, limit)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("querying changelog: %w", err)
	}
	defer rows.Close()

	var result []ChangelogEntry
	for rows.Next() {
		var e ChangelogEntry
		if err := rows.Scan(&e.ResourceType, &e.Namespace, &e.Slug, &e.Name, &e.Version, &e.PublishedAt); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning changelog row: %w", err)
		}
		result = append(result, e)
	}
	return result, rows.Err()
}
