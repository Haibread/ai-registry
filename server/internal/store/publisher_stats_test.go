package store_test

// Tests for the per-publisher admin-home queries: GetPublisherStats (counts,
// status breakdowns, members by role, pending-review) and the ListAuditEvents
// ResourceNS filter that backs the publisher activity feed.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

func TestGetPublisherStats(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubID := insertPublisher(t, "stats-pub", "Stats Pub")
	other := insertPublisher(t, "other-pub", "Other Pub")

	// stats-pub: one draft + one published MCP server; a server under the other
	// publisher proves the scoping.
	draftSrv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pubID, Slug: "draft-srv", Name: "Draft"})
	if err != nil {
		t.Fatalf("CreateMCPServer draft: %v", err)
	}
	pubSrv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: pubID, Slug: "pub-srv", Name: "Pub"})
	if err != nil {
		t.Fatalf("CreateMCPServer pub: %v", err)
	}
	if err := sharedDB.SetMCPServerStatus(ctx, pubSrv.ID, domain.StatusPublished); err != nil {
		t.Fatalf("SetMCPServerStatus: %v", err)
	}
	if _, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{PublisherID: other, Slug: "noise", Name: "Noise"}); err != nil {
		t.Fatalf("CreateMCPServer other: %v", err)
	}

	// One draft agent under stats-pub.
	if _, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pubID, Slug: "ag", Name: "Ag"}); err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// u1 dual-hats Editor + Reviewer; u2 is a Viewer. Distinct members = 2.
	u1, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "u1@stats.test"})
	if err != nil {
		t.Fatalf("CreateUser u1: %v", err)
	}
	u2, err := sharedDB.CreateUser(ctx, store.CreateUserParams{Email: "u2@stats.test"})
	if err != nil {
		t.Fatalf("CreateUser u2: %v", err)
	}
	for _, g := range []store.CreateGrantParams{
		{PrincipalType: domain.PrincipalUser, PrincipalID: u1.ID, PublisherID: pubID, Role: domain.RoleEditor},
		{PrincipalType: domain.PrincipalUser, PrincipalID: u1.ID, PublisherID: pubID, Role: domain.RoleReviewer},
		{PrincipalType: domain.PrincipalUser, PrincipalID: u2.ID, PublisherID: pubID, Role: domain.RoleViewer},
	} {
		if _, err := sharedDB.CreateGrant(ctx, g); err != nil {
			t.Fatalf("CreateGrant: %v", err)
		}
	}

	// A submitted version on the draft server → pending_review = 1.
	if _, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        draftSrv.ID,
		Version:         "1.0.0",
		Runtime:         domain.RuntimeStdio,
		Packages:        json.RawMessage(`[{"registryType":"npm","identifier":"@x/y","version":"1.0.0","transport":{"type":"stdio"}}]`),
		ProtocolVersion: "2024-11-05",
	}); err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}
	if err := sharedDB.SubmitMCPVersion(ctx, draftSrv.ID, "1.0.0", false, actor()); err != nil {
		t.Fatalf("SubmitMCPVersion: %v", err)
	}

	s, err := sharedDB.GetPublisherStats(ctx, pubID)
	if err != nil {
		t.Fatalf("GetPublisherStats: %v", err)
	}

	if s.MCPServers != 2 {
		t.Errorf("MCPServers = %d, want 2 (scoped to stats-pub)", s.MCPServers)
	}
	if s.MCPStatusBreakdown.Draft != 1 || s.MCPStatusBreakdown.Published != 1 {
		t.Errorf("mcp breakdown = %+v, want draft 1 / published 1", *s.MCPStatusBreakdown)
	}
	if s.Agents != 1 || s.AgentStatusBreakdown.Draft != 1 {
		t.Errorf("agents = %d breakdown = %+v, want 1 draft", s.Agents, *s.AgentStatusBreakdown)
	}
	if s.Members != 2 {
		t.Errorf("Members = %d, want 2 distinct principals", s.Members)
	}
	if s.MemberRoles.Editors != 1 || s.MemberRoles.Reviewers != 1 || s.MemberRoles.Viewers != 1 {
		t.Errorf("member_roles = %+v, want editors/reviewers/viewers each 1", *s.MemberRoles)
	}
	if s.PendingReview != 1 {
		t.Errorf("PendingReview = %d, want 1", s.PendingReview)
	}
}

func TestListAuditEvents_FiltersByResourceNS(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	for _, ns := range []string{"p1", "p1", "p2"} {
		sharedDB.LogAuditEvent(ctx, domain.AuditEvent{
			ActorSubject: "tester", ActorEmail: "t@x.test",
			Action: domain.ActionMCPServerCreated, ResourceType: "mcp_server",
			ResourceID: store.NewULID(), ResourceNS: ns, ResourceSlug: "srv",
		})
	}

	got, err := sharedDB.ListAuditEvents(ctx, store.ListAuditParams{ResourceNS: "p1", Limit: 50})
	if err != nil {
		t.Fatalf("ListAuditEvents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (only p1 events)", len(got))
	}
	for _, e := range got {
		if e.ResourceNS != "p1" {
			t.Errorf("resource_ns = %q, want p1", e.ResourceNS)
		}
	}
}
