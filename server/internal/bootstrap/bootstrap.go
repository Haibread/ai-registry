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
		id, created, err := upsertPublisher(ctx, db, p)
		if err != nil {
			return fmt.Errorf("bootstrap: publisher %q: %w", p.Slug, err)
		}
		pubIDs[p.Slug] = id
		if created {
			// Seed the audit log so the admin /audit page and public
			// activity feeds are populated on a fresh stack. Only on
			// first creation — re-runs must stay idempotent.
			logBootstrapAudit(ctx, db, domain.AuditEvent{
				Action:       domain.ActionPublisherCreated,
				ResourceType: "publisher",
				ResourceID:   id,
				ResourceSlug: p.Slug,
				Metadata: map[string]any{
					"name":     p.Name,
					"verified": p.Verified,
				},
			})
		}
		logger.Info("bootstrap: publisher ready", slog.String("slug", p.Slug))
	}

	// ── Groups ──────────────────────────────────────────────────────
	// Seed registry groups whose slug matches the IdP's group-membership claim,
	// so a federated user's claim groups resolve to the grants below.
	groupIDs := make(map[string]string, len(spec.Groups))
	for _, g := range spec.Groups {
		grp, err := db.EnsureGroupBySlug(ctx, g.Slug, g.Name)
		if err != nil {
			return fmt.Errorf("bootstrap: group %q: %w", g.Slug, err)
		}
		groupIDs[g.Slug] = grp.ID
		logger.Info("bootstrap: group ready", slog.String("slug", g.Slug))
	}

	// ── Grants ──────────────────────────────────────────────────────
	// Idempotent (EnsureGrant); re-applied every boot with source = config, so a
	// grant only disappears when removed from the bootstrap file.
	for i, gr := range spec.Grants {
		if err := upsertGrant(ctx, db, pubIDs, groupIDs, gr); err != nil {
			return fmt.Errorf("bootstrap: grants[%d]: %w", i, err)
		}
	}

	// ── MCP servers ───────────────────────────────────────────────────────────
	for _, s := range spec.MCPServers {
		pubID, ok := pubIDs[s.Publisher]
		if !ok {
			return fmt.Errorf("bootstrap: mcp_server %q references unknown publisher %q", s.Slug, s.Publisher)
		}
		if err := upsertMCPServer(ctx, db, pubID, s.Publisher, s, logger); err != nil {
			return fmt.Errorf("bootstrap: mcp_server %q: %w", s.Slug, err)
		}
	}

	// ── Agents ────────────────────────────────────────────────────────────────
	for _, a := range spec.Agents {
		pubID, ok := pubIDs[a.Publisher]
		if !ok {
			return fmt.Errorf("bootstrap: agent %q references unknown publisher %q", a.Slug, a.Publisher)
		}
		if err := upsertAgent(ctx, db, pubID, a.Publisher, a, logger); err != nil {
			return fmt.Errorf("bootstrap: agent %q: %w", a.Slug, err)
		}
	}

	logger.Info("bootstrap: complete",
		slog.Int("publishers", len(spec.Publishers)),
		slog.Int("groups", len(spec.Groups)),
		slog.Int("grants", len(spec.Grants)),
		slog.Int("mcp_servers", len(spec.MCPServers)),
		slog.Int("agents", len(spec.Agents)),
	)
	return nil
}

// ── groups & grants ─────────────────────────────────────────────────

// upsertGrant idempotently applies one GrantSpec. The group / publisher are
// resolved from the per-run lookup maps first (entities seeded in this same
// file), then from the database (entities seeded earlier — e.g. the reviewer
// group seeded at boot, or a publisher from a prior run). Grants carry
// source = config so they re-apply on every boot.
func upsertGrant(ctx context.Context, db *store.DB, pubIDs, groupIDs map[string]string, g GrantSpec) error {
	var publisherID string
	if g.Publisher != "" {
		id, ok := pubIDs[g.Publisher]
		if !ok {
			pub, err := db.GetPublisher(ctx, g.Publisher)
			if err != nil {
				return fmt.Errorf("references unknown publisher %q: %w", g.Publisher, err)
			}
			id = pub.ID
		}
		publisherID = id
	}

	params := store.CreateGrantParams{
		PublisherID: publisherID,
		Role:        domain.Role(strings.ToLower(g.Role)),
		Source:      domain.GrantSourceConfig,
	}

	switch {
	case g.Group != "":
		gid, ok := groupIDs[g.Group]
		if !ok {
			grp, err := db.GetGroupBySlug(ctx, g.Group)
			if err != nil {
				return fmt.Errorf("references unknown group %q: %w", g.Group, err)
			}
			gid = grp.ID
		}
		params.PrincipalType = domain.PrincipalGroup
		params.PrincipalID = gid
	case g.User != "":
		u, err := db.GetUserByEmail(ctx, g.User)
		if err != nil {
			return fmt.Errorf("references unknown user %q: %w", g.User, err)
		}
		params.PrincipalType = domain.PrincipalUser
		params.PrincipalID = u.ID
	}

	if err := db.EnsureGrant(ctx, params); err != nil {
		return fmt.Errorf("ensuring grant: %w", err)
	}
	return nil
}

