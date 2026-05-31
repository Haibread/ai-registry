package store

import (
	"context"
	"fmt"
)

// StatusBreakdown holds per-status counts for a resource type.
type StatusBreakdown struct {
	Draft      int `json:"draft"`
	Published  int `json:"published"`
	Deprecated int `json:"deprecated"`
}

// RegistryCounts holds the total number of entries for each resource type.
type RegistryCounts struct {
	MCPServers           int              `json:"mcp_servers"`
	Agents               int              `json:"agents"`
	Publishers           int              `json:"publishers"`
	MCPStatusBreakdown   *StatusBreakdown `json:"mcp_status_breakdown,omitempty"`
	AgentStatusBreakdown *StatusBreakdown `json:"agent_status_breakdown,omitempty"`
}

// GetRegistryCounts returns the total row count for each resource table.
// Counts include all visibility and status values (admin view).
func (db *DB) GetRegistryCounts(ctx context.Context) (*RegistryCounts, error) {
	ctx, span := startSpan(ctx, "GetRegistryCounts")
	defer span.End()

	row := db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM mcp_servers)::int,
			(SELECT COUNT(*) FROM agents)::int,
			(SELECT COUNT(*) FROM publishers)::int,
			(SELECT COUNT(*) FROM mcp_servers WHERE status='draft')::int,
			(SELECT COUNT(*) FROM mcp_servers WHERE status='published')::int,
			(SELECT COUNT(*) FROM mcp_servers WHERE status='deprecated')::int,
			(SELECT COUNT(*) FROM agents WHERE status='draft')::int,
			(SELECT COUNT(*) FROM agents WHERE status='published')::int,
			(SELECT COUNT(*) FROM agents WHERE status='deprecated')::int
	`)

	var c RegistryCounts
	var mcpBd, agentBd StatusBreakdown
	if err := row.Scan(&c.MCPServers, &c.Agents, &c.Publishers,
		&mcpBd.Draft, &mcpBd.Published, &mcpBd.Deprecated,
		&agentBd.Draft, &agentBd.Published, &agentBd.Deprecated,
	); err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting registry counts: %w", err)
	}
	c.MCPStatusBreakdown = &mcpBd
	c.AgentStatusBreakdown = &agentBd
	return &c, nil
}

// MemberRoles counts the role grants scoped to a single publisher, by role.
// A principal that holds two roles (e.g. Editor + Reviewer for deliberate
// dual-hatting) is counted under each, so these can exceed the distinct
// Members total.
type MemberRoles struct {
	Viewers   int `json:"viewers"`
	Editors   int `json:"editors"`
	Reviewers int `json:"reviewers"`
	Admins    int `json:"admins"`
}

// PublisherStats is the per-publisher rollup powering the scoped admin home.
// Totals exclude soft-deleted entries; the breakdowns cover the three live
// statuses. PendingReview is the count of items awaiting an approver on this
// publisher (submitted versions + requested deletions) — the "needs review"
// signal. Members counts distinct principals holding a grant scoped to this
// publisher (global-grant holders are not counted as members of one publisher).
type PublisherStats struct {
	MCPServers           int              `json:"mcp_servers"`
	Agents               int              `json:"agents"`
	Members              int              `json:"members"`
	PendingReview        int              `json:"pending_review"`
	MCPStatusBreakdown   *StatusBreakdown `json:"mcp_status_breakdown"`
	AgentStatusBreakdown *StatusBreakdown `json:"agent_status_breakdown"`
	MemberRoles          *MemberRoles     `json:"member_roles"`
}

// GetPublisherStats returns the per-publisher rollup for the given publisher id.
// All subqueries are scoped to publisher_id = $1.
func (db *DB) GetPublisherStats(ctx context.Context, publisherID string) (*PublisherStats, error) {
	ctx, span := startSpan(ctx, "GetPublisherStats")
	defer span.End()

	row := db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM mcp_servers WHERE publisher_id=$1 AND status!='deleted')::int,
			(SELECT COUNT(*) FROM mcp_servers WHERE publisher_id=$1 AND status='draft')::int,
			(SELECT COUNT(*) FROM mcp_servers WHERE publisher_id=$1 AND status='published')::int,
			(SELECT COUNT(*) FROM mcp_servers WHERE publisher_id=$1 AND status='deprecated')::int,
			(SELECT COUNT(*) FROM agents WHERE publisher_id=$1 AND status!='deleted')::int,
			(SELECT COUNT(*) FROM agents WHERE publisher_id=$1 AND status='draft')::int,
			(SELECT COUNT(*) FROM agents WHERE publisher_id=$1 AND status='published')::int,
			(SELECT COUNT(*) FROM agents WHERE publisher_id=$1 AND status='deprecated')::int,
			(SELECT COUNT(DISTINCT principal_id) FROM role_grants WHERE publisher_id=$1)::int,
			(SELECT COUNT(*) FROM role_grants WHERE publisher_id=$1 AND role='viewer')::int,
			(SELECT COUNT(*) FROM role_grants WHERE publisher_id=$1 AND role='editor')::int,
			(SELECT COUNT(*) FROM role_grants WHERE publisher_id=$1 AND role='reviewer')::int,
			(SELECT COUNT(*) FROM role_grants WHERE publisher_id=$1 AND role='admin')::int,
			(
				(SELECT COUNT(*) FROM mcp_server_versions v JOIN mcp_servers s ON s.id=v.server_id
					WHERE s.publisher_id=$1 AND v.review_state='pending_review' AND v.submitted_at IS NOT NULL)
				+ (SELECT COUNT(*) FROM agent_versions av JOIN agents a ON a.id=av.agent_id
					WHERE a.publisher_id=$1 AND av.review_state='pending_review' AND av.submitted_at IS NOT NULL)
				+ (SELECT COUNT(*) FROM mcp_servers WHERE publisher_id=$1 AND deletion_requested_at IS NOT NULL AND deleted_at IS NULL)
				+ (SELECT COUNT(*) FROM agents WHERE publisher_id=$1 AND deletion_requested_at IS NOT NULL AND deleted_at IS NULL)
			)::int
	`, publisherID)

	var s PublisherStats
	var mcpBd, agentBd StatusBreakdown
	var roles MemberRoles
	if err := row.Scan(
		&s.MCPServers, &mcpBd.Draft, &mcpBd.Published, &mcpBd.Deprecated,
		&s.Agents, &agentBd.Draft, &agentBd.Published, &agentBd.Deprecated,
		&s.Members, &roles.Viewers, &roles.Editors, &roles.Reviewers, &roles.Admins,
		&s.PendingReview,
	); err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting publisher stats: %w", err)
	}
	s.MCPStatusBreakdown = &mcpBd
	s.AgentStatusBreakdown = &agentBd
	s.MemberRoles = &roles
	return &s, nil
}

