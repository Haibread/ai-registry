package store_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// insertTag is a test helper that defines an instance tag via the store API.
func insertTag(t *testing.T, slug, name, color string) *domain.InstanceTag {
	t.Helper()
	tag, err := sharedDB.CreateInstanceTag(context.Background(), store.CreateInstanceTagParams{
		Slug: slug, Name: name, Color: color,
	})
	if err != nil {
		t.Fatalf("CreateInstanceTag(%s): %v", slug, err)
	}
	return tag
}

func TestInstanceTagCRUD(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	created := insertTag(t, "early-access", "Early Access", "yellow")
	if !created.Active {
		t.Error("new tag should be active by default")
	}

	// Duplicate slug conflicts.
	if _, err := sharedDB.CreateInstanceTag(ctx, store.CreateInstanceTagParams{
		Slug: "early-access", Name: "Other",
	}); err != store.ErrConflict {
		t.Errorf("expected ErrConflict on duplicate slug, got %v", err)
	}

	// List returns the full vocabulary ordered by name.
	insertTag(t, "free", "Free", "green")
	tags, err := sharedDB.ListInstanceTags(ctx)
	if err != nil {
		t.Fatalf("ListInstanceTags: %v", err)
	}
	if len(tags) != 2 || tags[0].Slug != "early-access" || tags[1].Slug != "free" {
		t.Errorf("unexpected listing: %+v", tags)
	}

	// Partial update: display fields change, slug stays.
	newName := "Beta"
	inactive := false
	updated, err := sharedDB.UpdateInstanceTag(ctx, "early-access", store.UpdateInstanceTagParams{
		Name: &newName, Active: &inactive,
	})
	if err != nil {
		t.Fatalf("UpdateInstanceTag: %v", err)
	}
	if updated.Name != "Beta" || updated.Active || updated.Color != "yellow" {
		t.Errorf("unexpected updated tag: %+v", updated)
	}

	// Deactivated tags stay listed (display resolution for frozen versions).
	tags, err = sharedDB.ListInstanceTags(ctx)
	if err != nil {
		t.Fatalf("ListInstanceTags after deactivate: %v", err)
	}
	if len(tags) != 2 {
		t.Errorf("deactivated tag must remain in the listing, got %d tags", len(tags))
	}

	// Update of a missing tag is ErrNotFound.
	if _, err := sharedDB.UpdateInstanceTag(ctx, "nope", store.UpdateInstanceTagParams{Name: &newName}); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	// Unused tags hard-delete; missing ones are ErrNotFound.
	if err := sharedDB.DeleteInstanceTag(ctx, "free"); err != nil {
		t.Fatalf("DeleteInstanceTag(unused): %v", err)
	}
	if err := sharedDB.DeleteInstanceTag(ctx, "free"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound on second delete, got %v", err)
	}
}

func TestDeleteInstanceTag_InUseConflicts(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "tagged", "Tagged Corp")
	insertTag(t, "free", "Free", "green")
	insertTag(t, "beta", "Beta", "orange")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "srv", Name: "Server",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}
	if _, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "1.0.0", Runtime: domain.RuntimeStdio,
		ProtocolVersion: "2024-11-05", Tags: []string{"free"},
	}); err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}

	// Carried by an MCP version (even a draft) → conflict.
	if err := sharedDB.DeleteInstanceTag(ctx, "free"); err != store.ErrConflict {
		t.Errorf("expected ErrConflict for in-use tag, got %v", err)
	}

	// Carried by an agent version → conflict too.
	agent, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "bot", Name: "Bot",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if _, err := sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID: agent.ID, Version: "1.0.0", EndpointURL: "https://bot.example.com",
		ProtocolVersion: "0.2.1", Tags: []string{"beta"},
	}); err != nil {
		t.Fatalf("CreateAgentVersion: %v", err)
	}
	if err := sharedDB.DeleteInstanceTag(ctx, "beta"); err != store.ErrConflict {
		t.Errorf("expected ErrConflict for agent-carried tag, got %v", err)
	}
}

