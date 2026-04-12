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

// ErrNotFound is returned when a requested entity does not exist.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned when a unique constraint would be violated.
var ErrConflict = errors.New("conflict")

// ErrImmutable is returned when a mutation is attempted on a published version.
var ErrImmutable = errors.New("published versions are immutable")

// ErrInvalidCursor is returned when the pagination cursor cannot be decoded.
var ErrInvalidCursor = errors.New("invalid cursor")

// LatestMCPVersion is a summary of the most recently published version,
// embedded inline in list and detail responses.
type LatestMCPVersion struct {
	Version         string
	Runtime         domain.Runtime
	ProtocolVersion string
	Packages        json.RawMessage
	Capabilities    json.RawMessage
	PublishedAt     *time.Time
}

// ListMCPServersParams controls filtering and pagination for ListMCPServers.
type ListMCPServersParams struct {
	PublicOnly     bool       // when true, only visibility='public' rows are returned
	Namespace      string     // filter by publisher slug (optional)
	Status         string     // filter by status: "draft" | "published" | "deprecated" | "" (all)
	Visibility     string     // filter by visibility: "public" | "private" | "" (all); only meaningful when PublicOnly=false
	Query          string     // full-text search term (optional)
	Limit          int32
	Cursor         string     // opaque cursor (created_at::text + "," + id)
	UpdatedSince   *time.Time // when non-nil, only rows updated after this time
	IncludeDeleted bool       // when true, include servers with status='deleted'
	VersionFilter  string     // when non-empty, filter to servers with this published version ("latest" = default latest-version behaviour)
	Transport      string     // filter by transport type in latest version packages JSONB: "stdio" | "sse" | "streamable_http"
	RegistryType   string     // filter by registryType in latest version packages JSONB (e.g. "npm", "pip")
	Sort           string     // sort order: "created_at_desc" (default), "updated_at_desc", "name_asc", "name_desc"
}

// MCPServerRow is a flat projection used by list queries (includes namespace).
type MCPServerRow struct {
	domain.MCPServer
	LatestVersion *LatestMCPVersion
}

