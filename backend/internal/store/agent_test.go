package store_test

import (
	"context"
	"encoding/json"
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
	rows, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{PublicOnly: true, Limit: 20})
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	for _, r := range rows {
		if r.Visibility != domain.VisibilityPublic {
			t.Errorf("expected public visibility, got %v for agent %v", r.Visibility, r.Slug)
		}
	}

	// Namespace filter.
	rows2, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{Namespace: "agent-filter-ns1", Limit: 20})
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

	rows, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{
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

	rows, err := sharedDB.ListAgents(ctx, store.ListAgentsParams{
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