func TestMissingActiveTagSlugs(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	insertTag(t, "free", "Free", "green")
	deactivated := insertTag(t, "old", "Old", "gray")
	inactive := false
	if _, err := sharedDB.UpdateInstanceTag(ctx, deactivated.Slug, store.UpdateInstanceTagParams{Active: &inactive}); err != nil {
		t.Fatalf("deactivating: %v", err)
	}

	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{"empty input", nil, nil},
		{"all known", []string{"free"}, nil},
		{"unknown slug", []string{"free", "nope"}, []string{"nope"}},
		{"deactivated counts as missing", []string{"old"}, []string{"old"}},
		{"order preserved", []string{"zzz", "aaa"}, []string{"zzz", "aaa"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sharedDB.MissingActiveTagSlugs(ctx, tt.in)
			if err != nil {
				t.Fatalf("MissingActiveTagSlugs: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MissingActiveTagSlugs(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestVersionTags_FlowThroughReads covers the read paths: version rows carry
// their tags, and the entry-level tags / `tag` filter reflect the LATEST
// PUBLISHED version, not drafts and not older versions.
func TestVersionTags_FlowThroughReads(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")
	insertTag(t, "free", "Free", "green")
	insertTag(t, "early-access", "Early Access", "yellow")

	srv, err := sharedDB.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: pubID, Slug: "srv", Name: "Server",
	})
	if err != nil {
		t.Fatalf("CreateMCPServer: %v", err)
	}

	v1, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "1.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, ProtocolVersion: "2024-11-05",
		Tags: []string{"early-access", "free"},
	})
	if err != nil {
		t.Fatalf("CreateMCPServerVersion: %v", err)
	}
	if !reflect.DeepEqual(v1.Tags, []string{"early-access", "free"}) {
		t.Errorf("created version tags = %v", v1.Tags)
	}

	// Round-trip through the version read.
	got, err := sharedDB.GetMCPServerVersion(ctx, srv.ID, "1.0.0", false)
	if err != nil {
		t.Fatalf("GetMCPServerVersion: %v", err)
	}
	if !reflect.DeepEqual(got.Tags, []string{"early-access", "free"}) {
		t.Errorf("read-back version tags = %v", got.Tags)
	}

	// Entry-level tags are empty while nothing is published.
	row, err := sharedDB.GetMCPServer(ctx, "acme", "srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer: %v", err)
	}
	if len(row.Tags) != 0 {
		t.Errorf("entry tags before publish = %v, want empty", row.Tags)
	}

	// Publish v1 → entry tags become v1's.
	if err := sharedDB.PublishMCPServerVersion(ctx, srv.ID, "1.0.0"); err != nil {
		t.Fatalf("PublishMCPServerVersion: %v", err)
	}
	row, err = sharedDB.GetMCPServer(ctx, "acme", "srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer after publish: %v", err)
	}
	if !reflect.DeepEqual(row.Tags, []string{"early-access", "free"}) {
		t.Errorf("entry tags after publish = %v", row.Tags)
	}
	if row.LatestVersion == nil || !reflect.DeepEqual(row.LatestVersion.Tags, []string{"early-access", "free"}) {
		t.Errorf("latest_version tags = %+v", row.LatestVersion)
	}

	// Publish v2 without "early-access" → entry now reflects v2 only.
	if _, err := sharedDB.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID: srv.ID, Version: "2.0.0", Runtime: domain.RuntimeStdio,
		Packages: validPackages, ProtocolVersion: "2024-11-05",
		Tags: []string{"free"},
	}); err != nil {
		t.Fatalf("CreateMCPServerVersion v2: %v", err)
	}
	if err := sharedDB.PublishMCPServerVersion(ctx, srv.ID, "2.0.0"); err != nil {
		t.Fatalf("PublishMCPServerVersion v2: %v", err)
	}
	row, err = sharedDB.GetMCPServer(ctx, "acme", "srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer after v2: %v", err)
	}
	if !reflect.DeepEqual(row.Tags, []string{"free"}) {
		t.Errorf("entry tags after v2 = %v, want [free]", row.Tags)
	}

	// The list filter matches the latest published version's tags only.
	byFree, _, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Tag: "free"})
	if err != nil {
		t.Fatalf("ListMCPServers(tag=free): %v", err)
	}
	if len(byFree) != 1 {
		t.Errorf("tag=free matched %d servers, want 1", len(byFree))
	}
	byEA, total, err := sharedDB.ListMCPServers(ctx, store.ListMCPServersParams{Tag: "early-access"})
	if err != nil {
		t.Fatalf("ListMCPServers(tag=early-access): %v", err)
	}
	if len(byEA) != 0 || total != 0 {
		t.Errorf("tag=early-access matched %d servers (total %d), want 0 — v1 is superseded", len(byEA), total)
	}
}

