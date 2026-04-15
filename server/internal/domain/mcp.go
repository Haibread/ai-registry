// Package domain contains the core business entities and validation logic.
package domain

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

// Visibility controls whether a registry entry is publicly listed.
type Visibility string

const (
	VisibilityPrivate Visibility = "private"
	VisibilityPublic  Visibility = "public"
)

// ServerStatus represents the lifecycle state of a registry entry.
type ServerStatus string

const (
	StatusDraft      ServerStatus = "draft"
	StatusPublished  ServerStatus = "published"
	StatusDeprecated ServerStatus = "deprecated"
	StatusDeleted    ServerStatus = "deleted"
)

// Runtime is the transport mechanism of an MCP server version.
type Runtime string

const (
	RuntimeStdio          Runtime = "stdio"
	RuntimeHTTP           Runtime = "http"
	RuntimeSSE            Runtime = "sse"
	RuntimeStreamableHTTP Runtime = "streamable_http"
)

// MCPServer is the top-level entity for an MCP server in the registry.
type MCPServer struct {
	ID          string
	PublisherID string
	Namespace   string // publisher slug
	Slug        string
	Name        string
	Description string
	HomepageURL string
	RepoURL     string
	License     string
	Visibility  Visibility
	Status      ServerStatus
	Featured    bool
	Verified    bool
	Readme      string
	ViewCount   int
	CopyCount   int
	Tags        []string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// VersionStatus is the lifecycle status of a published version.
type VersionStatus string

const (
	VersionStatusActive     VersionStatus = "active"
	VersionStatusDeprecated VersionStatus = "deprecated"
	VersionStatusDeleted    VersionStatus = "deleted"
)

// MCPServerVersion is an immutable versioned release of an MCPServer.
// Once published_at is set, no fields may be mutated.
//
// The JSON tags are the wire format used by the versions list/detail
// endpoints — they must stay in sync with the MCPServerVersion schema in
// server/api/openapi.yaml, and with the frontend VersionHistory component
// which reads these snake_case keys.
type MCPServerVersion struct {
	ID              string          `json:"id"`
	ServerID        string          `json:"server_id"`
	Version         string          `json:"version"` // semver
	Runtime         Runtime         `json:"runtime"`
	Packages        json.RawMessage `json:"packages,omitempty"`     // MCP packages array
	Capabilities    json.RawMessage `json:"capabilities,omitempty"` // MCP capabilities object (capability-negotiation flags)
	Tools           json.RawMessage `json:"tools,omitempty"`        // []MCPTool — publisher-declared tool list
	ProtocolVersion string          `json:"protocol_version"`
	Checksum        string          `json:"checksum,omitempty"`
	Signature       string          `json:"signature,omitempty"`
	Status          VersionStatus   `json:"status"` // active | deprecated | deleted
	StatusMessage   string          `json:"status_message,omitempty"`
	StatusChangedAt time.Time       `json:"status_changed_at"`
	PublishedAt     *time.Time      `json:"published_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// IsPublished reports whether the version has been published (immutable after this).
func (v *MCPServerVersion) IsPublished() bool {
	return v.PublishedAt != nil
}

// PackageEntry represents one entry in the MCP packages array.
// Used for structural validation only; stored as raw JSONB.
type PackageEntry struct {
	RegistryType    string    `json:"registryType"`
	RegistryBaseURL string    `json:"registryBaseUrl,omitempty"`
	Identifier      string    `json:"identifier"`
	Version         string    `json:"version"`
	Transport       Transport `json:"transport"`
}

// Transport holds the transport configuration for a package entry.
type Transport struct {
	Type string `json:"type"`
}

// slugRe matches valid registry slugs: lowercase alphanumeric and hyphens,
// 1-63 characters, not starting or ending with a hyphen.
var slugRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?$`)

// ValidateSlug checks that s is a valid registry slug (namespace or slug field).
// Rules: lowercase alphanumeric with hyphens, 1-63 characters,
// must start and end with an alphanumeric character.
func ValidateSlug(s string) error {
	if !slugRe.MatchString(s) {
		return fmt.Errorf("%q is not a valid slug (use lowercase letters, digits, and hyphens; max 63 chars)", s)
	}
	return nil
}

// serverNameRe matches valid MCP server names: namespace/slug.
// Spec pattern: ^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$
var serverNameRe = regexp.MustCompile(`^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$`)

// ValidateServerName checks that the given name matches the MCP registry spec
// pattern: ^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$
func ValidateServerName(name string) error {
	if !serverNameRe.MatchString(name) {
		return fmt.Errorf("name %q does not match required pattern ^[a-zA-Z0-9.-]+/[a-zA-Z0-9._-]+$", name)
	}
	return nil
}

// validRegistryTypes is the set of registry types allowed by the MCP spec.
var validRegistryTypes = map[string]bool{
	"npm": true, "pypi": true, "oci": true, "nuget": true, "mcpb": true,
}

// ValidatePackages checks that the packages JSONB is a non-empty array
// and that each entry has the required structural fields.
func ValidatePackages(raw json.RawMessage) error {
	if len(raw) == 0 {
		return fmt.Errorf("packages must not be empty")
	}
	var entries []PackageEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("packages must be a JSON array: %w", err)
	}
	if len(entries) == 0 {
		return fmt.Errorf("packages must contain at least one entry")
	}
	validTransports := map[string]bool{
		"stdio": true, "http": true, "sse": true, "streamable-http": true, "streamable_http": true,
	}
	for i, e := range entries {
		if e.RegistryType == "" {
			return fmt.Errorf("packages[%d].registryType is required", i)
		}
		if !validRegistryTypes[e.RegistryType] {
			return fmt.Errorf("packages[%d].registryType %q is not valid (must be one of: npm, pypi, oci, nuget, mcpb)", i, e.RegistryType)
		}
		if e.Identifier == "" {
			return fmt.Errorf("packages[%d].identifier is required", i)
		}
		if e.Version == "" {
			return fmt.Errorf("packages[%d].version is required", i)
		}
		if e.Version == "latest" {
			return fmt.Errorf("packages[%d].version must not be 'latest'; use an explicit version string", i)
		}
		if e.Transport.Type == "" {
			return fmt.Errorf("packages[%d].transport.type is required", i)
		}
		if !validTransports[e.Transport.Type] {
			return fmt.Errorf("packages[%d].transport.type %q is not valid (must be stdio, http, sse, or streamable-http)", i, e.Transport.Type)
		}
	}
	return nil
}

