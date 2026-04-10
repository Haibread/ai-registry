package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/haibread/ai-registry/internal/domain"
	"github.com/haibread/ai-registry/internal/store"
)

// LoadSpec reads and parses a bootstrap spec from path. The format is
// determined by the file extension: ".yaml" / ".yml" → YAML, ".json" → JSON.
// Any other extension returns an error.
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: read %q: %w", path, err)
	}

	ext := strings.ToLower(filepath.Ext(path))
	var spec Spec
	switch ext {
	case ".yaml", ".yml":
		dec := yaml.NewDecoder(strings.NewReader(string(data)))
		dec.KnownFields(true)
		if err := dec.Decode(&spec); err != nil {
			return nil, fmt.Errorf("bootstrap: parse YAML %q: %w", path, err)
		}
	case ".json":
		dec := json.NewDecoder(strings.NewReader(string(data)))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&spec); err != nil {
			return nil, fmt.Errorf("bootstrap: parse JSON %q: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("bootstrap: unsupported file extension %q (use .yaml, .yml, or .json)", ext)
	}

	if err := validateSpec(&spec); err != nil {
		return nil, fmt.Errorf("bootstrap: invalid spec %q: %w", path, err)
	}
	return &spec, nil
}

// Run applies spec to the database. It is idempotent: entities that already
// exist are skipped, no existing data is modified. logger may be nil.
func Run(ctx context.Context, db *store.DB, spec *Spec, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	// ── Publishers ────────────────────────────────────────────────────────────
	pubIDs := make(map[string]string, len(spec.Publishers))
	for _, p := range spec.Publishers {
		id, err := upsertPublisher(ctx, db, p)
		if err != nil {
			return fmt.Errorf("bootstrap: publisher %q: %w", p.Slug, err)
		}
		pubIDs[p.Slug] = id
		logger.Info("bootstrap: publisher ready", slog.String("slug", p.Slug))
	}

	// ── MCP servers ───────────────────────────────────────────────────────────
	for _, s := range spec.MCPServers {
		pubID, ok := pubIDs[s.Publisher]
		if !ok {
			return fmt.Errorf("bootstrap: mcp_server %q references unknown publisher %q", s.Slug, s.Publisher)
		}
		if err := upsertMCPServer(ctx, db, pubID, s, logger); err != nil {
			return fmt.Errorf("bootstrap: mcp_server %q: %w", s.Slug, err)
		}
	}

	// ── Agents ────────────────────────────────────────────────────────────────
	for _, a := range spec.Agents {
		pubID, ok := pubIDs[a.Publisher]
		if !ok {
			return fmt.Errorf("bootstrap: agent %q references unknown publisher %q", a.Slug, a.Publisher)
		}
		if err := upsertAgent(ctx, db, pubID, a, logger); err != nil {
			return fmt.Errorf("bootstrap: agent %q: %w", a.Slug, err)
		}
	}

	logger.Info("bootstrap: complete",
		slog.Int("publishers", len(spec.Publishers)),
		slog.Int("mcp_servers", len(spec.MCPServers)),
		slog.Int("agents", len(spec.Agents)),
	)
	return nil
}

// ── publishers ────────────────────────────────────────────────────────────────

