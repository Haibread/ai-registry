package store_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

var validSkills = json.RawMessage(
	`[{"id":"search","name":"Search","description":"Searches the web","tags":["search"]}]`)

var validAuthentication = json.RawMessage(`[{"scheme":"Bearer"}]`)

func TestCreateAndGetAgent(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pubID := insertPublisher(t, "agent-ns", "Agent Corp")

	agent, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID,
		Slug:        "my-agent",
		Name:        "My Agent",
		Description: "A test agent",
	})
	if err != nil {
		t.Fatalf("CreateAgent() error = %v", err)
	}
	if agent.ID == "" {
		t.Error("expected non-empty ID")
	}
	if agent.Status != domain.StatusDraft {
		t.Errorf("status = %v, want draft", agent.Status)
	}
	if agent.Visibility != domain.VisibilityPrivate {
		t.Errorf("visibility = %v, want private", agent.Visibility)
	}

	got, err := sharedDB.GetAgent(ctx, "agent-ns", "my-agent", false)
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if got.ID != agent.ID {
		t.Errorf("id = %v, want %v", got.ID, agent.ID)
	}
	if got.Namespace != "agent-ns" {
		t.Errorf("namespace = %v, want agent-ns", got.Namespace)
	}
	if got.Description != "A test agent" {
		t.Errorf("description = %q, want %q", got.Description, "A test agent")
	}
}

func TestCreateAgent_ConflictOnDuplicateSlug(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "agent-ns2", "Agent Corp 2")

	params := store.CreateAgentParams{PublisherID: pubID, Slug: "dup-agent", Name: "Dup"}
	if _, err := sharedDB.CreateAgent(ctx, params); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := sharedDB.CreateAgent(ctx, params)
	if err != store.ErrConflict {
		t.Errorf("expected ErrConflict, got %v", err)
	}
}

func TestGetAgent_NotFoundWhenPrivateAndPublicOnly(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "agent-ns3", "Agent Corp 3")

	if _, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "priv-agent", Name: "Private",
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := sharedDB.GetAgent(ctx, "agent-ns3", "priv-agent", true)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for private agent with publicOnly=true, got %v", err)
	}
}

func TestAgentVersionLifecycle(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "lifecycle-agent-ns", "Lifecycle Agent Corp")

	agent, err := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "lifecycle-agent", Name: "Lifecycle Agent",
	})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}

	// Create a draft version.
	ver, err := sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID:         agent.ID,
		Version:         "1.0.0",
		EndpointURL:     "https://agent.example.com/api",
		Skills:          validSkills,
		Authentication:  validAuthentication,
		ProtocolVersion: domain.A2AProtocolVersion,
	})
	if err != nil {
		t.Fatalf("CreateAgentVersion: %v", err)
	}
	if ver.IsPublished() {
		t.Error("newly created version should not be published")
	}
	if ver.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", ver.Version, "1.0.0")
	}

	// Publish it.
	if err := sharedDB.PublishAgentVersion(ctx, agent.ID, "1.0.0"); err != nil {
		t.Fatalf("PublishAgentVersion: %v", err)
	}

	// Fetch and verify published.
	got, err := sharedDB.GetAgentVersion(ctx, agent.ID, "1.0.0")
	if err != nil {
		t.Fatalf("GetAgentVersion: %v", err)
	}
	if !got.IsPublished() {
		t.Error("version should be published after publish call")
	}

	// Publishing again should return ErrImmutable.
	if err := sharedDB.PublishAgentVersion(ctx, agent.ID, "1.0.0"); err != store.ErrImmutable {
		t.Errorf("expected ErrImmutable on re-publish, got %v", err)
	}

	// Duplicate version should return ErrConflict.
	_, err = sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID:         agent.ID,
		Version:         "1.0.0",
		EndpointURL:     "https://agent.example.com/api",
		Skills:          validSkills,
		ProtocolVersion: domain.A2AProtocolVersion,
	})
	if err != store.ErrConflict {
		t.Errorf("expected ErrConflict on duplicate version, got %v", err)
	}
}