// ValidateCapabilities checks that the capabilities value is valid JSON.
func ValidateCapabilities(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil // empty is allowed; defaults to {}
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return fmt.Errorf("capabilities must be valid JSON: %w", err)
	}
	return nil
}

// MCPTool mirrors the MCP spec Tool object shape, restricted to the fields
// a publisher can sensibly declare at registration time.
//
// NOTE: the MCP spec's runtime `tools/list` response returns tools with this
// same shape. The registry stores a publisher-declared copy so the UI can
// show a tool count and descriptions without needing a live MCP connection.
type MCPTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"` // JSON Schema object
	Annotations json.RawMessage `json:"annotations,omitempty"`  // MCP spec tool annotations
}

// ValidateTools checks that the tools JSONB is a valid array and that each
// entry satisfies the structural requirements (non-empty unique name,
// input_schema is a JSON object if present).
//
// Unlike ValidateSkills, an empty array is allowed: a server legitimately
// may not have declared its tools yet, or may expose them only at runtime.
func ValidateTools(raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil // default to []
	}
	var tools []MCPTool
	if err := json.Unmarshal(raw, &tools); err != nil {
		return fmt.Errorf("tools must be a JSON array: %w", err)
	}
	seen := make(map[string]struct{}, len(tools))
	for i, t := range tools {
		if t.Name == "" {
			return fmt.Errorf("tools[%d].name is required", i)
		}
		if _, dup := seen[t.Name]; dup {
			return fmt.Errorf("tools[%d].name %q is duplicated within the array", i, t.Name)
		}
		seen[t.Name] = struct{}{}
		if len(t.InputSchema) > 0 && string(t.InputSchema) != "null" {
			// JSON Schema is always an object at the top level — reject
			// arrays, strings, numbers, etc. to catch obvious mistakes early.
			var probe map[string]any
			if err := json.Unmarshal(t.InputSchema, &probe); err != nil {
				return fmt.Errorf("tools[%d].input_schema must be a JSON object: %w", i, err)
			}
		}
	}
	return nil
}
