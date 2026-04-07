// Command seed inserts representative sample data into the registry database.
// It is idempotent: running it twice produces the same state (entries are
// skipped when they already exist via ON CONFLICT DO NOTHING).
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

	type mcpEntry struct {
		publisher   string
		publisherID string
		slug        string
		name        string
		description string
		repoURL     string
		version     string
		packages    json.RawMessage
	}

	mcpEntries := []mcpEntry{
		{
			publisherID: anthropic,
			slug:        "filesystem",
			name:        "Filesystem MCP Server",
			description: "Gives Claude read/write access to the local file system.",
			repoURL:     "https://github.com/anthropics/mcp-filesystem",
			version:     "1.0.0",
			packages: json.RawMessage(`[{
				"registryType": "npm",
				"identifier": "@anthropic/mcp-filesystem",
				"version": "1.0.0",
				"transport": {"type": "stdio"}
			}]`),
		},
		{
			publisherID: anthropic,
			slug:        "github",
			name:        "GitHub MCP Server",
			description: "Interact with GitHub repositories, issues, and pull requests.",
			repoURL:     "https://github.com/anthropics/mcp-github",
			version:     "0.5.0",
			packages: json.RawMessage(`[{
				"registryType": "npm",
				"identifier": "@anthropic/mcp-github",
				"version": "0.5.0",
				"transport": {"type": "stdio"}
			}]`),
		},
		{
			publisherID: openai,
			slug:        "web-search",
			name:        "Web Search MCP Server",
			description: "Provides real-time web search capabilities via a streaming HTTP transport.",
			repoURL:     "https://github.com/openai/mcp-web-search",
			version:     "2.1.0",
			packages: json.RawMessage(`[{
				"registryType": "npm",
				"identifier": "@openai/mcp-web-search",
				"version": "2.1.0",
				"transport": {"type": "streamable-http", "url": "https://mcp.openai.com/search"}
			}]`),
		},
		{
			publisherID: community,
			slug:        "postgres",
			name:        "PostgreSQL MCP Server",
			description: "Community-maintained server for querying and managing PostgreSQL databases.",
			repoURL:     "https://github.com/community/mcp-postgres",
			version:     "0.3.1",
			packages: json.RawMessage(`[{
				"registryType": "pip",
				"identifier": "mcp-postgres",
				"version": "0.3.1",
				"transport": {"type": "stdio"}
			}]`),
		},
		{
			publisherID: community,
			slug:        "sqlite",
			name:        "SQLite MCP Server",
			description: "Read and write SQLite databases using natural language.",
			repoURL:     "https://github.com/community/mcp-sqlite",
			version:     "1.2.0",
			packages: json.RawMessage(`[{
				"registryType": "pip",
				"identifier": "mcp-sqlite",
				"version": "1.2.0",
				"transport": {"type": "stdio"}
			}]`),
		},
	}

	for _, e := range mcpEntries {
		srv, err := upsertMCPServer(ctx, db, e.publisherID, e.slug, e.name, e.description, e.repoURL)
		if err != nil {
			return err
		}
		if err := upsertMCPVersion(ctx, db, srv.ID, e.version, e.packages); err != nil {
			return err
		}
		slog.Info("seeded MCP server", "slug", e.slug, "version", e.version)
	}

	// ── Agents ────────────────────────────────────────────────────────────────

	type agentEntry struct {
		publisherID string
		slug        string
		name        string
		description string
		endpoint    string
		version     string
		skills      json.RawMessage
	}

	agentEntries := []agentEntry{
		{
			publisherID: anthropic,
			slug:        "code-review",
			name:        "Code Review Agent",
			description: "Automatically reviews pull requests and suggests improvements.",
			endpoint:    "https://agents.anthropic.com/code-review",
			version:     "1.0.0",
			skills: json.RawMessage(`[
				{"id": "review-pr", "name": "Review Pull Request", "description": "Analyse a PR diff and post inline comments", "tags": ["git", "code-quality"]},
				{"id": "suggest-fix", "name": "Suggest Fix", "description": "Generate a code fix for a flagged issue", "tags": ["code-quality"]}
			]`),
		},
		{
			publisherID: openai,
			slug:        "data-analyst",
			name:        "Data Analyst Agent",
			description: "Analyses structured datasets and produces charts and summaries.",
			endpoint:    "https://agents.openai.com/data-analyst",
			version:     "0.9.0",
			skills: json.RawMessage(`[
				{"id": "analyse-csv", "name": "Analyse CSV", "description": "Parse and summarise a CSV file", "tags": ["data", "csv"]},
				{"id": "plot-chart", "name": "Plot Chart", "description": "Generate a chart from tabular data", "tags": ["data", "visualisation"]}
			]`),
		},
		{
			publisherID: community,
			slug:        "docs-writer",
			name:        "Documentation Writer",
			description: "Generates and maintains API documentation from source code.",
			endpoint:    "https://community.example.com/agents/docs-writer",
			version:     "0.2.0",
			skills: json.RawMessage(`[
				{"id": "generate-docs", "name": "Generate Docs", "description": "Extract and write API documentation from code", "tags": ["docs", "openapi"]}
			]`),
		},
	}

	for _, e := range agentEntries {
		ag, err := upsertAgent(ctx, db, e.publisherID, e.slug, e.name, e.description)
		if err != nil {
			return err
		}
		if err := upsertAgentVersion(ctx, db, ag.ID, e.version, e.endpoint, e.skills); err != nil {
			return err
		}
		slog.Info("seeded agent", "slug", e.slug, "version", e.version)
	}

	return nil
}