func TestGetLatestPublishedAgentVersion(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "latest-agent-ns", "Latest Agent Corp")

	agent, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "latest-agent", Name: "Latest Agent",
	})

	// No published version yet.
	_, err := sharedDB.GetLatestPublishedAgentVersion(ctx, agent.ID)
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound with no published versions, got %v", err)
	}

	// Create and publish 1.0.0.
	sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID: agent.ID, Version: "1.0.0", EndpointURL: "https://agent.example.com/api",
		Skills: validSkills, ProtocolVersion: domain.A2AProtocolVersion,
	})
	sharedDB.PublishAgentVersion(ctx, agent.ID, "1.0.0")

	// Create and publish 2.0.0.
	sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID: agent.ID, Version: "2.0.0", EndpointURL: "https://agent.example.com/api",
		Skills: validSkills, ProtocolVersion: domain.A2AProtocolVersion,
	})
	sharedDB.PublishAgentVersion(ctx, agent.ID, "2.0.0")

	latest, err := sharedDB.GetLatestPublishedAgentVersion(ctx, agent.ID)
	if err != nil {
		t.Fatalf("GetLatestPublishedAgentVersion: %v", err)
	}
	if latest.Version != "2.0.0" {
		t.Errorf("latest version = %q, want %q", latest.Version, "2.0.0")
	}
}

func TestListAgents_Filtering(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	pub1 := insertPublisher(t, "agent-filter-ns1", "Filter NS 1")
	pub2 := insertPublisher(t, "agent-filter-ns2", "Filter NS 2")

	// Create a public agent under pub1.
	ag1, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pub1, Slug: "public-agent", Name: "Public Agent",
	})
	sharedDB.SetAgentVisibility(ctx, ag1.ID, domain.VisibilityPublic)

	// Create a private agent under pub2.
	sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pub2, Slug: "private-agent", Name: "Private Agent",
	})

	// PublicOnly=true should return only public entries.
	rows, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{PublicOnly: true, Limit: 20})
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	for _, r := range rows {
		if r.Visibility != domain.VisibilityPublic {
			t.Errorf("expected public visibility, got %v for agent %v", r.Visibility, r.Slug)
		}
	}

	// Namespace filter.
	rows2, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{Namespace: "agent-filter-ns1", Limit: 20})
	if err != nil {
		t.Fatalf("ListAgents with namespace: %v", err)
	}
	for _, r := range rows2 {
		if r.Namespace != "agent-filter-ns1" {
			t.Errorf("expected namespace agent-filter-ns1, got %v", r.Namespace)
		}
	}
}

func TestDeprecateAgent(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "dep-agent-ns", "Deprecate Agent NS")

	agent, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "dep-agent", Name: "Dep Agent",
	})

	// Can't deprecate a draft agent.
	if err := sharedDB.DeprecateAgent(ctx, agent.ID); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound when deprecating draft, got %v", err)
	}

	// Publish a version first, which promotes agent to published.
	sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID: agent.ID, Version: "1.0.0", EndpointURL: "https://agent.example.com/api",
		Skills: validSkills, ProtocolVersion: domain.A2AProtocolVersion,
	})
	sharedDB.PublishAgentVersion(ctx, agent.ID, "1.0.0")

	if err := sharedDB.DeprecateAgent(ctx, agent.ID); err != nil {
		t.Fatalf("DeprecateAgent: %v", err)
	}

	// Check status is deprecated.
	got, _ := sharedDB.GetAgent(ctx, "dep-agent-ns", "dep-agent", false)
	if got.Status != domain.StatusDeprecated {
		t.Errorf("status = %v, want deprecated", got.Status)
	}
}

func TestListAgentVersions(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "listver-agent-ns", "ListVer Agent NS")

	agent, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "listver-agent", Name: "ListVer Agent",
	})

	for _, v := range []string{"1.0.0", "1.1.0", "2.0.0"} {
		sharedDB.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
			AgentID: agent.ID, Version: v, EndpointURL: "https://agent.example.com/api",
			Skills: validSkills, ProtocolVersion: domain.A2AProtocolVersion,
		})
	}

	versions, err := sharedDB.ListAgentVersions(ctx, agent.ID)
	if err != nil {
		t.Fatalf("ListAgentVersions: %v", err)
	}
	if len(versions) != 3 {
		t.Errorf("versions count = %d, want 3", len(versions))
	}
}

func TestGetAgentVersion_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "getaver-ns", "GetAgentVer NS")

	agent, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "getaver-agent", Name: "GetAgentVer Agent",
	})

	_, err := sharedDB.GetAgentVersion(ctx, agent.ID, "9.9.9")
	if err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for missing version, got %v", err)
	}
}

