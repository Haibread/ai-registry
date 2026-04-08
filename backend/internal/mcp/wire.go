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

// ListResponse is the wire format for GET /v0/servers and GET /v0/servers/{ns}/{slug}/versions.
type ListResponse struct {
	Servers  []ServerResponse `json:"servers"`
	Metadata ListMetadata     `json:"metadata"`
}

// ListMetadata holds pagination info for the list response.
type ListMetadata struct {
	Count      int    `json:"count"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ---- ServerResponse (canonical top-level shape) -------------------------

// ServerResponse is the canonical API response wrapping a server version.
// This is both the list item shape and the detail/publish response shape.
type ServerResponse struct {
	Server ServerDetail `json:"server"`
	Meta   ResponseMeta `json:"_meta"`
}

// ResponseMeta holds registry-managed metadata at the response level.
type ResponseMeta struct {
	Official OfficialMeta `json:"io.modelcontextprotocol.registry/official"`
}

// OfficialMeta is our registry's metadata block.
type OfficialMeta struct {
	Status          string     `json:"status"`
	StatusMessage   string     `json:"statusMessage,omitempty"`
	PublishedAt     *time.Time `json:"publishedAt,omitempty"`
	UpdatedAt       time.Time  `json:"updatedAt,omitempty"`
	StatusChangedAt time.Time  `json:"statusChangedAt,omitempty"`
	IsLatest        bool       `json:"isLatest"`
}

// ServerDetail is the server.json shape (publisher data only, no registry metadata).
type ServerDetail struct {
	ID           string          `json:"id,omitempty"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Title        string          `json:"title,omitempty"`
	Version      string          `json:"version,omitempty"`
	Packages     json.RawMessage `json:"packages,omitempty"`
	Capabilities json.RawMessage `json:"capabilities,omitempty"`
	Repository   *Repository     `json:"repository,omitempty"`
	WebsiteURL   string          `json:"websiteUrl,omitempty"`
}

// Repository mirrors the MCP server.json repository object.
type Repository struct {
	URL       string `json:"url"`
	Source    string `json:"source,omitempty"`
	Subfolder string `json:"subfolder,omitempty"`
	ID        string `json:"id,omitempty"`
}

// ---- Publish request/response --------------------------------------------

// PublishRequest is the body accepted by POST /v0/publish.
// Per spec, the body IS the ServerDetail directly (no wrapper object).
type PublishRequest struct {
	Name            string          `json:"name"`
	Description     string          `json:"description"`
	Version         string          `json:"version"`
	Title           string          `json:"title,omitempty"`
	Packages        json.RawMessage `json:"packages"`
	Capabilities    json.RawMessage `json:"capabilities,omitempty"`
	ProtocolVersion string          `json:"protocolVersion"`
	Repository      *Repository     `json:"repository,omitempty"`
	WebsiteURL      string          `json:"websiteUrl,omitempty"`
}

// ---- Status PATCH --------------------------------------------------------

// StatusPatchRequest is the body for PATCH status endpoints.
type StatusPatchRequest struct {
	Status        string `json:"status"`
	StatusMessage string `json:"statusMessage,omitempty"`
}

// ---- AllVersionsStatusResponse -------------------------------------------

// AllVersionsStatusResponse is returned by PATCH /v0/servers/{name}/status.
type AllVersionsStatusResponse struct {
	UpdatedCount int              `json:"updatedCount"`
	Servers      []ServerResponse `json:"servers"`
}

// ---- Conversion helpers --------------------------------------------------

// mapServerStatus converts our internal domain status to the MCP spec status enum.
// Spec enum: active | deprecated | deleted | draft
func mapServerStatus(s domain.Status) string {
	switch s {
	case domain.StatusPublished:
		return "active"
	case domain.StatusDeprecated:
		return "deprecated"
	case domain.StatusDeleted:
		return "deleted"
	default:
		return string(s)
	}
}

// mapVersionStatus converts a VersionStatus to spec wire format.
func mapVersionStatus(s domain.VersionStatus) string {
	switch s {
	case domain.VersionStatusActive:
		return "active"
	case domain.VersionStatusDeprecated:
		return "deprecated"
	case domain.VersionStatusDeleted:
		return "deleted"
	default:
		if string(s) == "" {
			return "active"
		}
		return string(s)
	}
}

// ToServerResponse converts an MCPServerRow and its version into the canonical ServerResponse shape.
func ToServerResponse(srv store.MCPServerRow, ver *domain.MCPServerVersion, isLatest bool) ServerResponse {
	detail := ServerDetail{
		ID:          srv.ID,
		Name:        srv.Namespace + "/" + srv.Slug,
		Description: srv.Description,
		WebsiteURL:  srv.HomepageURL,
	}
	if srv.RepoURL != "" {
		detail.Repository = repoFromURL(srv.RepoURL)
	}

	official := OfficialMeta{
		Status:    mapServerStatus(srv.Status),
		UpdatedAt: srv.UpdatedAt,
		IsLatest:  isLatest,
	}

	if ver != nil {
		detail.Version = ver.Version
		detail.Packages = ver.Packages
		detail.Capabilities = ver.Capabilities
		if ver.PublishedAt != nil {
			official.PublishedAt = ver.PublishedAt
		}
		official.StatusMessage = ver.StatusMessage
		if !ver.StatusChangedAt.IsZero() {
			official.StatusChangedAt = ver.StatusChangedAt
		}
		// Use the version-level status if available.
		if ver.Status != "" {
			official.Status = mapVersionStatus(ver.Status)
		}
	}

	return ServerResponse{
		Server: detail,
		Meta: ResponseMeta{
			Official: official,
		},
	}
}

// ToServerResponseFromVersion converts a version domain object + server row into a ServerResponse.
// Used when returning a specific version (status comes from version, not server).
func ToServerResponseFromVersion(srv store.MCPServerRow, ver *domain.MCPServerVersion, isLatest bool) ServerResponse {
	return ToServerResponse(srv, ver, isLatest)
}

func repoFromURL(u string) *Repository {
	r := &Repository{URL: u}
	switch {
	case len(u) > 19 && u[:19] == "https://github.com/":
		r.Source = "github"
	case len(u) > 19 && u[:19] == "https://gitlab.com/":
		r.Source = "gitlab"
	}
	return r
}
