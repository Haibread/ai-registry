// Command seed inserts representative sample data into the registry database.
// It is idempotent: running it twice produces the same state (entries are
// skipped when they already exist via ON CONFLICT DO NOTHING or existence
// checks).
//
// The data set is intentionally diverse:
//   - Transport types: stdio, sse, http, streamable-http
//   - Registry types: npm, pypi, oci, nuget, mcpb
//   - Server statuses: published (active), deprecated, draft (unpublished)
//   - Visibility: public, private
//   - Multi-version servers (to exercise version history UI)
//   - Agents with skills, input/output modes, and authentication schemes
//
// Usage:
//
//	DATABASE_URL=postgres://... go run ./cmd/seed
package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("seed failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	slog.Info("seed complete")
}

func run() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://registry:registry@localhost:5432/registry?sslmode=disable"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := store.Open(ctx, dsn, 5, 1)
	if err != nil {
		return err
	}
	defer db.Close()

	// ── Publishers ────────────────────────────────────────────────────────────

	anthropic := mustPublisher(ctx, db, "anthropic", "Anthropic")
	openai := mustPublisher(ctx, db, "openai", "OpenAI")
	community := mustPublisher(ctx, db, "community", "Community")

	// ── MCP Servers ───────────────────────────────────────────────────────────
	//
	// Covers all transport types and registry types used in the spec.

	// ── anthropic/filesystem ─ npm, stdio, two versions (v1 published, v2 draft)
	fs := mustMCPServer(ctx, db, anthropic, "filesystem", "Filesystem MCP Server",
		"Gives Claude read/write access to the local file system.",
		"https://github.com/anthropics/mcp-filesystem", true)
	mustMCPVersion(ctx, db, fs, "1.0.0", true, `[{
		"registryType": "npm",
		"identifier":   "@anthropic/mcp-filesystem",
		"version":      "1.0.0",
		"transport":    {"type": "stdio"}
	}]`)
	// v2 is a draft — visible in admin but not in public listing
	mustMCPVersion(ctx, db, fs, "2.0.0", false, `[{
		"registryType": "npm",
		"identifier":   "@anthropic/mcp-filesystem",
		"version":      "2.0.0",
		"transport":    {"type": "stdio"}
	}]`)

	// ── anthropic/computer-use ─ npm, SSE transport
	cu := mustMCPServer(ctx, db, anthropic, "computer-use", "Computer Use MCP Server",
		"Streams real-time desktop screenshots and control events over Server-Sent Events.",
		"https://github.com/anthropics/mcp-computer-use", true)
	mustMCPVersion(ctx, db, cu, "1.2.0", true, `[{
		"registryType": "npm",
		"identifier":   "@anthropic/mcp-computer-use",
		"version":      "1.2.0",
		"transport":    {"type": "sse", "url": "https://mcp.anthropic.com/computer-use/sse"}
	}]`)

	// ── anthropic/memory ─ npm, HTTP transport, deprecated
	mem := mustMCPServer(ctx, db, anthropic, "memory", "Memory MCP Server",
		"Provides long-term memory storage for conversations. Deprecated: use anthropic/knowledge-base.",
		"https://github.com/anthropics/mcp-memory", true)
	mustMCPVersion(ctx, db, mem, "1.0.0", true, `[{
		"registryType": "npm",
		"identifier":   "@anthropic/mcp-memory",
		"version":      "1.0.0",
		"transport":    {"type": "http", "url": "https://mcp.anthropic.com/memory"}
	}]`)
	// Deprecate both the version and the server
	if err := db.SetMCPVersionStatus(ctx, mem, "1.0.0", domain.VersionStatusDeprecated,
		"Replaced by anthropic/knowledge-base"); err != nil {
		return err
	}
	if err := db.SetMCPServerStatus(ctx, mem, domain.StatusDeprecated); err != nil {
		return err
	}

	// ── anthropic/github ─ npm, stdio, two published versions
	gh := mustMCPServer(ctx, db, anthropic, "github", "GitHub MCP Server",
		"Interact with GitHub repositories, issues, and pull requests.",
		"https://github.com/anthropics/mcp-github", true)
	mustMCPVersion(ctx, db, gh, "0.5.0", true, `[{
		"registryType": "npm",
		"identifier":   "@anthropic/mcp-github",
		"version":      "0.5.0",
		"transport":    {"type": "stdio"}
	}]`)
	mustMCPVersion(ctx, db, gh, "1.0.0", true, `[{
		"registryType": "npm",
		"identifier":   "@anthropic/mcp-github",
		"version":      "1.0.0",
		"transport":    {"type": "stdio"}
	}]`)

	// ── openai/web-search ─ npm, streamable-http
	mustMCPServerWithVersion(ctx, db, openai, "web-search", "Web Search MCP Server",
		"Provides real-time web search capabilities via a streaming HTTP transport.",
		"https://github.com/openai/mcp-web-search", true, "2.1.0", `[{
		"registryType": "npm",
		"identifier":   "@openai/mcp-web-search",
		"version":      "2.1.0",
		"transport":    {"type": "streamable-http", "url": "https://mcp.openai.com/search"}
	}]`)

	// ── openai/image-gen ─ OCI container + HTTP transport
	mustMCPServerWithVersion(ctx, db, openai, "image-gen", "Image Generation MCP Server",
		"Generate images from text prompts via a containerised HTTP endpoint.",
		"https://github.com/openai/mcp-image-gen", true, "1.0.0", `[{
		"registryType": "oci",
		"identifier":   "ghcr.io/openai/mcp-image-gen",
		"version":      "1.0.0",
		"transport":    {"type": "http", "url": "https://mcp.openai.com/image-gen"}
	}]`)

	// ── openai/code-interpreter ─ streamable-http, draft (never published)
	ci := mustMCPServer(ctx, db, openai, "code-interpreter", "Code Interpreter MCP Server",
		"Execute Python code in a sandboxed environment and return results.",
		"https://github.com/openai/mcp-code-interpreter", true)
	mustMCPVersion(ctx, db, ci, "0.3.0", false, `[{
		"registryType": "npm",
		"identifier":   "@openai/mcp-code-interpreter",
		"version":      "0.3.0",
		"transport":    {"type": "streamable-http", "url": "https://mcp.openai.com/code-interpreter"}
	}]`)

	// ── community/postgres ─ pypi, stdio
	mustMCPServerWithVersion(ctx, db, community, "postgres", "PostgreSQL MCP Server",
		"Community-maintained server for querying and managing PostgreSQL databases.",
		"https://github.com/community/mcp-postgres", true, "0.3.1", `[{
		"registryType": "pypi",
		"identifier":   "mcp-postgres",
		"version":      "0.3.1",
		"transport":    {"type": "stdio"}
	}]`)

	// ── community/sqlite ─ pypi, stdio
	mustMCPServerWithVersion(ctx, db, community, "sqlite", "SQLite MCP Server",
		"Read and write SQLite databases using natural language.",
		"https://github.com/community/mcp-sqlite", true, "1.2.0", `[{
		"registryType": "pypi",
		"identifier":   "mcp-sqlite",
		"version":      "1.2.0",
		"transport":    {"type": "stdio"}
	}]`)

	// ── community/kubernetes ─ OCI container, HTTP
	mustMCPServerWithVersion(ctx, db, community, "kubernetes", "Kubernetes MCP Server",
		"Manage Kubernetes clusters and workloads from your AI assistant.",
		"https://github.com/community/mcp-kubernetes", true, "0.5.0", `[{
		"registryType": "oci",
		"identifier":   "ghcr.io/community/mcp-kubernetes",
		"version":      "0.5.0",
		"transport":    {"type": "http", "url": "http://localhost:8080"}
	}]`)

	// ── community/dotnet-tools ─ nuget, stdio, private
	// Private: useful for testing admin-only visibility in the UI.
	mustMCPServerWithVersion(ctx, db, community, "dotnet-tools", ".NET Tools MCP Server",
		"Wraps common .NET CLI tools (build, test, publish) as MCP primitives.",
		"https://github.com/community/mcp-dotnet-tools", false, "1.0.0", `[{
		"registryType": "nuget",
		"identifier":   "MCP.DotNetTools",
		"version":      "1.0.0",
		"transport":    {"type": "stdio"}
	}]`)

	// ── community/mcp-bridge ─ mcpb registry type, stdio
	mustMCPServerWithVersion(ctx, db, community, "mcp-bridge", "MCP Bridge Server",
		"Bridges legacy tool APIs into the MCP ecosystem using the mcpb packaging format.",
		"https://github.com/community/mcp-bridge", true, "0.9.0", `[{
		"registryType": "mcpb",
		"identifier":   "mcp-bridge",
		"version":      "0.9.0",
		"transport":    {"type": "stdio"}
	}]`)

	// ── community/multi-transport ─ multiple packages (npm + oci), two transports
	mustMCPServerWithVersion(ctx, db, community, "multi-transport", "Multi-Transport MCP Server",
		"Ships as both an npm package (stdio) and an OCI image (HTTP) — pick the deployment model you need.",
		"https://github.com/community/mcp-multi-transport", true, "1.0.0", `[
		{
			"registryType": "npm",
			"identifier":   "@community/multi-transport",
			"version":      "1.0.0",
			"transport":    {"type": "stdio"}
		},
		{
			"registryType": "oci",
			"identifier":   "ghcr.io/community/mcp-multi-transport",
			"version":      "1.0.0",
			"transport":    {"type": "http", "url": "http://localhost:3000"}
		}
	]`)

	// ── Agents ────────────────────────────────────────────────────────────────

	// ── anthropic/code-review ─ published, public
	mustAgentWithVersion(ctx, db, anthropic, "code-review", "Code Review Agent",
		"Automatically reviews pull requests and suggests improvements.", true,
		"1.0.0", "https://agents.anthropic.com/code-review",
		`[
			{"id": "review-pr",    "name": "Review Pull Request", "description": "Analyse a PR diff and post inline comments", "tags": ["git", "code-quality"]},
			{"id": "suggest-fix",  "name": "Suggest Fix",         "description": "Generate a code fix for a flagged issue",   "tags": ["code-quality"]}
		]`,
		`["text/plain"]`, `["text/plain"]`, "Bearer", true)

	// ── anthropic/browser-agent ─ published, public, text + image I/O
	mustAgentWithVersion(ctx, db, anthropic, "browser-agent", "Browser Automation Agent",
		"Controls a real browser to fill forms, click buttons, and extract information from any website.", true,
		"1.2.0", "https://agents.anthropic.com/browser",
		`[
			{"id": "navigate",    "name": "Navigate",     "description": "Open a URL in the browser",                       "tags": ["browser", "automation"]},
			{"id": "click",       "name": "Click Element", "description": "Click on a CSS selector",                        "tags": ["browser", "automation"]},
			{"id": "screenshot",  "name": "Screenshot",   "description": "Capture the current viewport as a PNG",           "tags": ["browser", "visual"]},
			{"id": "extract",     "name": "Extract Data",  "description": "Pull structured data from a page using a schema", "tags": ["browser", "data"]}
		]`,
		`["text/plain", "application/json"]`, `["text/plain", "image/png"]`, "Bearer", true)

	// ── openai/data-analyst ─ published, public
	mustAgentWithVersion(ctx, db, openai, "data-analyst", "Data Analyst Agent",
		"Analyses structured datasets and produces charts and summaries.", true,
		"0.9.0", "https://agents.openai.com/data-analyst",
		`[
			{"id": "analyse-csv",  "name": "Analyse CSV",   "description": "Parse and summarise a CSV file",           "tags": ["data", "csv"]},
			{"id": "plot-chart",   "name": "Plot Chart",    "description": "Generate a chart from tabular data",        "tags": ["data", "visualisation"]},
			{"id": "sql-query",    "name": "SQL Query",     "description": "Run a SQL query against an uploaded file",  "tags": ["data", "sql"]}
		]`,
		`["text/plain", "application/json", "text/csv"]`, `["text/plain", "image/png", "application/json"]`, "ApiKey", false)

	// ── openai/research-agent ─ deprecated
	researchID := mustAgent(ctx, db, openai, "research-agent", "Research Agent",
		"Searches the web and academic databases to answer deep research questions.", true)
	mustAgentVersionRaw(ctx, db, researchID, "0.5.0",
		"https://agents.openai.com/research",
		`[{"id": "web-search",   "name": "Web Search",    "description": "Search the public web",          "tags": ["search", "research"]},
		  {"id": "paper-search", "name": "Paper Search",  "description": "Query arXiv and Semantic Scholar", "tags": ["research", "academic"]}]`,
		`["text/plain"]`, `["text/plain"]`,
		`[{"scheme": "Bearer"}]`, true)
	if err := db.DeprecateAgent(ctx, researchID); err != nil {
		return err
	}

	// ── community/docs-writer ─ published, public
	mustAgentWithVersion(ctx, db, community, "docs-writer", "Documentation Writer",
		"Generates and maintains API documentation from source code.", true,
		"0.2.0", "https://community.example.com/agents/docs-writer",
		`[{"id": "generate-docs", "name": "Generate Docs", "description": "Extract and write API documentation from code", "tags": ["docs", "openapi"]}]`,
		`["text/plain"]`, `["text/plain"]`, "Bearer", false)

	// ── community/devops-agent ─ draft (never published)
	devopsID := mustAgent(ctx, db, community, "devops-agent", "DevOps Agent",
		"Provisions cloud infrastructure, manages CI/CD pipelines, and monitors deployments.", true)
	mustAgentVersionRaw(ctx, db, devopsID, "0.1.0",
		"https://community.example.com/agents/devops",
		`[
			{"id": "deploy",     "name": "Deploy",       "description": "Deploy an application to a cloud provider", "tags": ["cloud", "deploy"]},
			{"id": "monitor",    "name": "Monitor",      "description": "Check deployment health and logs",          "tags": ["cloud", "observability"]},
			{"id": "rollback",   "name": "Rollback",     "description": "Roll back to a previous release",           "tags": ["cloud", "deploy"]}
		]`,
		`["text/plain", "application/json"]`, `["text/plain", "application/json"]`,
		`[{"scheme": "Bearer"}, {"scheme": "ApiKey"}]`, false /* unpublished draft */)

	// ── community/security-scanner ─ published, private
	secID := mustAgent(ctx, db, community, "security-scanner", "Security Scanner Agent",
		"Scans codebases and infrastructure configs for common vulnerabilities.", false /* private */)
	mustAgentVersionRaw(ctx, db, secID, "1.0.0",
		"https://community.example.com/agents/security-scanner",
		`[
			{"id": "scan-code",   "name": "Scan Code",    "description": "Static analysis for common CVEs",          "tags": ["security", "sast"]},
			{"id": "scan-infra",  "name": "Scan Infra",   "description": "Check IaC files for misconfigurations",    "tags": ["security", "iac"]}
		]`,
		`["text/plain", "application/json"]`, `["text/plain", "application/json"]`,
		`[{"scheme": "ApiKey"}]`, true)

	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mustPublisher(ctx context.Context, db *store.DB, slug, name string) string {
	id := store.NewULID()
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO publishers (id, slug, name, verified, created_at, updated_at)
		 VALUES ($1, $2, $3, true, NOW(), NOW())
		 ON CONFLICT (slug) DO NOTHING`,
		id, slug, name,
	)
	if err != nil {
		slog.Error("inserting publisher", "slug", slug, "error", err)
		os.Exit(1)
	}
	var realID string
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM publishers WHERE slug = $1`, slug).Scan(&realID); err != nil {
		slog.Error("fetching publisher id", "slug", slug, "error", err)
		os.Exit(1)
	}
	slog.Info("publisher ready", "slug", slug, "id", realID)
	return realID
}

