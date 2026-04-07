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
	Status      string    `json:"status"`
	PublishedAt time.Time `json:"publishedAt"`
	IsLatest    bool      `json:"isLatest"`
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

// ---- Publish request ------------------------------------------------------

// PublishRequest is the body accepted by POST /v0/publish.
type PublishRequest struct {
	Server PublishServerPayload `json:"server"`
}

// PublishServerPayload holds the fields required to create/update a server version.
type PublishServerPayload struct {
	Name            string          `json:"name"`        // "{namespace}/{slug}"
	Description     string          `json:"description"`
	Version         string          `json:"version"`
	Packages        json.RawMessage `json:"packages"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty"`
	ProtocolVersion string          `json:"protocolVersion"`
	Repository      *Repository     `json:"repository,omitempty"`
}

// ---- Conversion helpers --------------------------------------------------

// ToServerEntry converts an MCPServerRow and its latest published version
// into an MCP wire-format ServerEntry.
func ToServerEntry(srv store.MCPServerRow, ver *domain.MCPServerVersion, isLatest bool) ServerEntry {
	e := ServerEntry{
		ID:          srv.ID,
		Name:        srv.Namespace + "/" + srv.Slug,
		Description: srv.Description,
		Meta: ServerMeta{
			Official: OfficialMeta{
				Status:   string(srv.Status),
				IsLatest: isLatest,
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
				Status:   string(srv.Status),
				IsLatest: true,
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