// ── audit (bootstrap-synthesized) ─────────────────────────────────────────────

// bootstrapActorSubject / bootstrapActorEmail are the synthetic identity stamped
// onto every audit event the bootstrap loader writes. They're deliberately
// colon-prefixed / .local-suffixed so an admin can distinguish seed events
// from real user actions in the /audit UI, and so they can never collide with
// a Keycloak UUID or real email.
const (
	bootstrapActorSubject = "system:bootstrap"
	bootstrapActorEmail   = "bootstrap@ai-registry.local"
)

// logBootstrapAudit writes a synthetic audit event for a seeded mutation, so
// the /audit page and public activity feed have real rows on a fresh stack.
// We always tag metadata with `source: "bootstrap"` — the public feed's
// metadata allowlist doesn't include "source", so this marker stays
// admin-only while still appearing for operators reviewing the audit log.
func logBootstrapAudit(ctx context.Context, db *store.DB, e domain.AuditEvent) {
	if e.ActorSubject == "" {
		e.ActorSubject = bootstrapActorSubject
	}
	if e.ActorEmail == "" {
		e.ActorEmail = bootstrapActorEmail
	}
	if e.Metadata == nil {
		e.Metadata = map[string]any{}
	}
	if _, ok := e.Metadata["source"]; !ok {
		e.Metadata["source"] = "bootstrap"
	}
	db.LogAuditEvent(ctx, e)
}

// ── publishers ────────────────────────────────────────────────────────────────

func upsertPublisher(ctx context.Context, db *store.DB, p PublisherSpec) (string, bool, error) {
	id := store.NewULID()
	verified := p.Verified
	tag, err := db.Pool.Exec(ctx,
		`INSERT INTO publishers (id, slug, name, verified, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, NOW(), NOW())
		 ON CONFLICT (slug) DO NOTHING`,
		id, p.Slug, p.Name, verified,
	)
	if err != nil {
		return "", false, fmt.Errorf("upserting publisher: %w", err)
	}
	created := tag.RowsAffected() > 0
	// Fetch the real ID (may differ if the row already existed).
	var realID string
	if err := db.Pool.QueryRow(ctx, `SELECT id FROM publishers WHERE slug = $1`, p.Slug).Scan(&realID); err != nil {
		return "", false, fmt.Errorf("fetching publisher id: %w", err)
	}
	return realID, created, nil
}

// ── MCP servers ───────────────────────────────────────────────────────────────

