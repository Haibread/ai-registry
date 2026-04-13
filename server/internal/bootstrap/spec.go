// Package bootstrap loads an initial set of registry entries from a YAML or
// JSON file and upserts them into the database. It is idempotent: running it
// multiple times produces the same state.
package bootstrap

// Spec is the top-level structure of a bootstrap file.
type Spec struct {
	Publishers []PublisherSpec  `yaml:"publishers" json:"publishers"`
	MCPServers []MCPServerSpec  `yaml:"mcp_servers" json:"mcp_servers"`
	Agents     []AgentSpec      `yaml:"agents"      json:"agents"`
}

// PublisherSpec describes a publisher entry.
type PublisherSpec struct {
	Slug     string `yaml:"slug"     json:"slug"`
	Name     string `yaml:"name"     json:"name"`
	Verified bool   `yaml:"verified" json:"verified"`
}

// MCPServerSpec describes an MCP server and all its versions.
type MCPServerSpec struct {
	// Publisher is the slug of the publisher that owns this server.
	Publisher   string `yaml:"publisher"      json:"publisher"`
	Slug        string `yaml:"slug"           json:"slug"`
	Name        string `yaml:"name"           json:"name"`
	Description string `yaml:"description"    json:"description"`
	HomepageURL string `yaml:"homepage_url"   json:"homepage_url"`
	RepoURL     string `yaml:"repository_url" json:"repository_url"`
	License     string `yaml:"license"        json:"license"`
	// Public controls whether the server is visible to unauthenticated users.
	Public bool `yaml:"public" json:"public"`
	// Status of the server itself: "draft" | "published" | "deprecated".
	// When omitted the server status is derived from its versions.
	Status string `yaml:"status" json:"status"`
	// Featured flags a server for the home page "featured" carousel.
	Featured bool `yaml:"featured" json:"featured"`
	// Verified marks the server as officially vetted by the registry.
	Verified bool `yaml:"verified" json:"verified"`
	// Tags are free-form category labels surfaced in listing filters.
	Tags []string `yaml:"tags" json:"tags"`
	// Readme is long-form markdown content rendered on the detail page.
	Readme   string           `yaml:"readme"   json:"readme"`
	Versions []MCPVersionSpec `yaml:"versions" json:"versions"`
}

// MCPVersionSpec describes a single version of an MCP server.
type MCPVersionSpec struct {
	Version string `yaml:"version" json:"version"`
	// Status: "draft" | "published" | "deprecated". Defaults to "draft".
	Status          string        `yaml:"status"           json:"status"`
	StatusMessage   string        `yaml:"status_message"   json:"status_message"`
	ProtocolVersion string        `yaml:"protocol_version" json:"protocol_version"`
	Packages        []PackageSpec `yaml:"packages"         json:"packages"`
	// Capabilities is the free-form MCP capabilities object
	// (tools / resources / prompts / logging / …). Stored as JSONB.
	Capabilities map[string]any `yaml:"capabilities" json:"capabilities"`
}

// PackageSpec describes a single distribution package of an MCP server.
// YAML uses snake_case keys; JSON marshalling uses camelCase to match the
// MCP registry spec wire format stored in the database.
type PackageSpec struct {
	RegistryType string        `yaml:"registry_type" json:"registryType"`
	Identifier   string        `yaml:"identifier"    json:"identifier"`
	Version      string        `yaml:"version"       json:"version"`
	Transport    TransportSpec `yaml:"transport"     json:"transport"`
}

// TransportSpec describes how clients connect to an MCP server package.
type TransportSpec struct {
	Type string `yaml:"type"            json:"type"`
	URL  string `yaml:"url,omitempty"   json:"url,omitempty"`
}

// AgentSpec describes an agent and all its versions.
type AgentSpec struct {
	// Publisher is the slug of the publisher that owns this agent.
	Publisher   string `yaml:"publisher"   json:"publisher"`
	Slug        string `yaml:"slug"        json:"slug"`
	Name        string `yaml:"name"        json:"name"`
	Description string `yaml:"description" json:"description"`
	// Public controls whether the agent is visible to unauthenticated users.
	Public bool `yaml:"public" json:"public"`
	// Status of the agent itself: "draft" | "published" | "deprecated".
	Status string `yaml:"status" json:"status"`
	// Featured flags an agent for the home page "featured" carousel.
	Featured bool `yaml:"featured" json:"featured"`
	// Verified marks the agent as officially vetted by the registry.
	Verified bool `yaml:"verified" json:"verified"`
	// Tags are free-form category labels surfaced in listing filters.
	Tags []string `yaml:"tags" json:"tags"`
	// Readme is long-form markdown content rendered on the detail page.
	Readme   string             `yaml:"readme"   json:"readme"`
	Versions []AgentVersionSpec `yaml:"versions" json:"versions"`
}

// AgentVersionSpec describes a single version of an agent.
type AgentVersionSpec struct {
	Version            string      `yaml:"version"              json:"version"`
	// Status: "draft" | "published" | "deprecated". Defaults to "draft".
	Status             string      `yaml:"status"               json:"status"`
	StatusMessage      string      `yaml:"status_message"       json:"status_message"`
	EndpointURL        string      `yaml:"endpoint_url"         json:"endpoint_url"`
	ProtocolVersion    string      `yaml:"protocol_version"     json:"protocol_version"`
	DefaultInputModes  []string    `yaml:"default_input_modes"  json:"default_input_modes"`
	DefaultOutputModes []string    `yaml:"default_output_modes" json:"default_output_modes"`
	DocumentationURL   string      `yaml:"documentation_url"    json:"documentation_url"`
	IconURL            string      `yaml:"icon_url"             json:"icon_url"`
	Skills             []SkillSpec `yaml:"skills"               json:"skills"`
	Authentication     []AuthSpec  `yaml:"authentication"       json:"authentication"`
}

// SkillSpec describes a single skill exposed by an agent version.
type SkillSpec struct {
	ID          string   `yaml:"id"          json:"id"`
	Name        string   `yaml:"name"        json:"name"`
	Description string   `yaml:"description" json:"description"`
	Tags        []string `yaml:"tags"        json:"tags"`
}

// AuthSpec describes an authentication scheme accepted by an agent version.
type AuthSpec struct {
	Scheme string `yaml:"scheme" json:"scheme"`
}
