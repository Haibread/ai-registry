package domain

import "time"

// Workspace is the team-level grouping under a publisher. A workspace
// owns MCP servers and agents; the publisher is reached by joining
// through workspaces.publisher_id.
type Workspace struct {
	ID          string
	PublisherID string
	Slug        string
	Name        string
	Description string
	Contact     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