func upsertMCPServer(ctx context.Context, db *store.DB, publisherID, publisherSlug string, s MCPServerSpec, logger *slog.Logger) error {
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

		// Seed the audit log — mirrors what the real admin create handler
		// writes, so a fresh stack's /audit page and per-entry activity
		// feed have something to show.
		logBootstrapAudit(ctx, db, domain.AuditEvent{
			Action:       domain.ActionMCPServerCreated,
			ResourceType: "mcp_server",
			ResourceID:   serverID,
			ResourceNS:   publisherSlug,
			ResourceSlug: s.Slug,
			Metadata: map[string]any{
				"name": s.Name,
			},
		})

		vis := domain.VisibilityPrivate
		if s.Public {
			vis = domain.VisibilityPublic
		}
		if err := db.SetMCPServerVisibility(ctx, serverID, vis); err != nil {
			return fmt.Errorf("setting visibility: %w", err)
		}
		if s.Public {
			// Servers are created private; flipping to public is a
			// distinct audit event.
			logBootstrapAudit(ctx, db, domain.AuditEvent{
				Action:       domain.ActionMCPServerVisibility,
				ResourceType: "mcp_server",
				ResourceID:   serverID,
				ResourceNS:   publisherSlug,
				ResourceSlug: s.Slug,
				Metadata: map[string]any{
					"from": string(domain.VisibilityPrivate),
					"to":   string(domain.VisibilityPublic),
				},
			})
		}
		// Apply v0.2 metadata fields (featured / verified / tags / readme)
		// via direct SQL — the CreateMCPServer helper predates these columns.
		if s.Featured || s.Verified || len(s.Tags) > 0 || s.Readme != "" {
			tags := s.Tags
			if tags == nil {
				tags = []string{}
			}
			if _, err := db.Pool.Exec(ctx,
				`UPDATE mcp_servers
				 SET featured=$1, verified=$2, tags=$3, readme=$4, updated_at=now()
				 WHERE id=$5`,
				s.Featured, s.Verified, tags, s.Readme, serverID,
			); err != nil {
				return fmt.Errorf("setting mcp server metadata: %w", err)
			}
		}
		logger.Info("bootstrap: created mcp_server", slog.String("slug", s.Slug))
	} else {
		logger.Info("bootstrap: mcp_server already exists, skipping", slog.String("slug", s.Slug))
	}

	// Apply versions (idempotent per-version check inside).
	for _, v := range s.Versions {
		if err := upsertMCPVersion(ctx, db, serverID, publisherSlug, s.Slug, v, logger); err != nil {
			return fmt.Errorf("version %q: %w", v.Version, err)
		}
	}

	// Only apply server-level status mutations for newly created servers.
	// Existing servers keep whatever status they already have.
	if created && s.Status == "deprecated" {
		if err := db.SetMCPServerStatus(ctx, serverID, domain.StatusDeprecated); err != nil {
			return fmt.Errorf("deprecating server: %w", err)
		}
		logBootstrapAudit(ctx, db, domain.AuditEvent{
			Action:       domain.ActionMCPServerDeprecated,
			ResourceType: "mcp_server",
			ResourceID:   serverID,
			ResourceNS:   publisherSlug,
			ResourceSlug: s.Slug,
		})
	}

	return nil
}

