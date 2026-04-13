package bootstrap_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/haibread/ai-registry/internal/bootstrap"
	"github.com/haibread/ai-registry/internal/store"
)

// ── shared DB for integration tests ──────────────────────────────────────────

var sharedDB *store.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("bootstrap_test"),
		postgres.WithUsername("registry"),
		postgres.WithPassword("registry"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		panic("starting postgres container: " + err.Error())
	}
	defer testcontainers.TerminateContainer(ctr) //nolint:errcheck

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		panic("getting connection string: " + err.Error())
	}
	if err := store.Migrate(dsn); err != nil {
		panic("running migrations: " + err.Error())
	}
	sharedDB, err = store.Open(ctx, dsn, 5, 1)
	if err != nil {
		panic("opening db: " + err.Error())
	}
	defer sharedDB.Close()

	os.Exit(m.Run())
}

// resetDB truncates all tables between tests.
func resetDB(t *testing.T) {
	t.Helper()
	_, err := sharedDB.Pool.Exec(context.Background(),
		`TRUNCATE agent_versions, agents, mcp_server_versions, mcp_servers, publishers, audit_log RESTART IDENTITY CASCADE`)
	if err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
}

// writeFile creates a temp file with the given content and returns its path.
func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

// ── LoadSpec unit tests ───────────────────────────────────────────────────────

func TestLoadSpec_YAML(t *testing.T) {
	path := writeFile(t, "bootstrap.yaml", `
publishers:
  - slug: "acme"
    name: "Acme Corp"
    verified: true

mcp_servers:
  - publisher: "acme"
    slug: "my-server"
    name: "My Server"
    description: "A test server"
    public: true
    versions:
      - version: "1.0.0"
        status: "published"
        protocol_version: "2025-03-26"
        packages:
          - registry_type: "npm"
            identifier: "@acme/my-server"
            version: "1.0.0"
            transport:
              type: "stdio"

agents:
  - publisher: "acme"
    slug: "my-agent"
    name: "My Agent"
    description: "A test agent"
    public: true
    versions:
      - version: "1.0.0"
        status: "published"
        endpoint_url: "https://agents.acme.com/my-agent"
        skills:
          - id: "do-thing"
            name: "Do Thing"
            description: "Does the thing"
            tags: ["thing"]
        authentication:
          - scheme: "Bearer"
`)
	spec, err := bootstrap.LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}
	if len(spec.Publishers) != 1 {
		t.Errorf("publishers len = %d, want 1", len(spec.Publishers))
	}
	if spec.Publishers[0].Slug != "acme" {
		t.Errorf("publisher slug = %q, want acme", spec.Publishers[0].Slug)
	}
	if !spec.Publishers[0].Verified {
		t.Error("publisher verified = false, want true")
	}
	if len(spec.MCPServers) != 1 {
		t.Errorf("mcp_servers len = %d, want 1", len(spec.MCPServers))
	}
	srv := spec.MCPServers[0]
	if srv.Slug != "my-server" {
		t.Errorf("server slug = %q, want my-server", srv.Slug)
	}
	if len(srv.Versions) != 1 || srv.Versions[0].Version != "1.0.0" {
		t.Errorf("server versions = %v", srv.Versions)
	}
	if len(srv.Versions[0].Packages) != 1 {
		t.Errorf("packages len = %d, want 1", len(srv.Versions[0].Packages))
	}
	if srv.Versions[0].Packages[0].RegistryType != "npm" {
		t.Errorf("registry_type = %q, want npm", srv.Versions[0].Packages[0].RegistryType)
	}
	if len(spec.Agents) != 1 {
		t.Errorf("agents len = %d, want 1", len(spec.Agents))
	}
}

func TestLoadSpec_JSON(t *testing.T) {
	path := writeFile(t, "bootstrap.json", `{
  "publishers": [{"slug": "acme", "name": "Acme Corp", "verified": false}],
  "mcp_servers": [{
    "publisher": "acme",
    "slug": "srv",
    "name": "Server",
    "description": "desc",
    "versions": [{
      "version": "1.0.0",
      "status": "published",
      "packages": [{"registryType": "npm", "identifier": "pkg", "version": "1.0.0", "transport": {"type": "stdio"}}]
    }]
  }],
  "agents": []
}`)
	spec, err := bootstrap.LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}
	if len(spec.Publishers) != 1 || spec.Publishers[0].Slug != "acme" {
		t.Errorf("unexpected publishers: %v", spec.Publishers)
	}
	if len(spec.MCPServers) != 1 || spec.MCPServers[0].Slug != "srv" {
		t.Errorf("unexpected mcp_servers: %v", spec.MCPServers)
	}
}