func TestSetAgentVisibility(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "avis-ns", "AgentVis NS")

	agent, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "avis-agent", Name: "AgentVis Agent",
	})

	// Set to public.
	if err := sharedDB.SetAgentVisibility(ctx, agent.ID, domain.VisibilityPublic); err != nil {
		t.Fatalf("SetAgentVisibility(public): %v", err)
	}
	got, err := sharedDB.GetAgent(ctx, "avis-ns", "avis-agent", false)
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if got.Visibility != domain.VisibilityPublic {
		t.Errorf("visibility = %v, want public", got.Visibility)
	}

	// Set back to private.
	if err := sharedDB.SetAgentVisibility(ctx, agent.ID, domain.VisibilityPrivate); err != nil {
		t.Fatalf("SetAgentVisibility(private): %v", err)
	}
	got2, _ := sharedDB.GetAgent(ctx, "avis-ns", "avis-agent", false)
	if got2.Visibility != domain.VisibilityPrivate {
		t.Errorf("visibility = %v, want private", got2.Visibility)
	}

	// Non-existent ID must return ErrNotFound.
	if err := sharedDB.SetAgentVisibility(ctx, "nonexistent-id", domain.VisibilityPublic); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for bad ID, got %v", err)
	}
}

func TestListAgents_SearchQuery(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "asearch-ns", "AgentSearch NS")

	sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "alpha-agent", Name: "AlphaSearch Agent",
		Description: "Unique alpha description for agent search test",
	})
	sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pubID, Slug: "beta-agent", Name: "BetaOther Agent",
		Description: "Completely different beta description",
	})

	rows, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{
		Query: "alpha", Limit: 20,
	})
	if err != nil {
		t.Fatalf("ListAgents(query=alpha): %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 result for query 'alpha', got %d", len(rows))
	}
	if len(rows) > 0 && rows[0].Slug != "alpha-agent" {
		t.Errorf("expected slug alpha-agent, got %s", rows[0].Slug)
	}
}

func TestListAgents_NamespaceFilter(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pub1 := insertPublisher(t, "ansfilt-ns1", "AgentNS Filter 1")
	pub2 := insertPublisher(t, "ansfilt-ns2", "AgentNS Filter 2")

	sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pub1, Slug: "agent-in-ns1", Name: "Agent In NS1",
	})
	sharedDB.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: pub2, Slug: "agent-in-ns2", Name: "Agent In NS2",
	})

	rows, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{
		Namespace: "ansfilt-ns1", Limit: 20,
	})
	if err != nil {
		t.Fatalf("ListAgents(namespace=ansfilt-ns1): %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("expected 1 result for namespace ansfilt-ns1, got %d", len(rows))
	}
	for _, r := range rows {
		if r.Namespace != "ansfilt-ns1" {
			t.Errorf("expected namespace ansfilt-ns1, got %s", r.Namespace)
		}
	}
}

func TestDeprecateAgent_BadID(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// A completely non-existent ID must also return ErrNotFound.
	if err := sharedDB.DeprecateAgent(ctx, "nonexistent-id"); err != store.ErrNotFound {
		t.Errorf("expected ErrNotFound for bad ID, got %v", err)
	}
}

func TestListAgents_FilterByStatus(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "ag-status-ns", "Agent Status NS")

	ag1, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pubID, Slug: "ag-status-draft", Name: "Draft Agent"})
	ag2, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pubID, Slug: "ag-status-published", Name: "Published Agent"})
	ag3, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pubID, Slug: "ag-status-deprecated", Name: "Deprecated Agent"})

	if _, err := sharedDB.Pool.Exec(ctx, "UPDATE agents SET status=$1 WHERE id=$2", "published", ag2.ID); err != nil {
		t.Fatalf("setting published status: %v", err)
	}
	if _, err := sharedDB.Pool.Exec(ctx, "UPDATE agents SET status=$1 WHERE id=$2", "deprecated", ag3.ID); err != nil {
		t.Fatalf("setting deprecated status: %v", err)
	}
	_ = ag1 // stays draft

	for _, tc := range []struct {
		status string
		want   int
		slug   string
	}{
		{"draft", 1, "ag-status-draft"},
		{"published", 1, "ag-status-published"},
		{"deprecated", 1, "ag-status-deprecated"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			rows, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{Status: tc.status, Limit: 20})
			if err != nil {
				t.Fatalf("ListAgents(status=%s): %v", tc.status, err)
			}
			if len(rows) != tc.want {
				t.Errorf("status=%s: got %d rows, want %d", tc.status, len(rows), tc.want)
			}
			if len(rows) > 0 && rows[0].Slug != tc.slug {
				t.Errorf("status=%s: slug=%q, want %q", tc.status, rows[0].Slug, tc.slug)
			}
		})
	}
}

