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

// LatestAgentVersion is a summary of the most recently published agent version,
// embedded inline in list and detail responses.
type LatestAgentVersion struct {
	Version            string
	EndpointURL        string
	Skills             json.RawMessage
	DefaultInputModes  []string
	DefaultOutputModes []string
	Authentication     json.RawMessage
	ProtocolVersion    string
	PublishedAt        *time.Time
}

// AgentRow is a flat projection of an agent with its publisher namespace.
type AgentRow struct {
	domain.Agent
	LatestVersion *LatestAgentVersion
}

// ListAgentsParams controls filtering and pagination for ListAgents.
type ListAgentsParams struct {
	PublicOnly bool
	Namespace  string
	Status     string // filter by status: "draft" | "published" | "deprecated" | "" (all)
	Visibility string // filter by visibility: "public" | "private" | "" (all); only meaningful when PublicOnly=false
	Query      string
	Limit      int32
	Cursor     string
	Sort           string     // sort order: "created_at_desc" (default), "updated_at_desc", "published_at_desc", "name_asc", "name_desc"
	Featured       *bool      // when non-nil, filter by featured flag
	Tag            string     // when non-empty, filter to agents that contain this tag
	PublishedSince *time.Time // when non-nil, only entries whose latest version was published after this time
}

// ListAgents returns a paginated list of agents and the total count of rows
// that match the filters (before pagination).
func (db *DB) ListAgents(ctx context.Context, p ListAgentsParams) ([]AgentRow, int, error) {
	ctx, span := startSpan(ctx, "ListAgents")
	defer span.End()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	args := []any{}
	argN := 1
	countArgN := 1
	filterWhere := "WHERE 1=1"
	filterArgs := []any{}

	if p.PublicOnly {
		filterWhere += fmt.Sprintf(" AND a.visibility = $%d", argN)
		filterArgs = append(filterArgs, "public")
		argN++
		countArgN++
	} else if p.Visibility != "" {
		filterWhere += fmt.Sprintf(" AND a.visibility = $%d", argN)
		filterArgs = append(filterArgs, p.Visibility)
		argN++
		countArgN++
	}
	if p.Status != "" {
		filterWhere += fmt.Sprintf(" AND a.status = $%d", argN)
		filterArgs = append(filterArgs, p.Status)
		argN++
		countArgN++
	}
	if p.Namespace != "" {
		filterWhere += fmt.Sprintf(" AND pub.slug = $%d", argN)
		filterArgs = append(filterArgs, p.Namespace)
		argN++
		countArgN++
	}
	if p.Featured != nil {
		filterWhere += fmt.Sprintf(" AND a.featured = $%d", argN)
		filterArgs = append(filterArgs, *p.Featured)
		argN++
		countArgN++
	}
	if p.Tag != "" {
		filterWhere += fmt.Sprintf(" AND $%d = ANY(a.tags)", argN)
		filterArgs = append(filterArgs, p.Tag)
		argN++
		countArgN++
	}

	// Post-join filters (reference lav.* columns only available after lateral join).
	var postJoinFilter string
	var postJoinFilterCount string
	if p.PublishedSince != nil {
		postJoinFilter += fmt.Sprintf(" AND lav.published_at > $%d", argN)
		postJoinFilterCount += fmt.Sprintf(" AND lav.published_at > $%d", countArgN)
		filterArgs = append(filterArgs, *p.PublishedSince)
		argN++
		countArgN++
	}

	tsQuery := prefixTSQuery(p.Query)
	hasQuery := tsQuery != ""
	if hasQuery {
		filterWhere += fmt.Sprintf(
			" AND a.search_vector @@ to_tsquery('english', $%d)",
			argN,
		)
		filterArgs = append(filterArgs, tsQuery)
		argN++
		countArgN++
	}

	// Snapshot filterArgs before cursor / ORDER-BY args so the COUNT query
	// can reuse just the filter portion.
	countArgs := make([]any, len(filterArgs))
	copy(countArgs, filterArgs)

	args = append(args, filterArgs...)

	// Cursor (keyset pagination) — added to the main query only.
	whereClause := filterWhere
	if !hasQuery && p.Cursor != "" {
		at, id, err := decodeCursor(p.Cursor)
		if err != nil {
			return nil, 0, ErrInvalidCursor
		}
		cursorCol := "a.created_at"
		switch p.Sort {
		case "updated_at_desc":
			cursorCol = "a.updated_at"
		case "published_at_desc":
			cursorCol = "lav.published_at"
		}
		// For name-based sorts, cursor is not supported (cursor encodes a
		// timestamp, not a name). Silently ignore the cursor.
		if p.Sort != "name_asc" && p.Sort != "name_desc" {
			whereClause += fmt.Sprintf(" AND (%s, a.id) < ($%d, $%d)", cursorCol, argN, argN+1)
			args = append(args, at, id)
			argN += 2
		}
	}

	orderClause := "ORDER BY a.created_at DESC, a.id DESC"
	if hasQuery {
		orderClause = fmt.Sprintf(
			"ORDER BY ts_rank(a.search_vector, to_tsquery('english', $%d)) DESC, a.created_at DESC",
			argN,
		)
		args = append(args, tsQuery)
		argN++
	} else {
		switch p.Sort {
		case "updated_at_desc":
			orderClause = "ORDER BY a.updated_at DESC, a.id DESC"
		case "published_at_desc":
			orderClause = "ORDER BY lav.published_at DESC NULLS LAST, a.id DESC"
		case "name_asc":
			orderClause = "ORDER BY a.name ASC, a.id ASC"
		case "name_desc":
			orderClause = "ORDER BY a.name DESC, a.id DESC"
		}
	}

	// Apply post-join filters to both main and count WHERE clauses.
	if postJoinFilter != "" {
		whereClause += postJoinFilter
		filterWhere += postJoinFilterCount
	}

	args = append(args, p.Limit)
	q := fmt.Sprintf(`
		SELECT a.id, pub.slug AS namespace, a.publisher_id, a.slug, a.name,
		       coalesce(a.description,''), a.visibility, a.status, a.featured, a.verified, a.tags,
		       coalesce(a.readme,''), a.view_count, a.copy_count, a.created_at, a.updated_at,
		       lav.version, lav.endpoint_url, lav.skills, lav.default_input_modes,
		       lav.default_output_modes, lav.authentication, lav.protocol_version, lav.published_at
		FROM agents a
		JOIN publishers pub ON pub.id = a.publisher_id
		LEFT JOIN LATERAL (
		    SELECT av.version, av.endpoint_url, av.skills, av.default_input_modes,
		           av.default_output_modes, av.authentication, av.protocol_version, av.published_at
		    FROM agent_versions av
		    WHERE av.agent_id = a.id AND av.published_at IS NOT NULL
		    ORDER BY av.published_at DESC
		    LIMIT 1
		) lav ON true
		%s
		%s
		LIMIT $%d`, whereClause, orderClause, argN)

	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		recordErr(span, err)
		return nil, 0, fmt.Errorf("listing agents: %w", err)
	}
	defer rows.Close()

	var result []AgentRow
	for rows.Next() {
		var r AgentRow
		var (
			lavVersion     *string
			lavEndpoint    *string
			lavSkills      []byte
			lavInputModes  []string
			lavOutputModes []string
			lavAuth        []byte
			lavProto       *string
			lavPublishedAt *time.Time
		)
		if err := rows.Scan(
			&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
			&r.Description, &r.Visibility, &r.Status, &r.Featured, &r.Verified, &r.Tags,
			&r.Readme, &r.ViewCount, &r.CopyCount, &r.CreatedAt, &r.UpdatedAt,
			&lavVersion, &lavEndpoint, &lavSkills, &lavInputModes,
			&lavOutputModes, &lavAuth, &lavProto, &lavPublishedAt,
		); err != nil {
			recordErr(span, err)
			return nil, 0, fmt.Errorf("scanning agent row: %w", err)
		}
		if lavVersion != nil {
			r.LatestVersion = &LatestAgentVersion{
				Version:            *lavVersion,
				EndpointURL:        *lavEndpoint,
				Skills:             json.RawMessage(lavSkills),
				DefaultInputModes:  lavInputModes,
				DefaultOutputModes: lavOutputModes,
				Authentication:     json.RawMessage(lavAuth),
				ProtocolVersion:    *lavProto,
				PublishedAt:        lavPublishedAt,
			}
		}
		result = append(result, r)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, 0, err
	}

	// Separate COUNT query using the same filter conditions but without
	// cursor / ORDER-BY so it reflects the full matching set.
	// When post-join filters are present (e.g. published_since), the count
	// query must include the lateral join so those conditions can reference lav.*.
	var countQ string
	if postJoinFilterCount != "" {
		countQ = fmt.Sprintf(`
			SELECT COUNT(*)
			FROM agents a
			JOIN publishers pub ON pub.id = a.publisher_id
			LEFT JOIN LATERAL (
			    SELECT av.published_at
			    FROM agent_versions av
			    WHERE av.agent_id = a.id AND av.published_at IS NOT NULL
			    ORDER BY av.published_at DESC
			    LIMIT 1
			) lav ON true
			%s`, filterWhere)
	} else {
		countQ = fmt.Sprintf(`
			SELECT COUNT(*)
			FROM agents a
			JOIN publishers pub ON pub.id = a.publisher_id
			%s`, filterWhere)
	}

	var total int
	if err := db.Pool.QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		recordErr(span, err)
		return nil, 0, fmt.Errorf("counting agents: %w", err)
	}

	return result, total, nil
}