func TestLoadSpec_UnknownExtension(t *testing.T) {
	path := writeFile(t, "bootstrap.toml", "")
	_, err := bootstrap.LoadSpec(path)
	if err == nil {
		t.Error("expected error for unknown extension, got nil")
	}
}

func TestLoadSpec_InvalidYAML(t *testing.T) {
	path := writeFile(t, "bootstrap.yaml", `publishers: [this is: not: valid`)
	_, err := bootstrap.LoadSpec(path)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestLoadSpec_UnknownKey(t *testing.T) {
	path := writeFile(t, "bootstrap.yaml", `
publishers:
  - slug: "acme"
    name: "Acme Corp"
unknown_top_level: true
`)
	_, err := bootstrap.LoadSpec(path)
	if err == nil {
		t.Error("expected error for unknown key, got nil")
	}
}

func TestLoadSpec_ValidationError_MissingPublisher(t *testing.T) {
	path := writeFile(t, "bootstrap.yaml", `
publishers:
  - slug: "acme"
    name: "Acme Corp"
mcp_servers:
  - publisher: "nonexistent"
    slug: "srv"
    name: "Server"
    versions:
      - version: "1.0.0"
        packages:
          - registry_type: "npm"
            identifier: "pkg"
            version: "1.0.0"
            transport:
              type: "stdio"
`)
	_, err := bootstrap.LoadSpec(path)
	if err == nil {
		t.Error("expected validation error for unknown publisher reference, got nil")
	}
}

func TestLoadSpec_ValidationError_MissingVersion(t *testing.T) {
	path := writeFile(t, "bootstrap.yaml", `
publishers:
  - slug: "acme"
    name: "Acme Corp"
mcp_servers:
  - publisher: "acme"
    slug: "srv"
    name: "Server"
    versions:
      - status: "published"
        packages:
          - registry_type: "npm"
            identifier: "pkg"
            version: "1.0.0"
            transport:
              type: "stdio"
`)
	_, err := bootstrap.LoadSpec(path)
	if err == nil {
		t.Error("expected validation error for missing version string, got nil")
	}
}

// TestLoadSpec_ExampleYAML guards the shipped bootstrap example against
// drift. The file is embedded in the Helm chart (as files/bootstrap-sample.yaml)
// and documented as the canonical reference, so a failed parse here should
// fail CI long before a bad release ships.
func TestLoadSpec_ExampleYAML(t *testing.T) {
	// Repo root is three directories up from server/internal/bootstrap.
	path := filepath.Clean(filepath.Join("..", "..", "..", "deploy", "bootstrap.example.yaml"))
	spec, err := bootstrap.LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec(%q) error = %v", path, err)
	}
	if len(spec.Publishers) == 0 {
		t.Error("example has zero publishers; expected at least one")
	}
	if len(spec.MCPServers) == 0 {
		t.Error("example has zero MCP servers; expected at least one")
	}
	if len(spec.Agents) == 0 {
		t.Error("example has zero agents; expected at least one")
	}
	// Sanity-check that at least one entry exercises the v0.2 fields,
	// otherwise the example isn't demonstrating what the release notes claim.
	var featured, tagged, withReadme, withCaps bool
	for _, s := range spec.MCPServers {
		if s.Featured {
			featured = true
		}
		if len(s.Tags) > 0 {
			tagged = true
		}
		if s.Readme != "" {
			withReadme = true
		}
		for _, v := range s.Versions {
			if len(v.Capabilities) > 0 {
				withCaps = true
			}
		}
	}
	if !featured {
		t.Error("example has no featured MCP server")
	}
	if !tagged {
		t.Error("example has no tagged MCP server")
	}
	if !withReadme {
		t.Error("example has no MCP server with a readme")
	}
	if !withCaps {
		t.Error("example has no MCP version with capabilities")
	}
}