// mustMCPServer creates the MCP server record (without a version) and returns its ID.
func mustMCPServer(ctx context.Context, db *store.DB, publisherID, slug, name, description, repoURL string, public bool) string {
	var id string
	err := db.Pool.QueryRow(ctx,
		`SELECT id FROM mcp_servers WHERE publisher_id = $1 AND slug = $2`,
		publisherID, slug,
	).Scan(&id)
	if err == nil {
		return id // already exists
	}

	srv, createErr := db.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: publisherID,
		Slug:        slug,
		Name:        name,
		Description: description,
		RepoURL:     repoURL,
	})
	if createErr != nil {
		slog.Error("creating MCP server", "slug", slug, "error", createErr)
		os.Exit(1)
	}
	if public {
		if err := db.SetMCPServerVisibility(ctx, srv.ID, domain.VisibilityPublic); err != nil {
			slog.Error("setting visibility", "slug", slug, "error", err)
			os.Exit(1)
		}
	}
	return srv.ID
}

// mustMCPVersion creates a server version and optionally publishes it.
func mustMCPVersion(ctx context.Context, db *store.DB, serverID, version string, publish bool, packagesRaw string) {
	var exists bool
	_ = db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM mcp_server_versions WHERE server_id = $1 AND version = $2)`,
		serverID, version,
	).Scan(&exists)
	if exists {
		return
	}

	packages := json.RawMessage(packagesRaw)
	runtime := deriveRuntime(packages)
	ver, err := db.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        serverID,
		Version:         version,
		Runtime:         runtime,
		Packages:        packages,
		ProtocolVersion: "2025-03-26",
	})
	if err != nil {
		slog.Error("creating MCP version", "version", version, "error", err)
		os.Exit(1)
	}
	if publish {
		if err := db.PublishMCPServerVersion(ctx, serverID, ver.Version); err != nil {
			slog.Error("publishing MCP version", "version", version, "error", err)
			os.Exit(1)
		}
	}
	slog.Info("seeded MCP version", "server", serverID, "version", version, "published", publish)
}

// mustMCPServerWithVersion is a convenience wrapper for the common one-shot case.
func mustMCPServerWithVersion(ctx context.Context, db *store.DB, publisherID, slug, name, description, repoURL string, public bool, version, packagesRaw string) {
	id := mustMCPServer(ctx, db, publisherID, slug, name, description, repoURL, public)
	mustMCPVersion(ctx, db, id, version, true, packagesRaw)
	slog.Info("seeded MCP server", "slug", slug, "version", version)
}

// mustAgent creates the agent record and returns its ID.
func mustAgent(ctx context.Context, db *store.DB, publisherID, slug, name, description string, public bool) string {
	var id string
	err := db.Pool.QueryRow(ctx,
		`SELECT id FROM agents WHERE publisher_id = $1 AND slug = $2`,
		publisherID, slug,
	).Scan(&id)
	if err == nil {
		return id
	}

	ag, err := db.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: publisherID,
		Slug:        slug,
		Name:        name,
		Description: description,
	})
	if err != nil {
		slog.Error("creating agent", "slug", slug, "error", err)
		os.Exit(1)
	}
	if public {
		if err := db.SetAgentVisibility(ctx, ag.ID, domain.VisibilityPublic); err != nil {
			slog.Error("setting agent visibility", "slug", slug, "error", err)
			os.Exit(1)
		}
	}
	return ag.ID
}

// mustAgentVersionRaw creates and optionally publishes an agent version with full field control.
func mustAgentVersionRaw(
	ctx context.Context,
	db *store.DB,
	agentID, version, endpoint string,
	skillsRaw, inputModesRaw, outputModesRaw, authRaw string,
	publish bool,
) {
	var exists bool
	_ = db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM agent_versions WHERE agent_id = $1 AND version = $2)`,
		agentID, version,
	).Scan(&exists)
	if exists {
		return
	}

	var inputModes, outputModes []string
	_ = json.Unmarshal([]byte(inputModesRaw), &inputModes)
	_ = json.Unmarshal([]byte(outputModesRaw), &outputModes)

	ver, err := db.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID:            agentID,
		Version:            version,
		EndpointURL:        endpoint,
		Skills:             json.RawMessage(skillsRaw),
		Authentication:     json.RawMessage(authRaw),
		DefaultInputModes:  inputModes,
		DefaultOutputModes: outputModes,
		ProtocolVersion:    domain.A2AProtocolVersion,
	})
	if err != nil {
		slog.Error("creating agent version", "version", version, "error", err)
		os.Exit(1)
	}
	if publish {
		if err := db.PublishAgentVersion(ctx, agentID, ver.Version); err != nil {
			slog.Error("publishing agent version", "version", version, "error", err)
			os.Exit(1)
		}
	}
	slog.Info("seeded agent version", "agent", agentID, "version", version, "published", publish)
}