// GetAgent retrieves a single agent by namespace and slug.
func (db *DB) GetAgent(ctx context.Context, namespace, slug string, publicOnly bool) (*AgentRow, error) {
	ctx, span := startSpan(ctx, "GetAgent")
	defer span.End()

	q := `
		SELECT a.id, pub.slug, a.publisher_id, a.slug, a.name,
		       coalesce(a.description,''), a.visibility, a.status, a.featured, a.verified, a.tags,
		       coalesce(a.readme,''), a.view_count, a.copy_count, a.created_at, a.updated_at,
		       lav.version, lav.endpoint_url, lav.skills, lav.default_input_modes,
		       lav.default_output_modes, lav.authentication, lav.protocol_version, lav.published_at
		FROM agents a
		JOIN publishers pub ON pub.id = a.publisher_id
		LEFT JOIN LATERAL (
		    SELECT av.version, av.endpoint_url, av.skills, av.default_input_modes,
		           av.default_output_modes, av.authentication, av.protocol_version, av.published_at
		    FROM agent_versions av
		    WHERE av.agent_id = a.id AND av.published_at IS NOT NULL
		    ORDER BY av.published_at DESC
		    LIMIT 1
		) lav ON true
		WHERE pub.slug = $1 AND a.slug = $2`
	args := []any{namespace, slug}
	if publicOnly {
		q += " AND a.visibility = 'public'"
	}

	var r AgentRow
	var (
		lavVersion     *string
		lavEndpoint    *string
		lavSkills      []byte
		lavInputModes  []string
		lavOutputModes []string
		lavAuth        []byte
		lavProto       *string
		lavPublishedAt *time.Time
	)
	err := db.Pool.QueryRow(ctx, q, args...).Scan(
		&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
		&r.Description, &r.Visibility, &r.Status, &r.Featured, &r.Verified, &r.Tags,
		&r.Readme, &r.ViewCount, &r.CopyCount, &r.CreatedAt, &r.UpdatedAt,
		&lavVersion, &lavEndpoint, &lavSkills, &lavInputModes,
		&lavOutputModes, &lavAuth, &lavProto, &lavPublishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting agent: %w", err)
	}
	if lavVersion != nil {
		r.LatestVersion = &LatestAgentVersion{
			Version:            *lavVersion,
			EndpointURL:        *lavEndpoint,
			Skills:             json.RawMessage(lavSkills),
			DefaultInputModes:  lavInputModes,
			DefaultOutputModes: lavOutputModes,
			Authentication:     json.RawMessage(lavAuth),
			ProtocolVersion:    *lavProto,
			PublishedAt:        lavPublishedAt,
		}
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
	ctx, span := startSpan(ctx, "CreateAgent")
	defer span.End()

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
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
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
	ctx, span := startSpan(ctx, "ListAgentVersions")
	defer span.End()

	rows, err := db.Pool.Query(ctx, `
		SELECT id, agent_id, version, endpoint_url, skills, capabilities, authentication,
		       default_input_modes, default_output_modes, provider,
		       coalesce(documentation_url,''), coalesce(icon_url,''),
		       protocol_version, status, coalesce(status_message,''), status_changed_at, published_at, created_at, updated_at
		FROM agent_versions
		WHERE agent_id = $1
		ORDER BY created_at DESC`, agentID)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing agent versions: %w", err)
	}
	defer rows.Close()

	var result []domain.AgentVersion
	for rows.Next() {
		v, err := scanAgentVersion(rows)
		if err != nil {
			recordErr(span, err)
			return nil, err
		}
		result = append(result, v)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// GetAgentVersion retrieves a specific version by agent ID and semver string.
func (db *DB) GetAgentVersion(ctx context.Context, agentID, version string) (*domain.AgentVersion, error) {
	ctx, span := startSpan(ctx, "GetAgentVersion")
	defer span.End()

	row := db.Pool.QueryRow(ctx, `
		SELECT id, agent_id, version, endpoint_url, skills, capabilities, authentication,
		       default_input_modes, default_output_modes, provider,
		       coalesce(documentation_url,''), coalesce(icon_url,''),
		       protocol_version, status, coalesce(status_message,''), status_changed_at, published_at, created_at, updated_at
		FROM agent_versions
		WHERE agent_id = $1 AND version = $2`, agentID, version)

	v, err := scanAgentVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting agent version: %w", err)
	}
	return &v, nil
}

// GetLatestPublishedAgentVersion returns the most recently published version for an agent.
func (db *DB) GetLatestPublishedAgentVersion(ctx context.Context, agentID string) (*domain.AgentVersion, error) {
	ctx, span := startSpan(ctx, "GetLatestPublishedAgentVersion")
	defer span.End()

	row := db.Pool.QueryRow(ctx, `
		SELECT id, agent_id, version, endpoint_url, skills, capabilities, authentication,
		       default_input_modes, default_output_modes, provider,
		       coalesce(documentation_url,''), coalesce(icon_url,''),
		       protocol_version, status, coalesce(status_message,''), status_changed_at, published_at, created_at, updated_at
		FROM agent_versions
		WHERE agent_id = $1 AND published_at IS NOT NULL
		ORDER BY published_at DESC
		LIMIT 1`, agentID)

	v, err := scanAgentVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
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
	ctx, span := startSpan(ctx, "CreateAgentVersion")
	defer span.End()

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
		     documentation_url, icon_url, protocol_version)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		id, p.AgentID, p.Version, p.EndpointURL,
		p.Skills, p.Capabilities, p.Authentication,
		p.DefaultInputModes, p.DefaultOutputModes, p.Provider,
		p.DocumentationURL, p.IconURL, p.ProtocolVersion,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
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
		Status:             domain.VersionStatusActive,
		StatusChangedAt:    now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}, nil
}

