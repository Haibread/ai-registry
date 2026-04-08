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

// Status represents the lifecycle state of a registry entry.
type Status string

const (
	StatusDraft      Status = "draft"
	StatusPublished  Status = "published"
	StatusDeprecated Status = "deprecated"
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
	Status      Status
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
type MCPServerVersion struct {
	ID              string
	ServerID        string
	Version         string // semver
	Runtime         Runtime
	Packages        json.RawMessage // MCP packages array
	Capabilities    json.RawMessage // MCP capabilities object
	ProtocolVersion string
	Checksum        string
	Signature       string
	Status          VersionStatus // active | deprecated | deleted
	PublishedAt     *time.Time
	ReleasedAt      time.Time
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