func upsertPublisher(ctx context.Context, db *store.DB, p PublisherSpec) (string, error) {
	id := store.NewULID()
	verified := p.Verified
	_, err := db.Pool.Exec(ctx,
		`INSERT INTO publishers (id, slug, name, verified, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, NOW(), NOW())
		 ON CONFLICT (slug) DO NOTHING`,
		id, p.Slug, p.Name, verified,
	)
	if err != nil {
		return "", fmt.Errorf("upserting publisher: %w", err)
	}
	// Fetch the real ID (may differ if the row already existed).
	var realID string
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM publishers WHERE slug = $1`, p.Slug).Scan(&realID); err != nil {
		return "", fmt.Errorf("fetching publisher id: %w", err)
	}
	return realID, nil
}

// ── MCP servers ───────────────────────────────────────────────────────────────

func upsertMCPServer(ctx context.Context, db *store.DB, publisherID string, s MCPServerSpec, logger *slog.Logger) error {
	// Check if the server already exists.
	var serverID string
	created := false
	err := db.Pool.QueryRow(ctx,
		`SELECT id FROM mcp_servers WHERE publisher_id = $1 AND slug = $2`,
		publisherID, s.Slug,
	).Scan(&serverID)

	if err != nil {
		// Row not found — create it.
		srv, createErr := db.CreateMCPServer(ctx, store.CreateMCPServerParams{
			PublisherID: publisherID,
			Slug:        s.Slug,
			Name:        s.Name,
			Description: s.Description,
			HomepageURL: s.HomepageURL,
			RepoURL:     s.RepoURL,
			License:     s.License,
		})
		if createErr != nil {
			return fmt.Errorf("creating server: %w", createErr)
		}
		serverID = srv.ID
		created = true

		vis := domain.VisibilityPrivate
		if s.Public {
			vis = domain.VisibilityPublic
		}
		if err := db.SetMCPServerVisibility(ctx, serverID, vis); err != nil {
			return fmt.Errorf("setting visibility: %w", err)
		}
		logger.Info("bootstrap: created mcp_server", slog.String("slug", s.Slug))
	} else {
		logger.Info("bootstrap: mcp_server already exists, skipping", slog.String("slug", s.Slug))
	}

	// Apply versions (idempotent per-version check inside).
	for _, v := range s.Versions {
		if err := upsertMCPVersion(ctx, db, serverID, v, logger); err != nil {
			return fmt.Errorf("version %q: %w", v.Version, err)
		}
	}

	// Only apply server-level status mutations for newly created servers.
	// Existing servers keep whatever status they already have.
	if created && s.Status == "deprecated" {
		if err := db.SetMCPServerStatus(ctx, serverID, domain.StatusDeprecated); err != nil {
			return fmt.Errorf("deprecating server: %w", err)
		}
	}

	return nil
}

func upsertMCPVersion(ctx context.Context, db *store.DB, serverID string, v MCPVersionSpec, logger *slog.Logger) error {
	// Skip if version already exists.
	var exists bool
	_ = db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM mcp_server_versions WHERE server_id = $1 AND version = $2)`,
		serverID, v.Version,
	).Scan(&exists)
	if exists {
		logger.Info("bootstrap: mcp version already exists, skipping",
			slog.String("server", serverID), slog.String("version", v.Version))
		return nil
	}

	packages, err := json.Marshal(v.Packages)
	if err != nil {
		return fmt.Errorf("marshalling packages: %w", err)
	}
	runtime := deriveRuntime(v.Packages)
	protocolVersion := v.ProtocolVersion
	if protocolVersion == "" {
		protocolVersion = "2025-03-26"
	}

	ver, err := db.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        serverID,
		Version:         v.Version,
		Runtime:         runtime,
		Packages:        packages,
		ProtocolVersion: protocolVersion,
	})
	if err != nil {
		return fmt.Errorf("creating version: %w", err)
	}

	status := strings.ToLower(v.Status)
	switch status {
	case "published", "deprecated":
		if err := db.PublishMCPServerVersion(ctx, serverID, ver.Version); err != nil {
			return fmt.Errorf("publishing version: %w", err)
		}
		if status == "deprecated" {
			if err := db.SetMCPVersionStatus(ctx, serverID, ver.Version,
				domain.VersionStatusDeprecated, v.StatusMessage); err != nil {
				return fmt.Errorf("deprecating version: %w", err)
			}
		}
	case "draft", "":
		// draft is the default — nothing more to do.
	default:
		return fmt.Errorf("unknown version status %q (want draft|published|deprecated)", v.Status)
	}

	logger.Info("bootstrap: created mcp version",
		slog.String("server", serverID),
		slog.String("version", v.Version),
		slog.String("status", status),
	)
	return nil
}

// ── agents ────────────────────────────────────────────────────────────────────

func upsertAgent(ctx context.Context, db *store.DB, publisherID string, a AgentSpec, logger *slog.Logger) error {
	var agentID string
	created := false
	err := db.Pool.QueryRow(ctx,
		`SELECT id FROM agents WHERE publisher_id = $1 AND slug = $2`,
		publisherID, a.Slug,
	).Scan(&agentID)

	if err != nil {
		// Row not found — create it.
		ag, createErr := db.CreateAgent(ctx, store.CreateAgentParams{
			PublisherID: publisherID,
			Slug:        a.Slug,
			Name:        a.Name,
			Description: a.Description,
		})
		if createErr != nil {
			return fmt.Errorf("creating agent: %w", createErr)
		}
		agentID = ag.ID
		created = true

		vis := domain.VisibilityPrivate
		if a.Public {
			vis = domain.VisibilityPublic
		}
		if err := db.SetAgentVisibility(ctx, agentID, vis); err != nil {
			return fmt.Errorf("setting visibility: %w", err)
		}
		logger.Info("bootstrap: created agent", slog.String("slug", a.Slug))
	} else {
		logger.Info("bootstrap: agent already exists, skipping", slog.String("slug", a.Slug))
	}

	// Apply versions (idempotent per-version check inside).
	for _, v := range a.Versions {
		if err := upsertAgentVersion(ctx, db, agentID, v, logger); err != nil {
			return fmt.Errorf("version %q: %w", v.Version, err)
		}
	}

	// Only apply agent-level status mutations for newly created agents.
	// Existing agents keep whatever status they already have.
	if created && a.Status == "deprecated" {
		if err := db.DeprecateAgent(ctx, agentID); err != nil {
			return fmt.Errorf("deprecating agent: %w", err)
		}
	}

	return nil
}

