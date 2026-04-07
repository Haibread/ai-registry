package domain

import "time"

// AuditAction is the type of mutation that was recorded.
type AuditAction string

const (
	// MCP server actions
	ActionMCPServerCreated    AuditAction = "mcp_server.created"
	ActionMCPVersionCreated   AuditAction = "mcp_server_version.created"
	ActionMCPVersionPublished AuditAction = "mcp_server_version.published"
	ActionMCPServerDeprecated AuditAction = "mcp_server.deprecated"
	ActionMCPServerVisibility AuditAction = "mcp_server.visibility_changed"

	// Agent actions
	ActionAgentCreated        AuditAction = "agent.created"
	ActionAgentVersionCreated AuditAction = "agent_version.created"
	ActionAgentVersionPublished AuditAction = "agent_version.published"
	ActionAgentDeprecated     AuditAction = "agent.deprecated"
	ActionAgentVisibility     AuditAction = "agent.visibility_changed"

	// Publisher actions
	ActionPublisherCreated AuditAction = "publisher.created"
)

// AuditEvent is a single immutable entry in the audit log.
type AuditEvent struct {
	ID           string
	ActorSubject string      // Keycloak subject UUID
	ActorEmail   string      // human-readable identity
	Action       AuditAction
	ResourceType string      // "mcp_server" | "agent" | "publisher"
	ResourceID   string      // ULID of the mutated resource
	ResourceNS   string      // publisher slug
	ResourceSlug string      // resource slug
	Metadata     map[string]any
	CreatedAt    time.Time
}
