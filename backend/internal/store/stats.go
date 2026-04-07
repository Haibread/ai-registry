package store

import (
	"context"
	"fmt"
)

// RegistryCounts holds the total number of entries for each resource type.
type RegistryCounts struct {
	MCPServers int `json:"mcp_servers"`
	Agents     int `json:"agents"`
	Publishers int `json:"publishers"`
}

// GetRegistryCounts returns the total row count for each resource table.
// Counts include all visibility and status values (admin view).
func (db *DB) GetRegistryCounts(ctx context.Context) (*RegistryCounts, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM mcp_servers)::int,
			(SELECT COUNT(*) FROM agents)::int,
			(SELECT COUNT(*) FROM publishers)::int
	`)

	var c RegistryCounts
	if err := row.Scan(&c.MCPServers, &c.Agents, &c.Publishers); err != nil {
		return nil, fmt.Errorf("getting registry counts: %w", err)
	}
	return &c, nil
}
