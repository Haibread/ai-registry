// Package mcp translates internal domain objects into the strict MCP registry
// wire format defined at https://github.com/modelcontextprotocol/registry.
package mcp

import (
	"encoding/json"
	"time"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// ---- List response -------------------------------------------------------

// ListResponse is the wire format for GET /v0/servers.
type ListResponse struct {
	Servers  []ServerEntry `json:"servers"`
	Metadata ListMetadata  `json:"metadata"`
}

// ListMetadata holds pagination info for the list response.
type ListMetadata struct {
	Count      int    `json:"count"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ServerEntry is one item in the servers array.
type ServerEntry struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`        // "{namespace}/{slug}"
	Description string          `json:"description"`
	Version     string          `json:"version,omitempty"`
	Packages    json.RawMessage `json:"packages,omitempty"`
	Repository  *Repository     `json:"repository,omitempty"`
	Meta        ServerMeta      `json:"_meta"`
}

// Repository mirrors the MCP server.json repository object.
type Repository struct {
	URL    string `json:"url"`
	Source string `json:"source,omitempty"` // e.g. "github"
}

// ServerMeta holds registry-level metadata under our namespace key.
type ServerMeta struct {
	Official OfficialMeta `json:"io.modelcontextprotocol.registry/official"`
}

// OfficialMeta is our registry's metadata block inside _meta.
type OfficialMeta struct {
	Status        string    `json:"status"`
	PublishedAt   time.Time `json:"publishedAt"`
	UpdatedAt     time.Time `json:"updatedAt,omitempty"`
	StatusMessage string    `json:"statusMessage,omitempty"`
	IsLatest      bool      `json:"isLatest"`
}

// ---- Detail response -----------------------------------------------------

// DetailResponse is the wire format for GET /v0/servers/{id}.
type DetailResponse struct {
	Server ServerDetail `json:"server"`
}

// ServerDetail is the full server object returned by the detail endpoint.
type ServerDetail struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Version      string          `json:"version,omitempty"`
	Packages     json.RawMessage `json:"packages,omitempty"`
	Capabilities json.RawMessage `json:"capabilities,omitempty"`
	Repository   *Repository     `json:"repository,omitempty"`
	WebsiteURL   string          `json:"websiteUrl,omitempty"`
	Meta         ServerMeta      `json:"_meta"`
}

// ---- Publish request/response --------------------------------------------

// PublishRequest is the body accepted by POST /v0/publish.
// Per spec, the body IS the ServerDetail directly (no wrapper object).
type PublishRequest struct {
	Name            string          `json:"name"`        // "{namespace}/{slug}"
	Description     string          `json:"description"`
	Version         string          `json:"version"`
	Packages        json.RawMessage `json:"packages"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty"`
	ProtocolVersion string          `json:"protocolVersion"`
	Repository      *Repository     `json:"repository,omitempty"`
}

// ServerResponse is the publish response body: { server: ServerDetail }.
// Matches the spec's ServerResponse shape.
type ServerResponse struct {
	Server ServerDetail `json:"server"`
}

// ---- Version list/detail -------------------------------------------------

// VersionEntry is one item in a versions list.
type VersionEntry struct {
	Version     string          `json:"version"`
	Packages    json.RawMessage `json:"packages,omitempty"`
	PublishedAt *time.Time      `json:"publishedAt,omitempty"`
	Status      string          `json:"status"`
}

// VersionListResponse is the wire format for GET /v0/servers/{name}/versions.
type VersionListResponse struct {
	Versions []VersionEntry `json:"versions"`
}

// ---- Status PATCH --------------------------------------------------------

// StatusPatchRequest is the body for PATCH status endpoints.
type StatusPatchRequest struct {
	Status string `json:"status"` // active | deprecated | deleted
}

// ---- Conversion helpers --------------------------------------------------

// mapServerStatus converts our internal domain status to the MCP spec status enum.
// Spec enum: active | deprecated | deleted
func mapServerStatus(s domain.Status) string {
	switch s {
	case domain.StatusPublished:
		return "active"
	case domain.StatusDeprecated:
		return "deprecated"
	default:
		// draft maps to "draft" — not in the spec enum but we expose it for completeness
		return string(s)
	}
}

// ToServerEntry converts an MCPServerRow and its latest published version
// into an MCP wire-format ServerEntry.
func ToServerEntry(srv store.MCPServerRow, ver *domain.MCPServerVersion, isLatest bool) ServerEntry {
	e := ServerEntry{
		ID:          srv.ID,
		Name:        srv.Namespace + "/" + srv.Slug,
		Description: srv.Description,
		Meta: ServerMeta{
			Official: OfficialMeta{
				Status:    mapServerStatus(srv.Status),
				UpdatedAt: srv.UpdatedAt,
				IsLatest:  isLatest,
			},
		},
	}
	if ver != nil {
		e.Version = ver.Version
		e.Packages = ver.Packages
		e.Meta.Official.PublishedAt = *ver.PublishedAt
	}
	if srv.RepoURL != "" {
		e.Repository = repoFromURL(srv.RepoURL)
	}
	return e
}

// ToServerDetail converts an MCPServerRow and a version into a full detail object.
func ToServerDetail(srv store.MCPServerRow, ver *domain.MCPServerVersion) ServerDetail {
	d := ServerDetail{
		ID:          srv.ID,
		Name:        srv.Namespace + "/" + srv.Slug,
		Description: srv.Description,
		WebsiteURL:  srv.HomepageURL,
		Meta: ServerMeta{
			Official: OfficialMeta{
				Status:    mapServerStatus(srv.Status),
				UpdatedAt: srv.UpdatedAt,
				IsLatest:  true,
			},
		},
	}
	if ver != nil {
		d.Version = ver.Version
		d.Packages = ver.Packages
		d.Capabilities = ver.Capabilities
		if ver.PublishedAt != nil {
			d.Meta.Official.PublishedAt = *ver.PublishedAt
		}
	}
	if srv.RepoURL != "" {
		d.Repository = repoFromURL(srv.RepoURL)
	}
	return d
}

// ToVersionEntry converts a domain MCPServerVersion into a wire-format VersionEntry.
func ToVersionEntry(ver domain.MCPServerVersion) VersionEntry {
	status := string(ver.Status)
	if status == "" {
		status = "active"
	}
	return VersionEntry{
		Version:     ver.Version,
		Packages:    ver.Packages,
		PublishedAt: ver.PublishedAt,
		Status:      status,
	}
}

func repoFromURL(u string) *Repository {
	r := &Repository{URL: u}
	// Infer source from URL for common hosts.
	switch {
	case len(u) > 19 && u[:19] == "https://github.com/":
		r.Source = "github"
	case len(u) > 19 && u[:19] == "https://gitlab.com/":
		r.Source = "gitlab"
	}
	return r
}