func upsertAgentVersion(ctx context.Context, db *store.DB, agentID string, v AgentVersionSpec, logger *slog.Logger) error {
	var exists bool
	_ = db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM agent_versions WHERE agent_id = $1 AND version = $2)`,
		agentID, v.Version,
	).Scan(&exists)
	if exists {
		logger.Info("bootstrap: agent version already exists, skipping",
			slog.String("agent", agentID), slog.String("version", v.Version))
		return nil
	}

	skills, err := json.Marshal(v.Skills)
	if err != nil {
		return fmt.Errorf("marshalling skills: %w", err)
	}
	auth, err := json.Marshal(v.Authentication)
	if err != nil {
		return fmt.Errorf("marshalling authentication: %w", err)
	}

	protocolVersion := v.ProtocolVersion
	if protocolVersion == "" {
		protocolVersion = domain.A2AProtocolVersion
	}

	ver, err := db.CreateAgentVersion(ctx, store.CreateAgentVersionParams{
		AgentID:            agentID,
		Version:            v.Version,
		EndpointURL:        v.EndpointURL,
		Skills:             skills,
		Authentication:     auth,
		DefaultInputModes:  v.DefaultInputModes,
		DefaultOutputModes: v.DefaultOutputModes,
		DocumentationURL:   v.DocumentationURL,
		IconURL:            v.IconURL,
		ProtocolVersion:    protocolVersion,
	})
	if err != nil {
		return fmt.Errorf("creating version: %w", err)
	}

	status := strings.ToLower(v.Status)
	switch status {
	case "published", "deprecated":
		if err := db.PublishAgentVersion(ctx, agentID, ver.Version); err != nil {
			return fmt.Errorf("publishing version: %w", err)
		}
		if status == "deprecated" {
			if err := db.SetAgentVersionStatus(ctx, agentID, ver.Version,
				domain.VersionStatusDeprecated, v.StatusMessage); err != nil {
				return fmt.Errorf("deprecating version: %w", err)
			}
		}
	case "draft", "":
		// nothing
	default:
		return fmt.Errorf("unknown version status %q (want draft|published|deprecated)", v.Status)
	}

	logger.Info("bootstrap: created agent version",
		slog.String("agent", agentID),
		slog.String("version", v.Version),
		slog.String("status", status),
	)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// deriveRuntime infers the server runtime from the first package's transport type.
func deriveRuntime(packages []PackageSpec) domain.Runtime {
	if len(packages) == 0 {
		return domain.RuntimeStdio
	}
	switch strings.ToLower(packages[0].Transport.Type) {
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

// validateSpec performs basic structural validation of the spec before
// attempting any database operations.
func validateSpec(s *Spec) error {
	var errs []string

	pubSlugs := make(map[string]bool, len(s.Publishers))
	for i, p := range s.Publishers {
		if p.Slug == "" {
			errs = append(errs, fmt.Sprintf("publishers[%d]: slug is required", i))
		}
		if p.Name == "" {
			errs = append(errs, fmt.Sprintf("publishers[%d]: name is required", i))
		}
		pubSlugs[p.Slug] = true
	}

	for i, srv := range s.MCPServers {
		prefix := fmt.Sprintf("mcp_servers[%d](%s)", i, srv.Slug)
		if srv.Publisher == "" {
			errs = append(errs, prefix+": publisher is required")
		} else if !pubSlugs[srv.Publisher] {
			errs = append(errs, prefix+fmt.Sprintf(": publisher %q not found in publishers list", srv.Publisher))
		}
		if srv.Slug == "" {
			errs = append(errs, prefix+": slug is required")
		}
		if srv.Name == "" {
			errs = append(errs, prefix+": name is required")
		}
		for j, v := range srv.Versions {
			if v.Version == "" {
				errs = append(errs, fmt.Sprintf("%s.versions[%d]: version is required", prefix, j))
			}
			if len(v.Packages) == 0 {
				errs = append(errs, fmt.Sprintf("%s.versions[%d]: at least one package is required", prefix, j))
			}
		}
	}

	for i, ag := range s.Agents {
		prefix := fmt.Sprintf("agents[%d](%s)", i, ag.Slug)
		if ag.Publisher == "" {
			errs = append(errs, prefix+": publisher is required")
		} else if !pubSlugs[ag.Publisher] {
			errs = append(errs, prefix+fmt.Sprintf(": publisher %q not found in publishers list", ag.Publisher))
		}
		if ag.Slug == "" {
			errs = append(errs, prefix+": slug is required")
		}
		if ag.Name == "" {
			errs = append(errs, prefix+": name is required")
		}
		for j, v := range ag.Versions {
			if v.Version == "" {
				errs = append(errs, fmt.Sprintf("%s.versions[%d]: version is required", prefix, j))
			}
			if v.EndpointURL == "" {
				errs = append(errs, fmt.Sprintf("%s.versions[%d]: endpoint_url is required", prefix, j))
			}
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}