// TestLoadSpec_HelmSampleYAML verifies the helm-chart mirror is parseable and
// stays in lockstep with the canonical example. A drift here means someone
// edited one file and forgot the other.
func TestLoadSpec_HelmSampleYAML(t *testing.T) {
	canonical := filepath.Clean(filepath.Join("..", "..", "..", "deploy", "bootstrap.example.yaml"))
	sample := filepath.Clean(filepath.Join("..", "..", "..", "deploy", "helm", "ai-registry", "files", "bootstrap-sample.yaml"))

	canonBytes, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("reading canonical: %v", err)
	}
	sampleBytes, err := os.ReadFile(sample)
	if err != nil {
		t.Fatalf("reading helm sample: %v", err)
	}
	if string(canonBytes) != string(sampleBytes) {
		t.Fatalf("helm sample has drifted from deploy/bootstrap.example.yaml — re-sync with `cp`")
	}
	if _, err := bootstrap.LoadSpec(sample); err != nil {
		t.Fatalf("LoadSpec(helm sample) error = %v", err)
	}
}

func TestLoadSpec_MissingFile(t *testing.T) {
	_, err := bootstrap.LoadSpec("/tmp/this-file-does-not-exist-bootstrap.yaml")
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}

// ── Run integration tests ─────────────────────────────────────────────────────

func TestRun_BasicBootstrap(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	path := writeFile(t, "bootstrap.yaml", `
publishers:
  - slug: "acme"
    name: "Acme Corp"
    verified: true

mcp_servers:
  - publisher: "acme"
    slug: "my-server"
    name: "My Server"
    description: "Test server"
    public: true
    versions:
      - version: "1.0.0"
        status: "published"
        packages:
          - registry_type: "npm"
            identifier: "@acme/my-server"
            version: "1.0.0"
            transport:
              type: "stdio"
      - version: "2.0.0"
        status: "draft"
        packages:
          - registry_type: "npm"
            identifier: "@acme/my-server"
            version: "2.0.0"
            transport:
              type: "stdio"

agents:
  - publisher: "acme"
    slug: "my-agent"
    name: "My Agent"
    description: "Test agent"
    public: true
    versions:
      - version: "1.0.0"
        status: "published"
        endpoint_url: "https://agents.acme.com/my-agent"
        default_input_modes: ["text/plain"]
        default_output_modes: ["text/plain"]
        skills:
          - id: "do-thing"
            name: "Do Thing"
            description: "Does the thing"
            tags: ["thing"]
        authentication:
          - scheme: "Bearer"
`)
	spec, err := bootstrap.LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}

	if err := bootstrap.Run(ctx, sharedDB, spec, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	// Verify publisher exists.
	var pubID string
	if err := sharedDB.Pool.QueryRow(ctx, `SELECT id FROM publishers WHERE slug = $1`, "acme").Scan(&pubID); err != nil {
		t.Fatalf("publisher not found: %v", err)
	}

	// Verify MCP server exists and is public.
	srv, err := sharedDB.GetMCPServer(ctx, "acme", "my-server", false)
	if err != nil {
		t.Fatalf("GetMCPServer() error = %v", err)
	}
	if srv.Visibility != "public" {
		t.Errorf("server visibility = %q, want public", srv.Visibility)
	}

	// Verify both versions were created.
	versions, err := sharedDB.ListMCPServerVersions(ctx, srv.ID)
	if err != nil {
		t.Fatalf("ListMCPServerVersions() error = %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("version count = %d, want 2", len(versions))
	}

	// Verify agent exists.
	agent, err := sharedDB.GetAgent(ctx, "acme", "my-agent", false)
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if agent.Visibility != "public" {
		t.Errorf("agent visibility = %q, want public", agent.Visibility)
	}
}

func TestRun_Idempotent(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	path := writeFile(t, "bootstrap.yaml", `
publishers:
  - slug: "acme"
    name: "Acme Corp"
mcp_servers:
  - publisher: "acme"
    slug: "srv"
    name: "Server"
    description: "desc"
    public: true
    versions:
      - version: "1.0.0"
        status: "published"
        packages:
          - registry_type: "npm"
            identifier: "@acme/srv"
            version: "1.0.0"
            transport:
              type: "stdio"
agents: []
`)
	spec, err := bootstrap.LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}

	// Run twice — must not error.
	for i := range 2 {
		if err := bootstrap.Run(ctx, sharedDB, spec, nil); err != nil {
			t.Fatalf("Run() iteration %d error = %v", i, err)
		}
	}

	// Exactly one server and one version must exist.
	srv, err := sharedDB.GetMCPServer(ctx, "acme", "srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer() error = %v", err)
	}
	versions, err := sharedDB.ListMCPServerVersions(ctx, srv.ID)
	if err != nil {
		t.Fatalf("ListMCPServerVersions() error = %v", err)
	}
	if len(versions) != 1 {
		t.Errorf("version count = %d, want 1 (idempotency check)", len(versions))
	}
}

