package domain

import "time"

// AuditAction is the type of mutation that was recorded.
type AuditAction string

const (
	// MCP server actions
	ActionMCPServerCreated      AuditAction = "mcp_server.created"
	ActionMCPVersionCreated     AuditAction = "mcp_server_version.created"
	ActionMCPVersionPublished   AuditAction = "mcp_server_version.published"
	ActionMCPServerDeprecated   AuditAction = "mcp_server.deprecated"
	ActionMCPServerUndeprecated AuditAction = "mcp_server.undeprecated"
	ActionMCPServerVisibility   AuditAction = "mcp_server.visibility_changed"

	// Agent actions
	ActionAgentCreated          AuditAction = "agent.created"
	ActionAgentVersionCreated   AuditAction = "agent_version.created"
	ActionAgentVersionPublished AuditAction = "agent_version.published"
	ActionAgentDeprecated       AuditAction = "agent.deprecated"
	ActionAgentUndeprecated     AuditAction = "agent.undeprecated"
	ActionAgentVisibility       AuditAction = "agent.visibility_changed"

	// MCP server update/delete actions
	ActionMCPServerUpdated AuditAction = "mcp_server.updated"
	ActionMCPServerDeleted AuditAction = "mcp_server.deleted"

	// Agent update/delete actions
	ActionAgentUpdated AuditAction = "agent.updated"
	ActionAgentDeleted AuditAction = "agent.deleted"

	// Publisher actions
	ActionPublisherCreated AuditAction = "publisher.created"
	ActionPublisherUpdated AuditAction = "publisher.updated"
	ActionPublisherDeleted AuditAction = "publisher.deleted"

	// Change-approval workflow actions.
	ActionMCPVersionSubmitted  AuditAction = "mcp_server_version.submitted"
	ActionMCPVersionWithdrawn  AuditAction = "mcp_server_version.withdrawn"
	ActionMCPVersionApproved   AuditAction = "mcp_server_version.approved"
	ActionMCPVersionRejected   AuditAction = "mcp_server_version.rejected"
	ActionMCPDeletionRequested AuditAction = "mcp_server.deletion_requested"
	ActionMCPDeletionApproved  AuditAction = "mcp_server.deletion_approved"
	ActionMCPDeletionRejected  AuditAction = "mcp_server.deletion_rejected"

	ActionAgentVersionSubmitted  AuditAction = "agent_version.submitted"
	ActionAgentVersionWithdrawn  AuditAction = "agent_version.withdrawn"
	ActionAgentVersionApproved   AuditAction = "agent_version.approved"
	ActionAgentVersionRejected   AuditAction = "agent_version.rejected"
	ActionAgentDeletionRequested AuditAction = "agent.deletion_requested"
	ActionAgentDeletionApproved  AuditAction = "agent.deletion_approved"
	ActionAgentDeletionRejected  AuditAction = "agent.deletion_rejected"

	// Entry-change workflow actions. An Editor's visibility / deprecate /
	// metadata-edit request is now queued instead of applied immediately; a
	// Reviewer approves or rejects it. The concrete action and payload live in
	// the event metadata so "what was changed" is answerable from the log.
	// (Server Admins keep the immediate path, which still fires the concrete
	// ActionMCPServerVisibility / ...Deprecated / ...Updated events.)
	ActionMCPChangeRequested AuditAction = "mcp_server.change_requested"
	ActionMCPChangeApproved  AuditAction = "mcp_server.change_approved"
	ActionMCPChangeRejected  AuditAction = "mcp_server.change_rejected"
	ActionMCPChangeWithdrawn AuditAction = "mcp_server.change_withdrawn"

	ActionAgentChangeRequested AuditAction = "agent.change_requested"
	ActionAgentChangeApproved  AuditAction = "agent.change_approved"
	ActionAgentChangeRejected  AuditAction = "agent.change_rejected"
	ActionAgentChangeWithdrawn AuditAction = "agent.change_withdrawn"

	// RBAC actions. The security-sensitive mutations capture the
	// target principal / grant in the event metadata so "who granted whom what,
	// and when" is answerable from the log.
	ActionRoleGrantCreated   AuditAction = "role_grant.created"
	ActionRoleGrantDeleted   AuditAction = "role_grant.deleted"
	ActionGroupCreated       AuditAction = "group.created"
	ActionGroupUpdated       AuditAction = "group.updated"
	ActionGroupDeleted       AuditAction = "group.deleted"
	ActionGroupMemberAdded   AuditAction = "group.member_added"
	ActionGroupMemberRemoved AuditAction = "group.member_removed"
	ActionUserCreated        AuditAction = "user.created"
	ActionUserUpdated        AuditAction = "user.updated"
	ActionUserPasswordSet    AuditAction = "user.password_set"
)

// AuditEvent is a single immutable entry in the audit log.
type AuditEvent struct {
	ID           string
	ActorSubject string // Keycloak subject UUID
	ActorEmail   string // human-readable identity
	Action       AuditAction
	ResourceType string // "mcp_server" | "agent" | "publisher"
	ResourceID   string // ULID of the mutated resource
	ResourceNS   string // publisher slug
	ResourceSlug string // resource slug
	Metadata     map[string]any
	CreatedAt    time.Time
}