func TestAgentVersionTags_FlowThroughReads(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "acme", "Acme")
	insertTag(t, "free", "Free", "green")

	agent, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "bot", Name: "Bot",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	v, err := sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID: agent.ID, Version: "1.0.0", EndpointURL: "https://bot.example.com",
		Skills: json.RawMessage(`[]`), ProtocolVersion: "0.2.1",
		Tags: []string{"free"},
	})
	if err != nil {
		t.Fatalf("CreateAgentVersion: %v", err)
	}
	if !reflect.DeepEqual(v.Tags, []string{"free"}) {
		t.Errorf("created agent version tags = %v", v.Tags)
	}
	if err := sharedDB.PublishAgentVersion(ctx, agent.ID, "1.0.0"); err != nil {
		t.Fatalf("PublishAgentVersion: %v", err)
	}

	row, err := sharedDB.GetAgent(ctx, "acme", "bot", false)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if !reflect.DeepEqual(row.Tags, []string{"free"}) {
		t.Errorf("agent entry tags = %v", row.Tags)
	}
	if row.LatestVersion == nil || !reflect.DeepEqual(row.LatestVersion.Tags, []string{"free"}) {
		t.Errorf("agent latest_version tags = %+v", row.LatestVersion)
	}

	matched, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{Tag: "free"})
	if err != nil {
		t.Fatalf("ListAgents(tag=free): %v", err)
	}
	if len(matched) != 1 {
		t.Errorf("tag=free matched %d agents, want 1", len(matched))
	}
	none, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{Tag: "nope"})
	if err != nil {
		t.Fatalf("ListAgents(tag=nope): %v", err)
	}
	if len(none) != 0 {
		t.Errorf("tag=nope matched %d agents, want 0", len(none))
	}
}

func TestReconcileManagedInstanceTags(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// An admin-created tag that the configuration never mentions.
	insertTag(t, "manual", "Manual", "blue")

	if err := sharedDB.ReconcileManagedInstanceTags(ctx, []store.ManagedTagSpec{
		{Slug: "free", Name: "Free", Description: "No cost", Color: "green", Active: true},
		{Slug: "legacy", Name: "Legacy", Color: "gray", Active: false},
	}); err != nil {
		t.Fatalf("ReconcileManagedInstanceTags: %v", err)
	}

	byCfg := func() map[string]domain.InstanceTag {
		tags, err := sharedDB.ListInstanceTags(ctx)
		if err != nil {
			t.Fatalf("ListInstanceTags: %v", err)
		}
		m := make(map[string]domain.InstanceTag, len(tags))
		for _, tag := range tags {
			m[tag.Slug] = tag
		}
		return m
	}

	tags := byCfg()
	if len(tags) != 3 {
		t.Fatalf("got %d tags, want 3: %+v", len(tags), tags)
	}
	if !tags["free"].Managed || tags["free"].Name != "Free" || !tags["free"].Active {
		t.Errorf("free not reconciled as expected: %+v", tags["free"])
	}
	if !tags["legacy"].Managed || tags["legacy"].Active {
		t.Errorf("legacy should be managed + inactive: %+v", tags["legacy"])
	}
	if tags["manual"].Managed {
		t.Errorf("manual (admin-created) must not become managed: %+v", tags["manual"])
	}

	// Managed tags are read-only via the CRUD paths.
	name := "Hacked"
	if _, err := sharedDB.UpdateInstanceTag(ctx, "free", store.UpdateInstanceTagParams{Name: &name}); err != store.ErrManagedTag {
		t.Errorf("update of managed tag: got %v, want ErrManagedTag", err)
	}
	if err := sharedDB.DeleteInstanceTag(ctx, "free"); err != store.ErrManagedTag {
		t.Errorf("delete of managed tag: got %v, want ErrManagedTag", err)
	}

	// Second reconcile: free updated, manual adopted, legacy released.
	if err := sharedDB.ReconcileManagedInstanceTags(ctx, []store.ManagedTagSpec{
		{Slug: "free", Name: "Gratis", Color: "teal", Active: true},
		{Slug: "manual", Name: "Manual", Color: "blue", Active: true},
	}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	tags = byCfg()
	if tags["free"].Name != "Gratis" || tags["free"].Color != "teal" || !tags["free"].Managed {
		t.Errorf("free not updated by reconcile: %+v", tags["free"])
	}
	if !tags["manual"].Managed {
		t.Errorf("manual should be adopted as managed: %+v", tags["manual"])
	}
	if tags["legacy"].Managed {
		t.Errorf("legacy should be released (managed=false): %+v", tags["legacy"])
	}
	// Released tags become editable again.
	if _, err := sharedDB.UpdateInstanceTag(ctx, "legacy", store.UpdateInstanceTagParams{Name: &name}); err != nil {
		t.Errorf("update of released tag should succeed, got %v", err)
	}

	// Empty configuration releases everything but deletes nothing.
	if err := sharedDB.ReconcileManagedInstanceTags(ctx, nil); err != nil {
		t.Fatalf("empty reconcile: %v", err)
	}
	tags = byCfg()
	if len(tags) != 3 {
		t.Fatalf("empty reconcile must not delete tags, got %d", len(tags))
	}
	for slug, tag := range tags {
		if tag.Managed {
			t.Errorf("%s still managed after empty reconcile", slug)
		}
	}
}