func TestRun_V02Fields(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	path := writeFile(t, "bootstrap.yaml", `
publishers:
  - slug: "acme"
    name: "Acme Corp"

mcp_servers:
  - publisher: "acme"
    slug: "fancy-srv"
    name: "Fancy Server"
    description: "showcase entry"
    public: true
    featured: true
    verified: true
    tags: ["official", "featured"]
    readme: "# Fancy Server\n\nA showcase of the v0.2 fields."
    versions:
      - version: "1.0.0"
        status: "published"
        capabilities:
          tools:
            listChanged: true
          resources: {}
        packages:
          - registry_type: "npm"
            identifier: "@acme/fancy"
            version: "1.0.0"
            transport:
              type: "stdio"

agents:
  - publisher: "acme"
    slug: "fancy-agent"
    name: "Fancy Agent"
    description: "showcase agent"
    public: true
    featured: true
    verified: true
    tags: ["demo", "a2a"]
    readme: "# Fancy Agent\n\nA showcase of the v0.2 fields."
    versions:
      - version: "1.0.0"
        status: "published"
        endpoint_url: "https://agents.acme.com/fancy"
        skills:
          - id: "do-thing"
            name: "Do Thing"
            description: "Does a thing"
            tags: ["demo"]
        authentication:
          - scheme: "Bearer"
`)
	spec, err := bootstrap.LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}
	if err := bootstrap.Run(ctx, sharedDB, spec, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	srv, err := sharedDB.GetMCPServer(ctx, "acme", "fancy-srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer() error = %v", err)
	}
	if !srv.Featured {
		t.Error("server featured = false, want true")
	}
	if !srv.Verified {
		t.Error("server verified = false, want true")
	}
	if len(srv.Tags) != 2 || srv.Tags[0] != "official" || srv.Tags[1] != "featured" {
		t.Errorf("server tags = %v, want [official featured]", srv.Tags)
	}
	if srv.Readme == "" {
		t.Error("server readme is empty, want non-empty markdown")
	}

	// Verify capabilities persisted on the version.
	versions, err := sharedDB.ListMCPServerVersions(ctx, srv.ID)
	if err != nil {
		t.Fatalf("ListMCPServerVersions() error = %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("version count = %d, want 1", len(versions))
	}
	caps := string(versions[0].Capabilities)
	if caps == "" || caps == "{}" {
		t.Errorf("version capabilities = %q, want non-empty JSON", caps)
	}

	agent, err := sharedDB.GetAgent(ctx, "acme", "fancy-agent", false)
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if !agent.Featured {
		t.Error("agent featured = false, want true")
	}
	if !agent.Verified {
		t.Error("agent verified = false, want true")
	}
	if len(agent.Tags) != 2 || agent.Tags[0] != "demo" || agent.Tags[1] != "a2a" {
		t.Errorf("agent tags = %v, want [demo a2a]", agent.Tags)
	}
	if agent.Readme == "" {
		t.Error("agent readme is empty, want non-empty markdown")
	}
}

