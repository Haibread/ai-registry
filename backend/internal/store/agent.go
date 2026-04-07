package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/haibread/ai-registry/internal/domain"
)

// AgentRow is a flat projection of an agent with its publisher namespace.
type AgentRow struct {
	domain.Agent
}

// ListAgentsParams controls filtering and pagination for ListAgents.
type ListAgentsParams struct {
	PublicOnly bool
	Namespace  string
	Query      string
	Limit      int32
	Cursor     string
}

// ListAgents returns a paginated list of agents.
func (db *DB) ListAgents(ctx context.Context, p ListAgentsParams) ([]AgentRow, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	args := []any{}
	argN := 1
	whereClause := "WHERE 1=1"

	if p.PublicOnly {
		whereClause += fmt.Sprintf(" AND a.visibility = $%d", argN)
		args = append(args, "public")
		argN++
	}
	if p.Namespace != "" {
		whereClause += fmt.Sprintf(" AND pub.slug = $%d", argN)
		args = append(args, p.Namespace)
		argN++
	}
	if p.Query != "" {
		whereClause += fmt.Sprintf(
			" AND to_tsvector('english', coalesce(a.name,'') || ' ' || coalesce(a.description,'')) @@ plainto_tsquery('english', $%d)",
			argN,
		)
		args = append(args, p.Query)
		argN++
	}
	if p.Cursor != "" {
		at, id, err := decodeCursor(p.Cursor)
		if err == nil {
			whereClause += fmt.Sprintf(" AND (a.created_at, a.id) > ($%d, $%d)", argN, argN+1)
			args = append(args, at, id)
			argN += 2
		}
	}

	args = append(args, p.Limit)
	q := fmt.Sprintf(`
		SELECT a.id, pub.slug AS namespace, a.publisher_id, a.slug, a.name,
		       coalesce(a.description,''), a.visibility, a.status, a.created_at, a.updated_at
		FROM agents a
		JOIN publishers pub ON pub.id = a.publisher_id
		%s
		ORDER BY a.created_at ASC, a.id ASC
		LIMIT $%d`, whereClause, argN)

	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing agents: %w", err)
	}
	defer rows.Close()

	var result []AgentRow
	for rows.Next() {
		var r AgentRow
		if err := rows.Scan(
			&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
			&r.Description, &r.Visibility, &r.Status, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning agent row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetAgent retrieves a single agent by namespace and slug.
func (db *DB) GetAgent(ctx context.Context, namespace, slug string, publicOnly bool) (*AgentRow, error) {
	q := `
		SELECT a.id, pub.slug, a.publisher_id, a.slug, a.name,
		       coalesce(a.description,''), a.visibility, a.status, a.created_at, a.updated_at
		FROM agents a
		JOIN publishers pub ON pub.id = a.publisher_id
		WHERE pub.slug = $1 AND a.slug = $2`
	args := []any{namespace, slug}
	if publicOnly {
		q += " AND a.visibility = 'public'"
	}

	var r AgentRow
	err := db.Pool.QueryRow(ctx, q, args...).Scan(
		&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
		&r.Description, &r.Visibility, &r.Status, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting agent: %w", err)
	}
	return &r, nil
}

// CreateAgentParams holds the fields needed to insert a new agent.
type CreateAgentParams struct {
	PublisherID string
	Slug        string
	Name        string
	Description string
}

// CreateAgent inserts a new agent (draft, private by default).
func (db *DB) CreateAgent(ctx context.Context, p CreateAgentParams) (*domain.Agent, error) {
	id := NewULID()
	now := time.Now().UTC()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO agents (id, publisher_id, slug, name, description, visibility, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,'private','draft',$6,$6)`,
		id, p.PublisherID, p.Slug, p.Name, p.Description, now,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("creating agent: %w", err)
	}

	return &domain.Agent{
		ID:          id,
		PublisherID: p.PublisherID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		Visibility:  domain.VisibilityPrivate,
		Status:      domain.StatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// ListAgentVersions returns all versions for a given agent ID.
func (db *DB) ListAgentVersions(ctx context.Context, agentID string) ([]domain.AgentVersion, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, agent_id, version, endpoint_url, skills, capabilities, authentication,
		       default_input_modes, default_output_modes, provider,
		       coalesce(documentation_url,''), coalesce(icon_url,''),
		       protocol_version, published_at, released_at
		FROM agent_versions
		WHERE agent_id = $1
		ORDER BY released_at DESC`, agentID)
	if err != nil {
		return nil, fmt.Errorf("listing agent versions: %w", err)
	}
	defer rows.Close()

	var result []domain.AgentVersion
	for rows.Next() {
		v, err := scanAgentVersion(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

// GetAgentVersion retrieves a specific version by agent ID and semver string.
func (db *DB) GetAgentVersion(ctx context.Context, agentID, version string) (*domain.AgentVersion, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, agent_id, version, endpoint_url, skills, capabilities, authentication,
		       default_input_modes, default_output_modes, provider,
		       coalesce(documentation_url,''), coalesce(icon_url,''),
		       protocol_version, published_at, released_at
		FROM agent_versions
		WHERE agent_id = $1 AND version = $2`, agentID, version)

	v, err := scanAgentVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting agent version: %w", err)
	}
	return &v, nil
}

// GetLatestPublishedAgentVersion returns the most recently published version for an agent.
func (db *DB) GetLatestPublishedAgentVersion(ctx context.Context, agentID string) (*domain.AgentVersion, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, agent_id, version, endpoint_url, skills, capabilities, authentication,
		       default_input_modes, default_output_modes, provider,
		       coalesce(documentation_url,''), coalesce(icon_url,''),
		       protocol_version, published_at, released_at
		FROM agent_versions
		WHERE agent_id = $1 AND published_at IS NOT NULL
		ORDER BY published_at DESC
		LIMIT 1`, agentID)

	v, err := scanAgentVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting latest published agent version: %w", err)
	}
	return &v, nil
}

// CreateAgentVersionParams holds the fields needed to insert a new agent version.
type CreateAgentVersionParams struct {
	AgentID            string
	Version            string
	EndpointURL        string
	Skills             json.RawMessage
	Capabilities       json.RawMessage
	Authentication     json.RawMessage
	DefaultInputModes  []string
	DefaultOutputModes []string
	Provider           json.RawMessage
	DocumentationURL   string
	IconURL            string
	ProtocolVersion    string
}

// CreateAgentVersion inserts a new draft agent version.
func (db *DB) CreateAgentVersion(ctx context.Context, p CreateAgentVersionParams) (*domain.AgentVersion, error) {
	if len(p.Capabilities) == 0 {
		p.Capabilities = json.RawMessage("{}")
	}
	if len(p.Authentication) == 0 {
		p.Authentication = json.RawMessage("[]")
	}
	if len(p.DefaultInputModes) == 0 {
		p.DefaultInputModes = []string{"text/plain"}
	}
	if len(p.DefaultOutputModes) == 0 {
		p.DefaultOutputModes = []string{"text/plain"}
	}

	id := NewULID()
	now := time.Now().UTC()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO agent_versions
		    (id, agent_id, version, endpoint_url, skills, capabilities, authentication,
		     default_input_modes, default_output_modes, provider,
		     documentation_url, icon_url, protocol_version, released_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		id, p.AgentID, p.Version, p.EndpointURL,
		p.Skills, p.Capabilities, p.Authentication,
		p.DefaultInputModes, p.DefaultOutputModes, p.Provider,
		p.DocumentationURL, p.IconURL, p.ProtocolVersion, now,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrConflict
		}
		return nil, fmt.Errorf("creating agent version: %w", err)
	}

	return &domain.AgentVersion{
		ID:                 id,
		AgentID:            p.AgentID,
		Version:            p.Version,
		EndpointURL:        p.EndpointURL,
		Skills:             p.Skills,
		Capabilities:       p.Capabilities,
		Authentication:     p.Authentication,
		DefaultInputModes:  p.DefaultInputModes,
		DefaultOutputModes: p.DefaultOutputModes,
		Provider:           p.Provider,
		DocumentationURL:   p.DocumentationURL,
		IconURL:            p.IconURL,
		ProtocolVersion:    p.ProtocolVersion,
		ReleasedAt:         now,
	}, nil
}

// PublishAgentVersion sets published_at on a draft version.
func (db *DB) PublishAgentVersion(ctx context.Context, agentID, version string) error {
	now := time.Now().UTC()
	tag, err := db.Pool.Exec(ctx, `
		UPDATE agent_versions
		SET published_at = $1
		WHERE agent_id = $2 AND version = $3 AND published_at IS NULL`,
		now, agentID, version)
	if err != nil {
		return fmt.Errorf("publishing agent version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var publishedAt *time.Time
		err := db.Pool.QueryRow(ctx,
			`SELECT published_at FROM agent_versions WHERE agent_id=$1 AND version=$2`,
			agentID, version,
		).Scan(&publishedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return ErrImmutable
	}
	_, _ = db.Pool.Exec(ctx,
		`UPDATE agents SET status='published', updated_at=now() WHERE id=$1 AND status='draft'`,
		agentID)
	return nil
}

// DeprecateAgent marks an agent as deprecated.
func (db *DB) DeprecateAgent(ctx context.Context, agentID string) error {
	tag, err := db.Pool.Exec(ctx,
		`UPDATE agents SET status='deprecated', updated_at=now() WHERE id=$1 AND status='published'`,
		agentID)
	if err != nil {
		return fmt.Errorf("deprecating agent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAgentVisibility sets the visibility of an agent.
func (db *DB) SetAgentVisibility(ctx context.Context, agentID string, vis domain.Visibility) error {
	tag, err := db.Pool.Exec(ctx,
		`UPDATE agents SET visibility=$1, updated_at=now() WHERE id=$2`, vis, agentID)
	if err != nil {
		return fmt.Errorf("setting agent visibility: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func scanAgentVersion(s interface{ Scan(...any) error }) (domain.AgentVersion, error) {
	var v domain.AgentVersion
	err := s.Scan(
		&v.ID, &v.AgentID, &v.Version, &v.EndpointURL,
		&v.Skills, &v.Capabilities, &v.Authentication,
		&v.DefaultInputModes, &v.DefaultOutputModes, &v.Provider,
		&v.DocumentationURL, &v.IconURL,
		&v.ProtocolVersion, &v.PublishedAt, &v.ReleasedAt,
	)
	return v, err
}