// mustAgentWithVersion is a convenience wrapper for the common case.
func mustAgentWithVersion(
	ctx context.Context,
	db *store.DB,
	publisherID, slug, name, description string,
	public bool,
	version, endpoint, skillsRaw, inputModesRaw, outputModesRaw, authScheme string,
	streaming bool,
) {
	id := mustAgent(ctx, db, publisherID, slug, name, description, public)

	authRaw := `[{"scheme": "` + authScheme + `"}]`
	mustAgentVersionRaw(ctx, db, id, version, endpoint, skillsRaw, inputModesRaw, outputModesRaw, authRaw, true)
	_ = streaming // stored via input/output modes; kept as param for readability
	slog.Info("seeded agent", "slug", slug, "version", version)
}

// deriveRuntime infers the server runtime from the first package's transport type.
func deriveRuntime(packages json.RawMessage) domain.Runtime {
	var entries []struct {
		Transport struct {
			Type string `json:"type"`
		} `json:"transport"`
	}
	if err := json.Unmarshal(packages, &entries); err != nil || len(entries) == 0 {
		return domain.RuntimeStdio
	}
	switch entries[0].Transport.Type {
	case "http":
		return domain.RuntimeHTTP
	case "sse":
		return domain.RuntimeSSE
	case "streamable-http", "streamable_http":
		return domain.RuntimeStreamableHTTP
	default:
		return domain.RuntimeStdio
	}
}