// ListMCPServers returns a page of MCP server rows and the total count of
// rows that match the filters (before pagination).
func (db *DB) ListMCPServers(ctx context.Context, p ListMCPServersParams) ([]MCPServerRow, int, error) {
	ctx, span := startSpan(ctx, "ListMCPServers")
	defer span.End()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	args := []any{}
	argN := 1
	// countArgN tracks the arg index for the separate COUNT query, which uses
	// the same filter args but without the cursor and without ORDER-BY args.
	countArgN := 1

	// filterWhere / filterArgs hold conditions shared between the main query and
	// the COUNT query.  Cursor args are added to the main query only.
	filterWhere := "WHERE 1=1"
	filterArgs := []any{}

	if p.PublicOnly {
		filterWhere += fmt.Sprintf(" AND s.visibility = $%d", argN)
		filterArgs = append(filterArgs, "public")
		argN++
		countArgN++
	} else if p.Visibility != "" {
		// Admin-only explicit visibility filter (ignored when PublicOnly forces public)
		filterWhere += fmt.Sprintf(" AND s.visibility = $%d", argN)
		filterArgs = append(filterArgs, p.Visibility)
		argN++
		countArgN++
	}
	if p.Status != "" {
		filterWhere += fmt.Sprintf(" AND s.status = $%d", argN)
		filterArgs = append(filterArgs, p.Status)
		argN++
		countArgN++
	} else if !p.IncludeDeleted {
		// By default, exclude deleted servers.
		filterWhere += fmt.Sprintf(" AND s.status != $%d", argN)
		filterArgs = append(filterArgs, "deleted")
		argN++
		countArgN++
	}
	if p.Namespace != "" {
		filterWhere += fmt.Sprintf(" AND pub.slug = $%d", argN)
		filterArgs = append(filterArgs, p.Namespace)
		argN++
		countArgN++
	}
	if p.UpdatedSince != nil {
		filterWhere += fmt.Sprintf(" AND s.updated_at > $%d", argN)
		filterArgs = append(filterArgs, *p.UpdatedSince)
		argN++
		countArgN++
	}

	// Transport filter — checks that at least one package in the latest version
	// has the requested transport type. The condition is applied after the
	// lateral join, so we add it to a post-join filter list.
	var postJoinFilter string
	var postJoinFilterCount string
	if p.Transport != "" {
		postJoinFilter += fmt.Sprintf(
			" AND lv.packages @> $%d::jsonb",
			argN,
		)
		postJoinFilterCount += fmt.Sprintf(
			" AND lv.packages @> $%d::jsonb",
			countArgN,
		)
		transportJSON := fmt.Sprintf(`[{"transport":{"type":"%s"}}]`, p.Transport)
		filterArgs = append(filterArgs, transportJSON)
		argN++
		countArgN++
	}
	if p.RegistryType != "" {
		postJoinFilter += fmt.Sprintf(
			" AND lv.packages @> $%d::jsonb",
			argN,
		)
		postJoinFilterCount += fmt.Sprintf(
			" AND lv.packages @> $%d::jsonb",
			countArgN,
		)
		registryJSON := fmt.Sprintf(`[{"registryType":"%s"}]`, p.RegistryType)
		filterArgs = append(filterArgs, registryJSON)
		argN++
		countArgN++
	}

	// When searching, use the generated search_vector index and rank results.
	// Cursor pagination is skipped for ranked searches (rank is not a stable
	// column for keyset pagination).
	tsQuery := prefixTSQuery(p.Query)
	hasQuery := tsQuery != ""
	if hasQuery {
		filterWhere += fmt.Sprintf(
			" AND s.search_vector @@ to_tsquery('english', $%d)",
			argN,
		)
		filterArgs = append(filterArgs, tsQuery)
		argN++
		countArgN++
	}

	// Snapshot filterArgs before adding cursor / ORDER-BY args so the COUNT
	// query can reuse just the filter portion.
	countArgs := make([]any, len(filterArgs))
	copy(countArgs, filterArgs)

	// Main query args start from the filter args.
	args = append(args, filterArgs...)

	// Cursor (keyset pagination) — added to the main query only.
	whereClause := filterWhere
	// Keyset cursor works for time-based sort columns. For name-based sorts
	// we fall back to OFFSET via the cursor being empty (the frontend will
	// still get next_cursor=nil and can page by incrementing offset if needed).
	if !hasQuery && p.Cursor != "" {
		at, id, err := decodeCursor(p.Cursor)
		if err != nil {
			return nil, 0, ErrInvalidCursor
		}
		cursorCol := "s.created_at"
		if p.Sort == "updated_at_desc" {
			cursorCol = "s.updated_at"
		}
		// For name-based sorts, cursor is not supported (the cursor encodes a
		// timestamp, not a name). Silently ignore the cursor.
		if p.Sort != "name_asc" && p.Sort != "name_desc" {
			whereClause += fmt.Sprintf(" AND (%s, s.id) < ($%d, $%d)", cursorCol, argN, argN+1)
			args = append(args, at, id)
			argN += 2
		}
	}

	orderClause := "ORDER BY s.created_at DESC, s.id DESC"
	if hasQuery {
		orderClause = fmt.Sprintf(
			"ORDER BY ts_rank(s.search_vector, to_tsquery('english', $%d)) DESC, s.created_at DESC",
			argN,
		)
		args = append(args, tsQuery)
		argN++
	} else {
		switch p.Sort {
		case "updated_at_desc":
			orderClause = "ORDER BY s.updated_at DESC, s.id DESC"
		case "name_asc":
			orderClause = "ORDER BY s.name ASC, s.id ASC"
		case "name_desc":
			orderClause = "ORDER BY s.name DESC, s.id DESC"
		// default: created_at_desc — already set above
		}
	}

	// Build the lateral join for latest/specific version.
	// When VersionFilter is a specific semver (not "latest" and not empty),
	// the lateral join is narrowed to that exact version, and we require lv
	// to be non-NULL (i.e. the server must have that version published).
	lateralVersionCond := "v.published_at IS NOT NULL"
	lateralCountCond := lateralVersionCond // same for count unless overridden below
	if p.VersionFilter != "" && p.VersionFilter != "latest" {
		// Main query arg
		lateralVersionCond += fmt.Sprintf(" AND v.version = $%d", argN)
		args = append(args, p.VersionFilter)
		argN++
		// Count query arg (uses countArgN for its own numbering)
		lateralCountCond += fmt.Sprintf(" AND v.version = $%d", countArgN)
		countArgs = append(countArgs, p.VersionFilter)
		countArgN++
		// Only include servers that actually have this version.
		whereClause += " AND lv.version IS NOT NULL"
		filterWhere += " AND lv.version IS NOT NULL" // also for count query
	}

	// Post-join filters (transport / registry_type) reference lv.packages which
	// is only available after the lateral join. Apply to both main + count queries.
	if postJoinFilter != "" {
		whereClause += postJoinFilter
		filterWhere += postJoinFilterCount
	}

	args = append(args, p.Limit)
	q := fmt.Sprintf(`
		SELECT s.id, pub.slug AS namespace, s.publisher_id, s.slug, s.name,
		       coalesce(s.description,''), coalesce(s.homepage_url,''), coalesce(s.repo_url,''),
		       coalesce(s.license,''), s.visibility, s.status, s.created_at, s.updated_at,
		       lv.version, lv.runtime, lv.protocol_version, lv.packages, lv.capabilities, lv.published_at
		FROM mcp_servers s
		JOIN publishers pub ON pub.id = s.publisher_id
		LEFT JOIN LATERAL (
		    SELECT v.version, v.runtime, v.protocol_version, v.packages, v.capabilities, v.published_at
		    FROM mcp_server_versions v
		    WHERE v.server_id = s.id AND %s
		    ORDER BY v.published_at DESC
		    LIMIT 1
		) lv ON true
		%s
		%s
		LIMIT $%d`, lateralVersionCond, whereClause, orderClause, argN)

	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		recordErr(span, err)
		return nil, 0, fmt.Errorf("listing mcp servers: %w", err)
	}
	defer rows.Close()

	var result []MCPServerRow
	for rows.Next() {
		var r MCPServerRow
		var (
			lvVersion      *string
			lvRuntime      *string
			lvProto        *string
			lvPackages     []byte
			lvCapabilities []byte
			lvPublishedAt  *time.Time
		)
		if err := rows.Scan(
			&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
			&r.Description, &r.HomepageURL, &r.RepoURL, &r.License,
			&r.Visibility, &r.Status, &r.CreatedAt, &r.UpdatedAt,
			&lvVersion, &lvRuntime, &lvProto, &lvPackages, &lvCapabilities, &lvPublishedAt,
		); err != nil {
			recordErr(span, err)
			return nil, 0, fmt.Errorf("scanning mcp server row: %w", err)
		}
		if lvVersion != nil {
			r.LatestVersion = &LatestMCPVersion{
				Version:         *lvVersion,
				Runtime:         domain.Runtime(*lvRuntime),
				ProtocolVersion: *lvProto,
				Packages:        json.RawMessage(lvPackages),
				Capabilities:    json.RawMessage(lvCapabilities),
				PublishedAt:     lvPublishedAt,
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
	countQ := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM mcp_servers s
		JOIN publishers pub ON pub.id = s.publisher_id
		LEFT JOIN LATERAL (
		    SELECT v.version, v.packages
		    FROM mcp_server_versions v
		    WHERE v.server_id = s.id AND %s
		    ORDER BY v.published_at DESC
		    LIMIT 1
		) lv ON true
		%s`, lateralCountCond, filterWhere)

	var total int
	if err := db.Pool.QueryRow(ctx, countQ, countArgs...).Scan(&total); err != nil {
		recordErr(span, err)
		return nil, 0, fmt.Errorf("counting mcp servers: %w", err)
	}

	return result, total, nil
}

// GetMCPServer retrieves a single MCP server by namespace and slug.
// Returns ErrNotFound if no matching row exists.
func (db *DB) GetMCPServer(ctx context.Context, namespace, slug string, publicOnly bool) (*MCPServerRow, error) {
	ctx, span := startSpan(ctx, "GetMCPServer")
	defer span.End()

	q := `
		SELECT s.id, pub.slug, s.publisher_id, s.slug, s.name,
		       coalesce(s.description,''), coalesce(s.homepage_url,''), coalesce(s.repo_url,''),
		       coalesce(s.license,''), s.visibility, s.status, s.created_at, s.updated_at,
		       lv.version, lv.runtime, lv.protocol_version, lv.packages, lv.capabilities, lv.published_at
		FROM mcp_servers s
		JOIN publishers pub ON pub.id = s.publisher_id
		LEFT JOIN LATERAL (
		    SELECT v.version, v.runtime, v.protocol_version, v.packages, v.capabilities, v.published_at
		    FROM mcp_server_versions v
		    WHERE v.server_id = s.id AND v.published_at IS NOT NULL
		    ORDER BY v.published_at DESC
		    LIMIT 1
		) lv ON true
		WHERE pub.slug = $1 AND s.slug = $2`
	args := []any{namespace, slug}
	if publicOnly {
		q += " AND s.visibility = 'public'"
	}

	var r MCPServerRow
	var (
		lvVersion      *string
		lvRuntime      *string
		lvProto        *string
		lvPackages     []byte
		lvCapabilities []byte
		lvPublishedAt  *time.Time
	)
	err := db.Pool.QueryRow(ctx, q, args...).Scan(
		&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
		&r.Description, &r.HomepageURL, &r.RepoURL, &r.License,
		&r.Visibility, &r.Status, &r.CreatedAt, &r.UpdatedAt,
		&lvVersion, &lvRuntime, &lvProto, &lvPackages, &lvCapabilities, &lvPublishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting mcp server: %w", err)
	}
	if lvVersion != nil {
		r.LatestVersion = &LatestMCPVersion{
			Version:         *lvVersion,
			Runtime:         domain.Runtime(*lvRuntime),
			ProtocolVersion: *lvProto,
			Packages:        json.RawMessage(lvPackages),
			Capabilities:    json.RawMessage(lvCapabilities),
			PublishedAt:     lvPublishedAt,
		}
	}
	return &r, nil
}

// GetMCPServerByID retrieves an MCP server by its ULID.
func (db *DB) GetMCPServerByID(ctx context.Context, id string) (*MCPServerRow, error) {
	ctx, span := startSpan(ctx, "GetMCPServerByID")
	defer span.End()

	q := `
		SELECT s.id, pub.slug, s.publisher_id, s.slug, s.name,
		       coalesce(s.description,''), coalesce(s.homepage_url,''), coalesce(s.repo_url,''),
		       coalesce(s.license,''), s.visibility, s.status, s.created_at, s.updated_at,
		       lv.version, lv.runtime, lv.protocol_version, lv.packages, lv.capabilities, lv.published_at
		FROM mcp_servers s
		JOIN publishers pub ON pub.id = s.publisher_id
		LEFT JOIN LATERAL (
		    SELECT v.version, v.runtime, v.protocol_version, v.packages, v.capabilities, v.published_at
		    FROM mcp_server_versions v
		    WHERE v.server_id = s.id AND v.published_at IS NOT NULL
		    ORDER BY v.published_at DESC
		    LIMIT 1
		) lv ON true
		WHERE s.id = $1`

	var r MCPServerRow
	var (
		lvVersion      *string
		lvRuntime      *string
		lvProto        *string
		lvPackages     []byte
		lvCapabilities []byte
		lvPublishedAt  *time.Time
	)
	err := db.Pool.QueryRow(ctx, q, id).Scan(
		&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
		&r.Description, &r.HomepageURL, &r.RepoURL, &r.License,
		&r.Visibility, &r.Status, &r.CreatedAt, &r.UpdatedAt,
		&lvVersion, &lvRuntime, &lvProto, &lvPackages, &lvCapabilities, &lvPublishedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting mcp server by id: %w", err)
	}
	if lvVersion != nil {
		r.LatestVersion = &LatestMCPVersion{
			Version:         *lvVersion,
			Runtime:         domain.Runtime(*lvRuntime),
			ProtocolVersion: *lvProto,
			Packages:        json.RawMessage(lvPackages),
			Capabilities:    json.RawMessage(lvCapabilities),
			PublishedAt:     lvPublishedAt,
		}
	}
	return &r, nil
}

// CreateMCPServerParams holds the fields needed to insert a new MCP server.
type CreateMCPServerParams struct {
	PublisherID string
	Slug        string
	Name        string
	Description string
	HomepageURL string
	RepoURL     string
	License     string
}

// CreateMCPServer inserts a new MCP server row (draft, private by default).
// Returns ErrConflict if the (publisher_id, slug) pair already exists.
func (db *DB) CreateMCPServer(ctx context.Context, p CreateMCPServerParams) (*domain.MCPServer, error) {
	ctx, span := startSpan(ctx, "CreateMCPServer")
	defer span.End()

	id := NewULID()
	now := time.Now().UTC()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO mcp_servers
		    (id, publisher_id, slug, name, description, homepage_url, repo_url, license,
		     visibility, status, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,'private','draft',$9,$9)`,
		id, p.PublisherID, p.Slug, p.Name,
		p.Description, p.HomepageURL, p.RepoURL, p.License, now,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating mcp server: %w", err)
	}

	return &domain.MCPServer{
		ID:          id,
		PublisherID: p.PublisherID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		HomepageURL: p.HomepageURL,
		RepoURL:     p.RepoURL,
		License:     p.License,
		Visibility:  domain.VisibilityPrivate,
		Status:      domain.StatusDraft,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// ListMCPServerVersions returns all versions for a given server ID, ordered by created_at.
func (db *DB) ListMCPServerVersions(ctx context.Context, serverID string) ([]domain.MCPServerVersion, error) {
	ctx, span := startSpan(ctx, "ListMCPServerVersions")
	defer span.End()

	rows, err := db.Pool.Query(ctx, `
		SELECT id, server_id, version, runtime, packages, capabilities,
		       protocol_version, coalesce(checksum,''), coalesce(signature,''),
		       status, published_at, created_at, updated_at, coalesce(status_message,''), status_changed_at
		FROM mcp_server_versions
		WHERE server_id = $1
		ORDER BY created_at DESC`, serverID)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing mcp server versions: %w", err)
	}
	defer rows.Close()

	var result []domain.MCPServerVersion
	for rows.Next() {
		v, err := scanVersion(rows)
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

// GetMCPServerVersion retrieves a specific version by server ID and semver string.
func (db *DB) GetMCPServerVersion(ctx context.Context, serverID, version string) (*domain.MCPServerVersion, error) {
	ctx, span := startSpan(ctx, "GetMCPServerVersion")
	defer span.End()

	row := db.Pool.QueryRow(ctx, `
		SELECT id, server_id, version, runtime, packages, capabilities,
		       protocol_version, coalesce(checksum,''), coalesce(signature,''),
		       status, published_at, created_at, updated_at, coalesce(status_message,''), status_changed_at
		FROM mcp_server_versions
		WHERE server_id = $1 AND version = $2`, serverID, version)

	v, err := scanVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting mcp server version: %w", err)
	}
	return &v, nil
}

// GetLatestPublishedVersion returns the most recently published version for a server.
func (db *DB) GetLatestPublishedVersion(ctx context.Context, serverID string) (*domain.MCPServerVersion, error) {
	ctx, span := startSpan(ctx, "GetLatestPublishedVersion")
	defer span.End()

	row := db.Pool.QueryRow(ctx, `
		SELECT id, server_id, version, runtime, packages, capabilities,
		       protocol_version, coalesce(checksum,''), coalesce(signature,''),
		       status, published_at, created_at, updated_at, coalesce(status_message,''), status_changed_at
		FROM mcp_server_versions
		WHERE server_id = $1 AND published_at IS NOT NULL
		ORDER BY published_at DESC
		LIMIT 1`, serverID)

	v, err := scanVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting latest published version: %w", err)
	}
	return &v, nil
}

// CreateMCPServerVersionParams holds the fields needed to insert a new version.
type CreateMCPServerVersionParams struct {
	ServerID        string
	Version         string
	Runtime         domain.Runtime
	Packages        json.RawMessage
	Capabilities    json.RawMessage
	ProtocolVersion string
	Checksum        string
	Signature       string
}

// CreateMCPServerVersion inserts a new draft version.
// Returns ErrConflict if the (server_id, version) pair already exists.
func (db *DB) CreateMCPServerVersion(ctx context.Context, p CreateMCPServerVersionParams) (*domain.MCPServerVersion, error) {
	ctx, span := startSpan(ctx, "CreateMCPServerVersion")
	defer span.End()

	if len(p.Capabilities) == 0 {
		p.Capabilities = json.RawMessage("{}")
	}

	id := NewULID()
	now := time.Now().UTC()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO mcp_server_versions
		    (id, server_id, version, runtime, packages, capabilities,
		     protocol_version, checksum, signature)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		id, p.ServerID, p.Version, p.Runtime, p.Packages, p.Capabilities,
		p.ProtocolVersion, p.Checksum, p.Signature,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating mcp server version: %w", err)
	}

	return &domain.MCPServerVersion{
		ID:              id,
		ServerID:        p.ServerID,
		Version:         p.Version,
		Runtime:         p.Runtime,
		Packages:        p.Packages,
		Capabilities:    p.Capabilities,
		ProtocolVersion: p.ProtocolVersion,
		Checksum:        p.Checksum,
		Signature:       p.Signature,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}

// PublishMCPServerVersion sets published_at on a draft version, making it
// immutable. Returns ErrNotFound if version doesn't exist, ErrImmutable if
// already published.
func (db *DB) PublishMCPServerVersion(ctx context.Context, serverID, version string) error {
	ctx, span := startSpan(ctx, "PublishMCPServerVersion")
	defer span.End()

	now := time.Now().UTC()
	tag, err := db.Pool.Exec(ctx, `
		UPDATE mcp_server_versions
		SET published_at = $1
		WHERE server_id = $2 AND version = $3 AND published_at IS NULL`,
		now, serverID, version)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("publishing version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Check whether it exists at all or is already published.
		var publishedAt *time.Time
		err := db.Pool.QueryRow(ctx,
			`SELECT published_at FROM mcp_server_versions WHERE server_id=$1 AND version=$2`,
			serverID, version,
		).Scan(&publishedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			recordErr(span, ErrNotFound)
			return ErrNotFound
		}
		recordErr(span, ErrImmutable)
		return ErrImmutable
	}
	// After publish, promote the server status to 'published' if it was draft.
	_, _ = db.Pool.Exec(ctx,
		`UPDATE mcp_servers SET status='published', updated_at=now() WHERE id=$1 AND status='draft'`,
		serverID)
	return nil
}

// DeprecateMCPServer marks an MCP server as deprecated.
func (db *DB) DeprecateMCPServer(ctx context.Context, serverID string) error {
	ctx, span := startSpan(ctx, "DeprecateMCPServer")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE mcp_servers SET status='deprecated', updated_at=now() WHERE id=$1 AND status='published'`,
		serverID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deprecating mcp server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// SetMCPServerVisibility sets the visibility of an MCP server.
func (db *DB) SetMCPServerVisibility(ctx context.Context, serverID string, vis domain.Visibility) error {
	ctx, span := startSpan(ctx, "SetMCPServerVisibility")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE mcp_servers SET visibility=$1, updated_at=now() WHERE id=$2`,
		vis, serverID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("setting visibility: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// GetPublisherBySlug returns a publisher ID and name for a given slug.
func (db *DB) GetPublisherBySlug(ctx context.Context, slug string) (id string, err error) {
	ctx, span := startSpan(ctx, "GetPublisherBySlug")
	defer span.End()

	err = db.Pool.QueryRow(ctx, `SELECT id FROM publishers WHERE slug=$1`, slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return "", ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
	}
	return id, err
}

// scanVersion scans one mcp_server_versions row from any pgx scanner.
// Column order must match SELECT: id, server_id, version, runtime, packages, capabilities,
// protocol_version, checksum, signature, status, published_at, created_at, updated_at, status_message, status_changed_at
func scanVersion(s interface {
	Scan(...any) error
}) (domain.MCPServerVersion, error) {
	var v domain.MCPServerVersion
	err := s.Scan(
		&v.ID, &v.ServerID, &v.Version, &v.Runtime,
		&v.Packages, &v.Capabilities,
		&v.ProtocolVersion, &v.Checksum, &v.Signature,
		&v.Status, &v.PublishedAt, &v.CreatedAt, &v.UpdatedAt,
		&v.StatusMessage, &v.StatusChangedAt,
	)
	return v, err
}

// SetMCPServerStatus updates the lifecycle status of an MCP server.
// Allowed values: published (active), deprecated.
// Returns ErrNotFound if the server does not exist.
func (db *DB) SetMCPServerStatus(ctx context.Context, serverID string, status domain.ServerStatus) error {
	ctx, span := startSpan(ctx, "SetMCPServerStatus")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE mcp_servers SET status=$1, updated_at=now() WHERE id=$2`,
		status, serverID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("setting mcp server status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// SetMCPVersionStatus updates the per-version status for a specific server version.
// Allowed values: active, deprecated, deleted.
// Returns ErrNotFound if the version does not exist.
func (db *DB) SetMCPVersionStatus(ctx context.Context, serverID, version string, status domain.VersionStatus, statusMessage string) error {
	ctx, span := startSpan(ctx, "SetMCPVersionStatus")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE mcp_server_versions SET status=$1, status_message=$2, status_changed_at=now() WHERE server_id=$3 AND version=$4`,
		status, statusMessage, serverID, version)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("setting mcp version status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// SetAllVersionsStatus updates the status of all published versions of a server atomically.
// Returns the updated versions.
func (db *DB) SetAllVersionsStatus(ctx context.Context, serverID string, status domain.VersionStatus, statusMessage string) ([]domain.MCPServerVersion, error) {
	ctx, span := startSpan(ctx, "SetAllVersionsStatus")
	defer span.End()

	rows, err := db.Pool.Query(ctx, `
		UPDATE mcp_server_versions
		SET status=$1, status_message=$2, status_changed_at=now()
		WHERE server_id=$3 AND published_at IS NOT NULL
		RETURNING id, server_id, version, runtime, packages, capabilities,
		          protocol_version, coalesce(checksum,''), coalesce(signature,''),
		          status, published_at, created_at, updated_at, coalesce(status_message,''), status_changed_at`,
		status, statusMessage, serverID)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("setting all versions status: %w", err)
	}
	defer rows.Close()

	var result []domain.MCPServerVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning updated version: %w", err)
		}
		result = append(result, v)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// UpdateMCPServerParams holds the mutable fields for a PATCH operation.
type UpdateMCPServerParams struct {
	Name        string
	Description string
	HomepageURL string
	RepoURL     string
	License     string
}

// UpdateMCPServer updates the mutable metadata fields of an MCP server.
// Returns ErrNotFound if the server does not exist.
func (db *DB) UpdateMCPServer(ctx context.Context, serverID string, p UpdateMCPServerParams) (*MCPServerRow, error) {
	ctx, span := startSpan(ctx, "UpdateMCPServer")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `
		UPDATE mcp_servers
		SET name=$1, description=$2, homepage_url=$3, repo_url=$4, license=$5, updated_at=now()
		WHERE id=$6 AND status != 'deleted'`,
		p.Name, p.Description, p.HomepageURL, p.RepoURL, p.License, serverID,
	)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("updating mcp server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	return db.GetMCPServerByID(ctx, serverID)
}

// DeleteMCPServer soft-deletes an MCP server by setting status='deleted' on
// the server and all its versions. Returns ErrNotFound if not found.
func (db *DB) DeleteMCPServer(ctx context.Context, serverID string) error {
	ctx, span := startSpan(ctx, "DeleteMCPServer")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE mcp_servers SET status='deleted', updated_at=now() WHERE id=$1 AND status != 'deleted'`,
		serverID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting mcp server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	// Mark all versions deleted too.
	_, err = db.Pool.Exec(ctx,
		`UPDATE mcp_server_versions SET status='deleted', status_changed_at=now() WHERE server_id=$1`,
		serverID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting mcp server versions: %w", err)
	}
	return nil
}

// decodeCursor splits a cursor string into (time, id).
func decodeCursor(cursor string) (time.Time, string, error) {
	// cursor format: "<RFC3339>,<ulid>"
	idx := len(cursor) - 26 // ULID is always 26 chars
	if idx < 2 || cursor[idx-1] != ',' {
		return time.Time{}, "", fmt.Errorf("invalid cursor")
	}
	t, err := time.Parse(time.RFC3339Nano, cursor[:idx-1])
	if err != nil {
		return time.Time{}, "", err
	}
	return t, cursor[idx:], nil
}

// EncodeCursor produces an opaque cursor from a time and ID.
func EncodeCursor(t time.Time, id string) string {
	return t.UTC().Format(time.RFC3339Nano) + "," + id
}