// mustPublisher inserts a publisher if it doesn't exist and returns its ID.
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

	// Fetch the real ID (may differ if row already existed).
	var realID string
	if err := db.Pool.QueryRow(ctx,
		`SELECT id FROM publishers WHERE slug = $1`, slug,
	).Scan(&realID); err != nil {
		slog.Error("fetching publisher id", "slug", slug, "error", err)
		os.Exit(1)
	}
	slog.Info("publisher ready", "slug", slug, "id", realID)
	return realID
}

// upsertMCPServer creates the server if it doesn't exist.
func upsertMCPServer(ctx context.Context, db *store.DB, publisherID, slug, name, description, repoURL string) (*domain.MCPServer, error) {
	var id string
	err := db.Pool.QueryRow(ctx,
		`SELECT id FROM mcp_servers WHERE publisher_id = $1 AND slug = $2`,
		publisherID, slug,
	).Scan(&id)

	if err == nil {
		// Already exists.
		return &domain.MCPServer{ID: id}, nil
	}

	srv, createErr := db.CreateMCPServer(ctx, store.CreateMCPServerParams{
		PublisherID: publisherID,
		Slug:        slug,
		Name:        name,
		Description: description,
		RepoURL:     repoURL,
	})
	if createErr != nil {
		return nil, createErr
	}

	// Make it public.
	if err := db.SetMCPServerVisibility(ctx, srv.ID, domain.VisibilityPublic); err != nil {
		return nil, err
	}
	return srv, nil
}

// upsertMCPVersion creates and publishes a version if it doesn't exist.
func upsertMCPVersion(ctx context.Context, db *store.DB, serverID, version string, packages json.RawMessage) error {
	var exists bool
	_ = db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM mcp_server_versions WHERE server_id = $1 AND version = $2)`,
		serverID, version,
	).Scan(&exists)
	if exists {
		return nil
	}

	runtime := deriveRuntime(packages)
	ver, err := db.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        serverID,
		Version:         version,
		Runtime:         runtime,
		Packages:        packages,
		ProtocolVersion: "2025-03-26",
	})
	if err != nil {
		return err
	}
	return db.PublishMCPServerVersion(ctx, serverID, ver.Version)
}

// upsertAgent creates an agent if it doesn't exist and returns it.
func upsertAgent(ctx context.Context, db *store.DB, publisherID, slug, name, description string) (*domain.Agent, error) {
	var id string
	err := db.Pool.QueryRow(ctx,
		`SELECT id FROM agents WHERE publisher_id = $1 AND slug = $2`,
		publisherID, slug,
	).Scan(&id)
	if err == nil {
		return &domain.Agent{ID: id}, nil
	}

	ag, err := db.CreateAgent(ctx, store.CreateAgentParams{
		PublisherID: publisherID,
		Slug:        slug,
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, err
	}
	if err := db.SetAgentVisibility(ctx, ag.ID, domain.VisibilityPublic); err != nil {
		return nil, err
	}
	return ag, nil
}

// upsertAgentVersion creates and publishes an agent version if it doesn't exist.
func upsertAgentVersion(ctx context.Context, db *store.DB, agentID, version, endpoint string, skills json.RawMessage) error {
	var exists bool
	_ = db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM agent_versions WHERE agent_id = $1 AND version = $2)`,
		agentID, version,
	).Scan(&exists)
	if exists {
		return nil
	}

	ver, err := db.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID:         agentID,
		Version:         version,
		EndpointURL:     endpoint,
		Skills:          skills,
		ProtocolVersion: domain.A2AProtocolVersion,
	})
	if err != nil {
		return err
	}
	return db.PublishAgentVersion(ctx, agentID, ver.Version)
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
