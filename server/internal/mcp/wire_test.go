package mcp_test

import (
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/domain"
	mcpwire "github.com/haibread/ai-registry/internal/mcp"
	"github.com/haibread/ai-registry/internal/store"
)

func TestToServerResponse_WithVersion(t *testing.T) {
	now := time.Now().UTC()
	srv := store.MCPServerRow{
		MCPServer: domain.MCPServer{
			ID:         "01ABCDEF",
			Slug:       "my-server",
			Visibility: domain.VisibilityPublic,
			Status:     domain.StatusPublished,
			RepoURL:    "https://github.com/acme/my-server",
		},
	}
	srv.Namespace = "acme"

	ver := &domain.MCPServerVersion{
		Version:     "1.2.3",
		Status:      domain.VersionStatusActive,
		PublishedAt: &now,
	}

	resp := mcpwire.ToServerResponse(srv, ver, true)

	if resp.Server.Name != "acme/my-server" {
		t.Errorf("Name = %q, want %q", resp.Server.Name, "acme/my-server")
	}
	if resp.Server.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", resp.Server.Version, "1.2.3")
	}
	if resp.Server.Repository == nil || resp.Server.Repository.URL != "https://github.com/acme/my-server" {
		t.Error("expected repository URL to be set")
	}
	if resp.Server.Repository.Source != "github" {
		t.Errorf("Source = %q, want github", resp.Server.Repository.Source)
	}
	if !resp.Meta.Official.IsLatest {
		t.Error("expected IsLatest=true")
	}
	if resp.Meta.Official.Status != "active" {
		t.Errorf("Status = %q, want active", resp.Meta.Official.Status)
	}
}

func TestToServerResponse_WithoutVersion(t *testing.T) {
	srv := store.MCPServerRow{
		MCPServer: domain.MCPServer{
			ID:     "01ABCDEF",
			Slug:   "no-version",
			Status: domain.StatusDraft,
		},
	}
	srv.Namespace = "acme"

	resp := mcpwire.ToServerResponse(srv, nil, false)

	if resp.Server.Version != "" {
		t.Errorf("Version = %q, want empty when no published version", resp.Server.Version)
	}
	if resp.Meta.Official.IsLatest {
		t.Error("expected IsLatest=false")
	}
}

func TestToServerResponse_MetaAtTopLevel(t *testing.T) {
	srv := store.MCPServerRow{
		MCPServer: domain.MCPServer{
			ID:     "01ABCDEF",
			Slug:   "test-srv",
			Status: domain.StatusPublished,
		},
	}
	srv.Namespace = "ns"

	resp := mcpwire.ToServerResponse(srv, nil, true)

	// _meta must be at the ServerResponse level, not inside ServerDetail
	if resp.Meta.Official.Status == "" {
		t.Error("_meta.official.status must not be empty")
	}
}

func TestToServerResponse_RepoSourceInference(t *testing.T) {
	tests := []struct {
		url    string
		source string
	}{
		{"https://github.com/org/repo", "github"},
		{"https://gitlab.com/org/repo", "gitlab"},
		{"https://bitbucket.org/org/repo", ""},
	}

	for _, tt := range tests {
		srv := store.MCPServerRow{MCPServer: domain.MCPServer{RepoURL: tt.url}}
		srv.Namespace = "ns"
		resp := mcpwire.ToServerResponse(srv, nil, true)
		if resp.Server.Repository == nil {
			t.Errorf("url=%s: expected non-nil repository", tt.url)
			continue
		}
		if resp.Server.Repository.Source != tt.source {
			t.Errorf("url=%s: Source = %q, want %q", tt.url, resp.Server.Repository.Source, tt.source)
		}
	}
}

func TestMapServerStatus(t *testing.T) {
	tests := []struct {
		srv     store.MCPServerRow
		want    string
	}{
		{store.MCPServerRow{MCPServer: domain.MCPServer{Status: domain.StatusPublished}}, "active"},
		{store.MCPServerRow{MCPServer: domain.MCPServer{Status: domain.StatusDeprecated}}, "deprecated"},
		{store.MCPServerRow{MCPServer: domain.MCPServer{Status: domain.StatusDeleted}}, "deleted"},
		{store.MCPServerRow{MCPServer: domain.MCPServer{Status: domain.StatusDraft}}, "draft"},
	}

	for _, tt := range tests {
		tt.srv.Namespace = "ns"
		resp := mcpwire.ToServerResponse(tt.srv, nil, false)
		// When no version is given, status comes from the server-level status
		if resp.Meta.Official.Status != tt.want {
			t.Errorf("status(%q) = %q, want %q", tt.srv.Status, resp.Meta.Official.Status, tt.want)
		}
	}
}

func TestToServerResponse_StatusMessageAndChangedAt(t *testing.T) {
	now := time.Now().UTC()
	srv := store.MCPServerRow{
		MCPServer: domain.MCPServer{
			ID:     "01ABCDEF",
			Slug:   "test-srv",
			Status: domain.StatusDeprecated,
		},
	}
	srv.Namespace = "ns"

	ver := &domain.MCPServerVersion{
		Version:         "1.0.0",
		Status:          domain.VersionStatusDeprecated,
		StatusMessage:   "Use v2 instead",
		StatusChangedAt: now,
		PublishedAt:     &now,
	}

	resp := mcpwire.ToServerResponse(srv, ver, false)

	if resp.Meta.Official.StatusMessage != "Use v2 instead" {
		t.Errorf("StatusMessage = %q, want 'Use v2 instead'", resp.Meta.Official.StatusMessage)
	}
	if resp.Meta.Official.StatusChangedAt.IsZero() {
		t.Error("StatusChangedAt must be set when version has a changed-at time")
	}
}