// TestRun_V02Fields_IdempotentPreservesAdminEdits verifies that re-running
// bootstrap does NOT overwrite admin edits to the metadata fields. This is
// the whole point of the `if created` guard in upsertMCPServer/upsertAgent:
// bootstrap seeds the initial state, but after that it must be hands-off.
func TestRun_V02Fields_IdempotentPreservesAdminEdits(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	path := writeFile(t, "bootstrap.yaml", `
publishers:
  - slug: "acme"
    name: "Acme Corp"

mcp_servers:
  - publisher: "acme"
    slug: "fancy-srv"
    name: "Fancy Server"
    description: "showcase"
    public: true
    featured: true
    verified: true
    tags: ["bootstrap-tag"]
    readme: "bootstrap readme"
    versions:
      - version: "1.0.0"
        status: "published"
        packages:
          - registry_type: "npm"
            identifier: "@acme/fancy"
            version: "1.0.0"
            transport:
              type: "stdio"

agents:
  - publisher: "acme"
    slug: "fancy-agent"
    name: "Fancy Agent"
    description: "showcase"
    public: true
    featured: true
    verified: true
    tags: ["bootstrap-tag"]
    readme: "bootstrap readme"
    versions:
      - version: "1.0.0"
        status: "published"
        endpoint_url: "https://agents.acme.com/fancy"
        skills:
          - id: "do"
            name: "Do"
            description: "do it"
            tags: ["x"]
        authentication:
          - scheme: "Bearer"
`)
	spec, err := bootstrap.LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}
	if err := bootstrap.Run(ctx, sharedDB, spec, nil); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}

	// Simulate an admin editing the metadata out-of-band (the way the real
	// admin UI would through its PATCH handler). These values deliberately
	// differ from the bootstrap spec.
	srv, err := sharedDB.GetMCPServer(ctx, "acme", "fancy-srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer() error = %v", err)
	}
	if _, err := sharedDB.Pool.Exec(ctx,
		`UPDATE mcp_servers SET featured=false, verified=false, tags=$1, readme=$2 WHERE id=$3`,
		[]string{"admin-tag"}, "admin readme", srv.ID,
	); err != nil {
		t.Fatalf("admin update (mcp): %v", err)
	}

	agent, err := sharedDB.GetAgent(ctx, "acme", "fancy-agent", false)
	if err != nil {
		t.Fatalf("GetAgent() error = %v", err)
	}
	if _, err := sharedDB.Pool.Exec(ctx,
		`UPDATE agents SET featured=false, verified=false, tags=$1, readme=$2 WHERE id=$3`,
		[]string{"admin-tag"}, "admin readme", agent.ID,
	); err != nil {
		t.Fatalf("admin update (agent): %v", err)
	}

	// Re-run bootstrap. The existing rows must be left untouched.
	if err := bootstrap.Run(ctx, sharedDB, spec, nil); err != nil {
		t.Fatalf("second Run() error = %v", err)
	}

	srvAfter, err := sharedDB.GetMCPServer(ctx, "acme", "fancy-srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer() after: %v", err)
	}
	if srvAfter.Featured {
		t.Error("mcp server: bootstrap clobbered admin featured=false")
	}
	if srvAfter.Verified {
		t.Error("mcp server: bootstrap clobbered admin verified=false")
	}
	if len(srvAfter.Tags) != 1 || srvAfter.Tags[0] != "admin-tag" {
		t.Errorf("mcp server: tags = %v, want [admin-tag]", srvAfter.Tags)
	}
	if srvAfter.Readme != "admin readme" {
		t.Errorf("mcp server: readme = %q, want %q", srvAfter.Readme, "admin readme")
	}

	agentAfter, err := sharedDB.GetAgent(ctx, "acme", "fancy-agent", false)
	if err != nil {
		t.Fatalf("GetAgent() after: %v", err)
	}
	if agentAfter.Featured {
		t.Error("agent: bootstrap clobbered admin featured=false")
	}
	if agentAfter.Verified {
		t.Error("agent: bootstrap clobbered admin verified=false")
	}
	if len(agentAfter.Tags) != 1 || agentAfter.Tags[0] != "admin-tag" {
		t.Errorf("agent: tags = %v, want [admin-tag]", agentAfter.Tags)
	}
	if agentAfter.Readme != "admin readme" {
		t.Errorf("agent: readme = %q, want %q", agentAfter.Readme, "admin readme")
	}
}

func TestRun_DeprecatedServerAndVersion(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	path := writeFile(t, "bootstrap.yaml", `
publishers:
  - slug: "acme"
    name: "Acme Corp"
mcp_servers:
  - publisher: "acme"
    slug: "old-srv"
    name: "Old Server"
    description: "deprecated server"
    status: "deprecated"
    public: true
    versions:
      - version: "1.0.0"
        status: "deprecated"
        status_message: "Use new-srv instead"
        packages:
          - registry_type: "npm"
            identifier: "@acme/old-srv"
            version: "1.0.0"
            transport:
              type: "stdio"
agents: []
`)
	spec, err := bootstrap.LoadSpec(path)
	if err != nil {
		t.Fatalf("LoadSpec() error = %v", err)
	}
	if err := bootstrap.Run(ctx, sharedDB, spec, nil); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	srv, err := sharedDB.GetMCPServer(ctx, "acme", "old-srv", false)
	if err != nil {
		t.Fatalf("GetMCPServer() error = %v", err)
	}
	if srv.Status != "deprecated" {
		t.Errorf("server status = %q, want deprecated", srv.Status)
	}
	versions, err := sharedDB.ListMCPServerVersions(ctx, srv.ID)
	if err != nil {
		t.Fatalf("ListMCPServerVersions() error = %v", err)
	}
	if len(versions) != 1 || versions[0].Status != "deprecated" {
		t.Errorf("version status = %q, want deprecated", versions[0].Status)
	}
}
