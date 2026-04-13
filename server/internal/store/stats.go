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
	MCPServers        int             `json:"mcp_servers"`
	Agents            int             `json:"agents"`
	Publishers        int             `json:"publishers"`
	MCPStatusBreakdown *StatusBreakdown `json:"mcp_status_breakdown,omitempty"`
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
			SELECT publisher_id FROM mcp_servers WHERE status='published' AND visibility='public'
			UNION
			SELECT publisher_id FROM agents WHERE status='published' AND visibility='public'
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
