package mcp_test

import (
	"testing"
	"time"

	"github.com/haibread/ai-registry/internal/domain"
	mcpwire "github.com/haibread/ai-registry/internal/mcp"
	"github.com/haibread/ai-registry/internal/store"
)

func TestToServerEntry_WithVersion(t *testing.T) {
	now := time.Now().UTC()
	srv := store.MCPServerRow{
		MCPServer: domain.MCPServer{
			ID:          "01ABCDEF",
			Slug:        "my-server",
			Visibility:  domain.VisibilityPublic,
			Status:      domain.StatusPublished,
			RepoURL:     "https://github.com/acme/my-server",
		},
	}
	srv.Namespace = "acme"

	ver := &domain.MCPServerVersion{
		Version:     "1.2.3",
		PublishedAt: &now,
	}

	entry := mcpwire.ToServerEntry(srv, ver, true)

	if entry.Name != "acme/my-server" {
		t.Errorf("Name = %q, want %q", entry.Name, "acme/my-server")
	}
	if entry.Version != "1.2.3" {
		t.Errorf("Version = %q, want %q", entry.Version, "1.2.3")
	}
	if entry.Repository == nil || entry.Repository.URL != "https://github.com/acme/my-server" {
		t.Error("expected repository URL to be set")
	}
	if entry.Repository.Source != "github" {
		t.Errorf("Source = %q, want github", entry.Repository.Source)
	}
	if !entry.Meta.Official.IsLatest {
		t.Error("expected IsLatest=true")
	}
}

func TestToServerEntry_WithoutVersion(t *testing.T) {
	srv := store.MCPServerRow{
		MCPServer: domain.MCPServer{
			ID:     "01ABCDEF",
			Slug:   "no-version",
			Status: domain.StatusDraft,
		},
	}
	srv.Namespace = "acme"

	entry := mcpwire.ToServerEntry(srv, nil, false)

	if entry.Version != "" {
		t.Errorf("Version = %q, want empty when no published version", entry.Version)
	}
}

func TestToServerDetail_RepoSourceInference(t *testing.T) {
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
		detail := mcpwire.ToServerDetail(srv, nil)
		if detail.Repository == nil {
			t.Errorf("url=%s: expected non-nil repository", tt.url)
			continue
		}
		if detail.Repository.Source != tt.source {
			t.Errorf("url=%s: Source = %q, want %q", tt.url, detail.Repository.Source, tt.source)
		}
	}
}