func upsertMCPVersion(ctx context.Context, db *store.DB, serverID, publisherSlug, serverSlug string, v MCPVersionSpec, logger *slog.Logger) error {
	// Tools: marshal the publisher-declared list (possibly empty) and run
	// it through domain.ValidateTools so bootstrap catches structural
	// mistakes at load time rather than letting the UI render garbage.
	// Marshaled up front so both the "create new version" path and the
	// "backfill tools on existing version" path can use the same bytes.
	var tools json.RawMessage
	if v.Tools != nil {
		var err error
		tools, err = json.Marshal(v.Tools)
		if err != nil {
			return fmt.Errorf("marshalling tools: %w", err)
		}
		if err := domain.ValidateTools(tools); err != nil {
			return fmt.Errorf("validating tools: %w", err)
		}
	}

	// Does the version already exist?
	var exists bool
	_ = db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM mcp_server_versions WHERE server_id = $1 AND version = $2)`,
		serverID, v.Version,
	).Scan(&exists)
	if exists {
		// Existing version: normally immutable, but we allow a narrow
		// backfill for `tools` when the stored array is empty and the
		// spec now declares a non-empty list. This lets a stack that was
		// seeded before the tools field existed catch up on the next
		// bootstrap run without wiping the database. We do NOT touch
		// packages, runtime, capabilities, or protocol_version — those
		// stay frozen to preserve the MCP publish-immutability contract.
		if len(v.Tools) > 0 {
			var current []byte
			if err := db.Pool.QueryRow(ctx,
				`SELECT tools FROM mcp_server_versions WHERE server_id = $1 AND version = $2`,
				serverID, v.Version,
			).Scan(&current); err == nil && isEmptyJSONArray(current) {
				if _, err := db.Pool.Exec(ctx,
					`UPDATE mcp_server_versions SET tools = $1, updated_at = now()
					 WHERE server_id = $2 AND version = $3`,
					tools, serverID, v.Version,
				); err != nil {
					return fmt.Errorf("backfilling tools: %w", err)
				}
				logger.Info("bootstrap: backfilled mcp version tools",
					slog.String("server", serverID),
					slog.String("version", v.Version),
					slog.Int("tool_count", len(v.Tools)))
				return nil
			}
		}
		logger.Info("bootstrap: mcp version already exists, skipping",
			slog.String("server", serverID), slog.String("version", v.Version))
		return nil
	}

	packages, err := json.Marshal(v.Packages)
	if err != nil {
		return fmt.Errorf("marshalling packages: %w", err)
	}
	var remotes json.RawMessage
	if len(v.Remotes) > 0 {
		remotes, err = json.Marshal(v.Remotes)
		if err != nil {
			return fmt.Errorf("marshalling remotes: %w", err)
		}
	}
	runtime := deriveRuntime(v.Packages, v.Remotes)
	protocolVersion := v.ProtocolVersion
	if protocolVersion == "" {
		protocolVersion = "2025-03-26"
	}

	var capabilities json.RawMessage
	if len(v.Capabilities) > 0 {
		capabilities, err = json.Marshal(v.Capabilities)
		if err != nil {
			return fmt.Errorf("marshalling capabilities: %w", err)
		}
	}

	ver, err := db.CreateMCPServerVersion(ctx, store.CreateMCPServerVersionParams{
		ServerID:        serverID,
		Version:         v.Version,
		Runtime:         runtime,
		Packages:        packages,
		Remotes:         remotes,
		Capabilities:    capabilities,
		Tools:           tools,
		ProtocolVersion: protocolVersion,
	})
	if err != nil {
		return fmt.Errorf("creating version: %w", err)
	}

	// Draft creation audit event, mirroring what the real create-version
	// handler writes. Kept out of the publicActionWhitelist in the public
	// activity handler — drafts shouldn't leak to unauthenticated viewers.
	logBootstrapAudit(ctx, db, domain.AuditEvent{
		Action:       domain.ActionMCPVersionCreated,
		ResourceType: "mcp_server",
		ResourceID:   serverID,
		ResourceNS:   publisherSlug,
		ResourceSlug: serverSlug,
		Metadata: map[string]any{
			"version": ver.Version,
		},
	})

	status := strings.ToLower(v.Status)
	switch status {
	case "published", "deprecated":
		if err := db.PublishMCPServerVersion(ctx, serverID, ver.Version); err != nil {
			return fmt.Errorf("publishing version: %w", err)
		}
		logBootstrapAudit(ctx, db, domain.AuditEvent{
			Action:       domain.ActionMCPVersionPublished,
			ResourceType: "mcp_server",
			ResourceID:   serverID,
			ResourceNS:   publisherSlug,
			ResourceSlug: serverSlug,
			Metadata: map[string]any{
				"version": ver.Version,
			},
		})
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

func upsertAgent(ctx context.Context, db *store.DB, publisherID, publisherSlug string, a AgentSpec, logger *slog.Logger) error {
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

		logBootstrapAudit(ctx, db, domain.AuditEvent{
			Action:       domain.ActionAgentCreated,
			ResourceType: "agent",
			ResourceID:   agentID,
			ResourceNS:   publisherSlug,
			ResourceSlug: a.Slug,
			Metadata: map[string]any{
				"name": a.Name,
			},
		})

		vis := domain.VisibilityPrivate
		if a.Public {
			vis = domain.VisibilityPublic
		}
		if err := db.SetAgentVisibility(ctx, agentID, vis); err != nil {
			return fmt.Errorf("setting visibility: %w", err)
		}
		if a.Public {
			logBootstrapAudit(ctx, db, domain.AuditEvent{
				Action:       domain.ActionAgentVisibility,
				ResourceType: "agent",
				ResourceID:   agentID,
				ResourceNS:   publisherSlug,
				ResourceSlug: a.Slug,
				Metadata: map[string]any{
					"from": string(domain.VisibilityPrivate),
					"to":   string(domain.VisibilityPublic),
				},
			})
		}
		// Apply v0.2 metadata fields (featured / verified / tags / readme)
		// via direct SQL — the CreateAgent helper predates these columns.
		if a.Featured || a.Verified || len(a.Tags) > 0 || a.Readme != "" {
			tags := a.Tags
			if tags == nil {
				tags = []string{}
			}
			if _, err := db.Pool.Exec(ctx,
				`UPDATE agents
				 SET featured=$1, verified=$2, tags=$3, readme=$4, updated_at=now()
				 WHERE id=$5`,
				a.Featured, a.Verified, tags, a.Readme, agentID,
			); err != nil {
				return fmt.Errorf("setting agent metadata: %w", err)
			}
		}
		logger.Info("bootstrap: created agent", slog.String("slug", a.Slug))
	} else {
		logger.Info("bootstrap: agent already exists, skipping", slog.String("slug", a.Slug))
	}

	// Apply versions (idempotent per-version check inside).
	for _, v := range a.Versions {
		if err := upsertAgentVersion(ctx, db, agentID, publisherSlug, a.Slug, v, logger); err != nil {
			return fmt.Errorf("version %q: %w", v.Version, err)
		}
	}

	// Only apply agent-level status mutations for newly created agents.
	// Existing agents keep whatever status they already have.
	if created && a.Status == "deprecated" {
		if err := db.DeprecateAgent(ctx, agentID); err != nil {
			return fmt.Errorf("deprecating agent: %w", err)
		}
		logBootstrapAudit(ctx, db, domain.AuditEvent{
			Action:       domain.ActionAgentDeprecated,
			ResourceType: "agent",
			ResourceID:   agentID,
			ResourceNS:   publisherSlug,
			ResourceSlug: a.Slug,
		})
	}

	return nil
}

func upsertAgentVersion(ctx context.Context, db *store.DB, agentID, publisherSlug, agentSlug string, v AgentVersionSpec, logger *slog.Logger) error {
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

	logBootstrapAudit(ctx, db, domain.AuditEvent{
		Action:       domain.ActionAgentVersionCreated,
		ResourceType: "agent",
		ResourceID:   agentID,
		ResourceNS:   publisherSlug,
		ResourceSlug: agentSlug,
		Metadata: map[string]any{
			"version": ver.Version,
		},
	})

	status := strings.ToLower(v.Status)
	switch status {
	case "published", "deprecated":
		if err := db.PublishAgentVersion(ctx, agentID, ver.Version); err != nil {
			return fmt.Errorf("publishing version: %w", err)
		}
		logBootstrapAudit(ctx, db, domain.AuditEvent{
			Action:       domain.ActionAgentVersionPublished,
			ResourceType: "agent",
			ResourceID:   agentID,
			ResourceNS:   publisherSlug,
			ResourceSlug: agentSlug,
			Metadata: map[string]any{
				"version": ver.Version,
			},
		})
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

// isEmptyJSONArray returns true if raw is JSON-equivalent to an empty array.
// Accepts `null`, `[]`, and `[ ]` (with interior whitespace). Used by the
// tools-backfill path in upsertMCPVersion to decide whether it's safe to
// overwrite the stored value.
func isEmptyJSONArray(raw []byte) bool {
	if len(raw) == 0 {
		return true
	}
	var v []json.RawMessage
	if err := json.Unmarshal(raw, &v); err != nil {
		// Not an array at all — don't touch it.
		return false
	}
	return len(v) == 0
}

// deriveRuntime infers the server runtime from the first package's transport
// type, falling back to the first remote's type for hosted-only servers.
func deriveRuntime(packages []PackageSpec, remotes []RemoteSpec) domain.Runtime {
	transport := ""
	switch {
	case len(packages) > 0:
		transport = packages[0].Transport.Type
	case len(remotes) > 0:
		transport = remotes[0].Type
	}
	switch strings.ToLower(transport) {
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

	// Lookup tables intentionally skip rows that already failed their
	// own field-level checks — keeping `""` keys in either map would let
	// downstream "not found" assertions pass against rubble and emit
	// confusing follow-up errors instead of the real "slug is required"
	// for the offending row.
	pubSlugs := make(map[string]bool, len(s.Publishers))
	for i, p := range s.Publishers {
		if p.Slug == "" {
			errs = append(errs, fmt.Sprintf("publishers[%d]: slug is required", i))
		}
		if p.Name == "" {
			errs = append(errs, fmt.Sprintf("publishers[%d]: name is required", i))
		}
		if p.Slug != "" {
			pubSlugs[p.Slug] = true
		}
	}

	for i, g := range s.Groups {
		if g.Slug == "" {
			errs = append(errs, fmt.Sprintf("groups[%d]: slug is required", i))
		}
		if g.Name == "" {
			errs = append(errs, fmt.Sprintf("groups[%d]: name is required", i))
		}
	}

	// Grants reference a group/user and a publisher that may live in this file
	// OR already exist in the registry (the reviewer group, a prior-run
	// publisher, a provisioned user). Existence is therefore resolved at run
	// time; here we only enforce the structural shape.
	for i, gr := range s.Grants {
		prefix := fmt.Sprintf("grants[%d]", i)
		if (gr.Group != "") == (gr.User != "") {
			errs = append(errs, prefix+": exactly one of group or user is required")
		}
		if !domain.ValidRole(strings.ToLower(gr.Role)) {
			errs = append(errs, prefix+fmt.Sprintf(": invalid role %q (want viewer|editor|reviewer|admin)", gr.Role))
		}
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
			if len(v.Packages) == 0 && len(v.Remotes) == 0 {
				errs = append(errs, fmt.Sprintf("%s.versions[%d]: at least one package or remote is required", prefix, j))
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
