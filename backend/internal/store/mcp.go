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

// ListMCPServersParams controls filtering and pagination for ListMCPServers.
type ListMCPServersParams struct {
	PublicOnly bool   // when true, only visibility='public' rows are returned
	Namespace  string // filter by publisher slug (optional)
	Query      string // full-text search term (optional)
	Limit      int32
	Cursor     string // opaque cursor (created_at::text + "," + id)
}

// MCPServerRow is a flat projection used by list queries (includes namespace).
type MCPServerRow struct {
	domain.MCPServer
}

// ListMCPServers returns a page of MCP server rows.
func (db *DB) ListMCPServers(ctx context.Context, p ListMCPServersParams) ([]MCPServerRow, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	args := []any{}
	argN := 1

	whereClause := "WHERE 1=1"
	if p.PublicOnly {
		whereClause += fmt.Sprintf(" AND s.visibility = $%d", argN)
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
			" AND to_tsvector('english', coalesce(s.name,'') || ' ' || coalesce(s.description,'')) @@ plainto_tsquery('english', $%d)",
			argN,
		)
		args = append(args, p.Query)
		argN++
	}
	if p.Cursor != "" {
		// cursor = created_at_rfc3339 + "," + id
		at, id, err := decodeCursor(p.Cursor)
		if err == nil {
			whereClause += fmt.Sprintf(" AND (s.created_at, s.id) > ($%d, $%d)", argN, argN+1)
			args = append(args, at, id)
			argN += 2
		}
	}

	args = append(args, p.Limit)
	q := fmt.Sprintf(`
		SELECT s.id, pub.slug AS namespace, s.publisher_id, s.slug, s.name,
		       coalesce(s.description,''), coalesce(s.homepage_url,''), coalesce(s.repo_url,''),
		       coalesce(s.license,''), s.visibility, s.status, s.created_at, s.updated_at
		FROM mcp_servers s
		JOIN publishers pub ON pub.id = s.publisher_id
		%s
		ORDER BY s.created_at ASC, s.id ASC
		LIMIT $%d`, whereClause, argN)

	rows, err := db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("listing mcp servers: %w", err)
	}
	defer rows.Close()

	var result []MCPServerRow
	for rows.Next() {
		var r MCPServerRow
		if err := rows.Scan(
			&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
			&r.Description, &r.HomepageURL, &r.RepoURL, &r.License,
			&r.Visibility, &r.Status, &r.CreatedAt, &r.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning mcp server row: %w", err)
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

// GetMCPServer retrieves a single MCP server by namespace and slug.
// Returns ErrNotFound if no matching row exists.
func (db *DB) GetMCPServer(ctx context.Context, namespace, slug string, publicOnly bool) (*MCPServerRow, error) {
	q := `
		SELECT s.id, pub.slug, s.publisher_id, s.slug, s.name,
		       coalesce(s.description,''), coalesce(s.homepage_url,''), coalesce(s.repo_url,''),
		       coalesce(s.license,''), s.visibility, s.status, s.created_at, s.updated_at
		FROM mcp_servers s
		JOIN publishers pub ON pub.id = s.publisher_id
		WHERE pub.slug = $1 AND s.slug = $2`
	args := []any{namespace, slug}
	if publicOnly {
		q += " AND s.visibility = 'public'"
	}

	var r MCPServerRow
	err := db.Pool.QueryRow(ctx, q, args...).Scan(
		&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
		&r.Description, &r.HomepageURL, &r.RepoURL, &r.License,
		&r.Visibility, &r.Status, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting mcp server: %w", err)
	}
	return &r, nil
}

// GetMCPServerByID retrieves an MCP server by its ULID.
func (db *DB) GetMCPServerByID(ctx context.Context, id string) (*MCPServerRow, error) {
	q := `
		SELECT s.id, pub.slug, s.publisher_id, s.slug, s.name,
		       coalesce(s.description,''), coalesce(s.homepage_url,''), coalesce(s.repo_url,''),
		       coalesce(s.license,''), s.visibility, s.status, s.created_at, s.updated_at
		FROM mcp_servers s
		JOIN publishers pub ON pub.id = s.publisher_id
		WHERE s.id = $1`

	var r MCPServerRow
	err := db.Pool.QueryRow(ctx, q, id).Scan(
		&r.ID, &r.Namespace, &r.PublisherID, &r.Slug, &r.Name,
		&r.Description, &r.HomepageURL, &r.RepoURL, &r.License,
		&r.Visibility, &r.Status, &r.CreatedAt, &r.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting mcp server by id: %w", err)
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
			return nil, ErrConflict
		}
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

// ListMCPServerVersions returns all versions for a given server ID, ordered by released_at.
func (db *DB) ListMCPServerVersions(ctx context.Context, serverID string) ([]domain.MCPServerVersion, error) {
	rows, err := db.Pool.Query(ctx, `
		SELECT id, server_id, version, runtime, packages, capabilities,
		       protocol_version, coalesce(checksum,''), coalesce(signature,''),
		       published_at, released_at
		FROM mcp_server_versions
		WHERE server_id = $1
		ORDER BY released_at DESC`, serverID)
	if err != nil {
		return nil, fmt.Errorf("listing mcp server versions: %w", err)
	}
	defer rows.Close()

	var result []domain.MCPServerVersion
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}

// GetMCPServerVersion retrieves a specific version by server ID and semver string.
func (db *DB) GetMCPServerVersion(ctx context.Context, serverID, version string) (*domain.MCPServerVersion, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, server_id, version, runtime, packages, capabilities,
		       protocol_version, coalesce(checksum,''), coalesce(signature,''),
		       published_at, released_at
		FROM mcp_server_versions
		WHERE server_id = $1 AND version = $2`, serverID, version)

	v, err := scanVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting mcp server version: %w", err)
	}
	return &v, nil
}

// GetLatestPublishedVersion returns the most recently published version for a server.
func (db *DB) GetLatestPublishedVersion(ctx context.Context, serverID string) (*domain.MCPServerVersion, error) {
	row := db.Pool.QueryRow(ctx, `
		SELECT id, server_id, version, runtime, packages, capabilities,
		       protocol_version, coalesce(checksum,''), coalesce(signature,''),
		       published_at, released_at
		FROM mcp_server_versions
		WHERE server_id = $1 AND published_at IS NOT NULL
		ORDER BY published_at DESC
		LIMIT 1`, serverID)

	v, err := scanVersion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
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
	if len(p.Capabilities) == 0 {
		p.Capabilities = json.RawMessage("{}")
	}

	id := NewULID()
	now := time.Now().UTC()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO mcp_server_versions
		    (id, server_id, version, runtime, packages, capabilities,
		     protocol_version, checksum, signature, released_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		id, p.ServerID, p.Version, p.Runtime, p.Packages, p.Capabilities,
		p.ProtocolVersion, p.Checksum, p.Signature, now,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrConflict
		}
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
		ReleasedAt:      now,
	}, nil
}

// PublishMCPServerVersion sets published_at on a draft version, making it
// immutable. Returns ErrNotFound if version doesn't exist, ErrImmutable if
// already published.
func (db *DB) PublishMCPServerVersion(ctx context.Context, serverID, version string) error {
	now := time.Now().UTC()
	tag, err := db.Pool.Exec(ctx, `
		UPDATE mcp_server_versions
		SET published_at = $1
		WHERE server_id = $2 AND version = $3 AND published_at IS NULL`,
		now, serverID, version)
	if err != nil {
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
			return ErrNotFound
		}
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
	tag, err := db.Pool.Exec(ctx,
		`UPDATE mcp_servers SET status='deprecated', updated_at=now() WHERE id=$1 AND status='published'`,
		serverID)
	if err != nil {
		return fmt.Errorf("deprecating mcp server: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetMCPServerVisibility sets the visibility of an MCP server.
func (db *DB) SetMCPServerVisibility(ctx context.Context, serverID string, vis domain.Visibility) error {
	tag, err := db.Pool.Exec(ctx,
		`UPDATE mcp_servers SET visibility=$1, updated_at=now() WHERE id=$2`,
		vis, serverID)
	if err != nil {
		return fmt.Errorf("setting visibility: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetPublisherBySlug returns a publisher ID and name for a given slug.
func (db *DB) GetPublisherBySlug(ctx context.Context, slug string) (id string, err error) {
	err = db.Pool.QueryRow(ctx, `SELECT id FROM publishers WHERE slug=$1`, slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return id, err
}

// scanVersion scans one mcp_server_versions row from any pgx scanner.
func scanVersion(s interface {
	Scan(...any) error
}) (domain.MCPServerVersion, error) {
	var v domain.MCPServerVersion
	err := s.Scan(
		&v.ID, &v.ServerID, &v.Version, &v.Runtime,
		&v.Packages, &v.Capabilities,
		&v.ProtocolVersion, &v.Checksum, &v.Signature,
		&v.PublishedAt, &v.ReleasedAt,
	)
	return v, err
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