func TestListAgents_FilterByVisibility(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "ag-vis-filter-ns", "Agent Vis Filter NS")

	ag1, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pubID, Slug: "ag-vf-public-1", Name: "Public Agent 1"})
	ag2, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pubID, Slug: "ag-vf-public-2", Name: "Public Agent 2"})
	_, _ = sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pubID, Slug: "ag-vf-private", Name: "Private Agent"})

	for _, id := range []string{ag1.ID, ag2.ID} {
		if _, err := sharedDB.Pool.Exec(ctx, "UPDATE agents SET visibility=$1 WHERE id=$2", "public", id); err != nil {
			t.Fatalf("setting visibility: %v", err)
		}
	}

	pubRows, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{Visibility: "public", Limit: 20})
	if err != nil {
		t.Fatalf("ListAgents(visibility=public): %v", err)
	}
	if len(pubRows) != 2 {
		t.Errorf("visibility=public: got %d rows, want 2", len(pubRows))
	}
	for _, r := range pubRows {
		if r.Visibility != "public" {
			t.Errorf("expected public visibility, got %q for slug %q", r.Visibility, r.Slug)
		}
	}

	privRows, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{Visibility: "private", Limit: 20})
	if err != nil {
		t.Fatalf("ListAgents(visibility=private): %v", err)
	}
	if len(privRows) != 1 {
		t.Errorf("visibility=private: got %d rows, want 1", len(privRows))
	}
	if len(privRows) > 0 && privRows[0].Slug != "ag-vf-private" {
		t.Errorf("visibility=private: slug=%q, want ag-vf-private", privRows[0].Slug)
	}
}

func TestListAgents_FilterCombined(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pub1 := insertPublisher(t, "ag-comb-ns1", "Agent Combined NS1")
	pub2 := insertPublisher(t, "ag-comb-ns2", "Agent Combined NS2")

	// ns1: one public+published, one public+draft, one private+published
	agA, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pub1, Slug: "ag-comb-a", Name: "Comb A"})
	agB, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pub1, Slug: "ag-comb-b", Name: "Comb B"})
	agC, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pub1, Slug: "ag-comb-c", Name: "Comb C"})
	// ns2: one public+published
	agD, _ := sharedDB.CreateAgent(ctx, store.CreateAgentParams{PublisherID: pub2, Slug: "ag-comb-d", Name: "Comb D"})

	type update struct{ id, col, val string }
	for _, u := range []update{
		{agA.ID, "visibility", "public"},
		{agA.ID, "status", "published"},
		{agB.ID, "visibility", "public"},
		// agB stays draft
		{agC.ID, "status", "published"},
		// agC stays private
		{agD.ID, "visibility", "public"},
		{agD.ID, "status", "published"},
	} {
		if _, err := sharedDB.Pool.Exec(ctx, "UPDATE agents SET "+u.col+"=$1 WHERE id=$2", u.val, u.id); err != nil {
			t.Fatalf("update %s=%s on %s: %v", u.col, u.val, u.id, err)
		}
	}

	// namespace=ag-comb-ns1 + status=published + visibility=public => only agA
	rows, _, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{
		Namespace:  "ag-comb-ns1",
		Status:     "published",
		Visibility: "public",
		Limit:      20,
	})
	if err != nil {
		t.Fatalf("ListAgents(combined): %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("combined filter: got %d rows, want 1", len(rows))
	}
	if len(rows) > 0 && rows[0].Slug != "ag-comb-a" {
		t.Errorf("combined filter: slug=%q, want ag-comb-a", rows[0].Slug)
	}
}

func TestListAgents_TotalCount(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	pubID := insertPublisher(t, "atc-ns", "AgentTotalCount NS")

	// Create 3 agents.
	for i := range 3 {
		sharedDB.CreateAgent(ctx, store.CreateAgentParams{ //nolint:errcheck
			PublisherID: pubID,
			Slug:        fmt.Sprintf("atc-ag-%d", i),
			Name:        fmt.Sprintf("AgentTotalCount %d", i),
		})
	}

	// Request page of 2 — should return 2 rows but total_count = 3.
	rows, total, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{Limit: 2})
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(rows) != 2 {
		t.Errorf("page size: got %d rows, want 2", len(rows))
	}
	if total != 3 {
		t.Errorf("total_count: got %d, want 3", total)
	}

	// Second page — total_count must still reflect the full set.
	cursor := store.EncodeCursor(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].ID)
	_, total2, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{Limit: 2, Cursor: cursor})
	if err != nil {
		t.Fatalf("ListAgents page 2: %v", err)
	}
	if total2 != 3 {
		t.Errorf("total_count page 2: got %d, want 3", total2)
	}
}