// PublishAgentVersion sets published_at on a draft version.
func (db *DB) PublishAgentVersion(ctx context.Context, agentID, version string) error {
	ctx, span := startSpan(ctx, "PublishAgentVersion")
	defer span.End()

	now := time.Now().UTC()
	tag, err := db.Pool.Exec(ctx, `
		UPDATE agent_versions
		SET published_at = $1
		WHERE agent_id = $2 AND version = $3 AND published_at IS NULL`,
		now, agentID, version)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("publishing agent version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var publishedAt *time.Time
		err := db.Pool.QueryRow(ctx,
			`SELECT published_at FROM agent_versions WHERE agent_id=$1 AND version=$2`,
			agentID, version,
		).Scan(&publishedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			recordErr(span, ErrNotFound)
			return ErrNotFound
		}
		recordErr(span, ErrImmutable)
		return ErrImmutable
	}
	_, _ = db.Pool.Exec(ctx,
		`UPDATE agents SET status='published', updated_at=now() WHERE id=$1 AND status='draft'`,
		agentID)
	return nil
}

// DeprecateAgent marks an agent as deprecated.
func (db *DB) DeprecateAgent(ctx context.Context, agentID string) error {
	ctx, span := startSpan(ctx, "DeprecateAgent")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE agents SET status='deprecated', updated_at=now() WHERE id=$1 AND status='published'`,
		agentID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deprecating agent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// SetAgentVisibility sets the visibility of an agent.
func (db *DB) SetAgentVisibility(ctx context.Context, agentID string, vis domain.Visibility) error {
	ctx, span := startSpan(ctx, "SetAgentVisibility")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE agents SET visibility=$1, updated_at=now() WHERE id=$2`, vis, agentID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("setting agent visibility: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// SetAgentVersionStatus updates the lifecycle status of a specific agent version.
// statusMessage is optional and should be empty when status is "active".
func (db *DB) SetAgentVersionStatus(ctx context.Context, agentID, version string, status domain.VersionStatus, statusMessage string) error {
	ctx, span := startSpan(ctx, "SetAgentVersionStatus")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agent_versions
		SET status=$1, status_message=$2, status_changed_at=now()
		WHERE agent_id=$3 AND version=$4`,
		status, statusMessage, agentID, version)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("setting agent version status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// SetAllAgentVersionsStatus updates the lifecycle status on all published versions of
// an agent atomically. Returns the updated versions.
func (db *DB) SetAllAgentVersionsStatus(ctx context.Context, agentID string, status domain.VersionStatus, statusMessage string) ([]domain.AgentVersion, error) {
	ctx, span := startSpan(ctx, "SetAllAgentVersionsStatus")
	defer span.End()

	rows, err := db.Pool.Query(ctx, `
		UPDATE agent_versions
		SET status=$1, status_message=$2, status_changed_at=now()
		WHERE agent_id=$3 AND published_at IS NOT NULL
		RETURNING id, agent_id, version, endpoint_url, skills, capabilities, authentication,
		          default_input_modes, default_output_modes, provider,
		          coalesce(documentation_url,''), coalesce(icon_url,''),
		          protocol_version, status, coalesce(status_message,''), status_changed_at,
		          published_at, created_at`,
		status, statusMessage, agentID)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("setting all agent versions status: %w", err)
	}
	defer rows.Close()

	var result []domain.AgentVersion
	for rows.Next() {
		v, err := scanAgentVersion(rows)
		if err != nil {
			recordErr(span, err)
			return nil, err
		}
		result = append(result, v)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// UpdateAgentParams holds the mutable fields for a PATCH operation on an agent.
type UpdateAgentParams struct {
	Name        string
	Description string
}

// UpdateAgent updates the mutable metadata fields of an agent.
// Returns ErrNotFound if the agent does not exist.
func (db *DB) UpdateAgent(ctx context.Context, agentID string, p UpdateAgentParams) (*AgentRow, error) {
	ctx, span := startSpan(ctx, "UpdateAgent")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agents
		SET name=$1, description=$2, updated_at=now()
		WHERE id=$3 AND status != 'deleted'`,
		p.Name, p.Description, agentID,
	)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("updating agent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	return db.getAgentByID(ctx, agentID)
}

// DeleteAgent soft-deletes an agent by setting status='deleted' on the agent
// and all its versions. Returns ErrNotFound if not found.
func (db *DB) DeleteAgent(ctx context.Context, agentID string) error {
	ctx, span := startSpan(ctx, "DeleteAgent")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE agents SET status='deleted', updated_at=now() WHERE id=$1 AND status != 'deleted'`,
		agentID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting agent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	_, err = db.Pool.Exec(ctx,
		`UPDATE agent_versions SET status='deleted', status_changed_at=now() WHERE agent_id=$1`,
		agentID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting agent versions: %w", err)
	}
	return nil
}

// getAgentByID retrieves an agent by its ULID (internal helper for update returns).
func (db *DB) getAgentByID(ctx context.Context, id string) (*AgentRow, error) {
	ctx, span := startSpan(ctx, "getAgentByID")
	defer span.End()

	q := `
		SELECT a.id, pub.slug, a.publisher_id, a.slug, a.name,
		       coalesce(a.description,''), a.visibility, a.status, a.featured, a.verified, a.tags,
		       coalesce(a.readme,''), a.view_count, a.copy_count, a.created_at, a.updated_at,
		       lav.version, lav.endpoint_url, lav.skills, lav.default_input_modes,
		       lav.default_output_modes, lav.authentication, lav.protocol_version, lav.published_at
		FROM agents a
		JOIN publishers pub ON pub.id = a.publisher_id
		LEFT JOIN LATERAL (
		    SELECT av.version, av.endpoint_url, av.skills, av.default_input_modes,
		           av.default_output_modes, av.authentication, av.protocol_version, av.published_at
		    FROM agent_versions av
		    WHERE av.agent_id = a.id AND av.published_at IS NOT NULL
		    ORDER BY av.published_at DESC
		    LIMIT 1
		) lav ON true
		WHERE a.id = $1`

	var r AgentRow
	var (
		lavVersion     *string
		lavEndpoint    *string
		lavSkills      []byte
		lavInputModes  []string
		lavOutputModes []string
		lavAuth        []byte
		lavProto       *string
		lavPublishedAt *time.Time
	)
	err := db.Pool.QueryRow(ctx, q, id).Scan(
		&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
		&r.Description, &r.Visibility, &r.Status, &r.Featured, &r.Verified, &r.Tags,
		&r.Readme, &r.ViewCount, &r.CopyCount, &r.CreatedAt, &r.UpdatedAt,
		&lavVersion, &lavEndpoint, &lavSkills, &lavInputModes,
		&lavOutputModes, &lavAuth, &lavProto, &lavPublishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting agent by id: %w", err)
	}
	if lavVersion != nil {
		r.LatestVersion = &LatestAgentVersion{
			Version:            *lavVersion,
			EndpointURL:        *lavEndpoint,
			Skills:             json.RawMessage(lavSkills),
			DefaultInputModes:  lavInputModes,
			DefaultOutputModes: lavOutputModes,
			Authentication:     json.RawMessage(lavAuth),
			ProtocolVersion:    *lavProto,
			PublishedAt:        lavPublishedAt,
		}
	}
	return &r, nil
}

func scanAgentVersion(s interface{ Scan(...any) error }) (domain.AgentVersion, error) {
	var v domain.AgentVersion
	err := s.Scan(
		&v.ID, &v.AgentID, &v.Version, &v.EndpointURL,
		&v.Skills, &v.Capabilities, &v.Authentication,
		&v.DefaultInputModes, &v.DefaultOutputModes, &v.Provider,
		&v.DocumentationURL, &v.IconURL,
		&v.ProtocolVersion, &v.Status, &v.StatusMessage, &v.StatusChangedAt,
		&v.PublishedAt, &v.CreatedAt, &v.UpdatedAt,
	)
	return v, err
}

// IncrementAgentViewCount atomically increments the view_count for the
// given agent identified by namespace and slug.
func (db *DB) IncrementAgentViewCount(ctx context.Context, namespace, slug string) error {
	ctx, span := startSpan(ctx, "IncrementAgentViewCount")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agents a
		SET view_count = view_count + 1
		FROM publishers pub
		WHERE pub.id = a.publisher_id AND pub.slug = $1 AND a.slug = $2`,
		namespace, slug,
	)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("incrementing agent view count: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// IncrementAgentCopyCount atomically increments the copy_count for the
// given agent identified by namespace and slug.
func (db *DB) IncrementAgentCopyCount(ctx context.Context, namespace, slug string) error {
	ctx, span := startSpan(ctx, "IncrementAgentCopyCount")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE agents a
		SET copy_count = copy_count + 1
		FROM publishers pub
		WHERE pub.id = a.publisher_id AND pub.slug = $1 AND a.slug = $2`,
		namespace, slug,
	)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("incrementing agent copy count: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
