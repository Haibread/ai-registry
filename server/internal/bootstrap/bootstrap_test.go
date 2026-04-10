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