// PublicStats holds counts visible to unauthenticated users (published + public only).
type PublicStats struct {
	MCPServers        int `json:"mcp_servers"`
	Agents            int `json:"agents"`
	Publishers        int `json:"publishers"`
	NewMCPServersWeek int `json:"new_mcp_servers_this_week"`
	NewAgentsWeek     int `json:"new_agents_this_week"`
	NewPublishersWeek int `json:"new_publishers_this_week"`
}

// GetPublicStats returns aggregate counts scoped to published, public entries.
func (db *DB) GetPublicStats(ctx context.Context) (*PublicStats, error) {
	ctx, span := startSpan(ctx, "GetPublicStats")
	defer span.End()

	row := db.Pool.QueryRow(ctx, `
		WITH active_publishers AS (
			SELECT s.publisher_id
			FROM mcp_servers s
			WHERE s.status='published' AND s.visibility='public'
			UNION
			SELECT a.publisher_id
			FROM agents a
			WHERE a.status='published' AND a.visibility='public'
		)
		SELECT
			(SELECT COUNT(*) FROM mcp_servers WHERE status='published' AND visibility='public')::int,
			(SELECT COUNT(*) FROM agents WHERE status='published' AND visibility='public')::int,
			(SELECT COUNT(*) FROM active_publishers)::int,
			(SELECT COUNT(*) FROM mcp_servers WHERE status='published' AND visibility='public' AND created_at > now() - interval '7 days')::int,
			(SELECT COUNT(*) FROM agents WHERE status='published' AND visibility='public' AND created_at > now() - interval '7 days')::int,
			(SELECT COUNT(*) FROM publishers p
				WHERE p.id IN (SELECT publisher_id FROM active_publishers)
				AND p.created_at > now() - interval '7 days')::int
	`)

	var s PublicStats
	if err := row.Scan(
		&s.MCPServers, &s.Agents, &s.Publishers,
		&s.NewMCPServersWeek, &s.NewAgentsWeek, &s.NewPublishersWeek,
	); err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting public stats: %w", err)
	}
	return &s, nil
}
