package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Workspace is the full workspace row returned by list/get queries. See
// ADR 0001 — Workspaces under publishers.
type Workspace struct {
	ID          string    `json:"id"`
	PublisherID string    `json:"publisher_id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Contact     string    `json:"contact,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListWorkspacesParams controls pagination for ListWorkspaces. PublisherID
// is required — workspaces are always listed under a single publisher.
type ListWorkspacesParams struct {
	PublisherID string
	Limit       int32
	Cursor      string
}

// CreateWorkspaceParams holds the fields needed to insert a new workspace.
type CreateWorkspaceParams struct {
	PublisherID string
	Slug        string
	Name        string
	Description string
	Contact     string
}

// UpdateWorkspaceParams holds the mutable fields for a PATCH operation.
type UpdateWorkspaceParams struct {
	Name        string
	Description string
	Contact     string
}

// ListWorkspaces returns a page of workspaces under one publisher, ordered
// by created_at DESC.
func (db *DB) ListWorkspaces(ctx context.Context, p ListWorkspacesParams) ([]Workspace, error) {
	ctx, span := startSpan(ctx, "ListWorkspaces")
	defer span.End()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	args := []any{p.PublisherID}
	argN := 2
	where := "WHERE publisher_id = $1"

	if p.Cursor != "" {
		at, id, err := decodeCursor(p.Cursor)
		if err == nil {
			where += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argN, argN+1)
			args = append(args, at, id)
			argN += 2
		}
	}

	args = append(args, p.Limit)
	rows, err := db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT id, publisher_id, slug, name,
		       coalesce(description, ''), coalesce(contact, ''),
		       created_at, updated_at
		FROM workspaces
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, where, argN), args...)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing workspaces: %w", err)
	}
	defer rows.Close()

	var result []Workspace
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.ID, &w.PublisherID, &w.Slug, &w.Name,
			&w.Description, &w.Contact, &w.CreatedAt, &w.UpdatedAt); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning workspace: %w", err)
		}
		result = append(result, w)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// GetWorkspace returns a single workspace by (publisher_id, slug).
func (db *DB) GetWorkspace(ctx context.Context, publisherID, slug string) (*Workspace, error) {
	ctx, span := startSpan(ctx, "GetWorkspace")
	defer span.End()

	var w Workspace
	err := db.Pool.QueryRow(ctx, `
		SELECT id, publisher_id, slug, name,
		       coalesce(description, ''), coalesce(contact, ''),
		       created_at, updated_at
		FROM workspaces WHERE publisher_id = $1 AND slug = $2`,
		publisherID, slug).
		Scan(&w.ID, &w.PublisherID, &w.Slug, &w.Name,
			&w.Description, &w.Contact, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting workspace: %w", err)
	}
	return &w, nil
}

// GetWorkspaceByID returns a single workspace by ULID.
func (db *DB) GetWorkspaceByID(ctx context.Context, id string) (*Workspace, error) {
	ctx, span := startSpan(ctx, "GetWorkspaceByID")
	defer span.End()

	var w Workspace
	err := db.Pool.QueryRow(ctx, `
		SELECT id, publisher_id, slug, name,
		       coalesce(description, ''), coalesce(contact, ''),
		       created_at, updated_at
		FROM workspaces WHERE id = $1`, id).
		Scan(&w.ID, &w.PublisherID, &w.Slug, &w.Name,
			&w.Description, &w.Contact, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting workspace by id: %w", err)
	}
	return &w, nil
}

// ResolveWorkspace resolves (publisher_slug, workspace_slug) → workspace.
// Used by request handlers that take the hierarchical URL form.
func (db *DB) ResolveWorkspace(ctx context.Context, publisherSlug, workspaceSlug string) (*Workspace, error) {
	ctx, span := startSpan(ctx, "ResolveWorkspace")
	defer span.End()

	var w Workspace
	err := db.Pool.QueryRow(ctx, `
		SELECT w.id, w.publisher_id, w.slug, w.name,
		       coalesce(w.description, ''), coalesce(w.contact, ''),
		       w.created_at, w.updated_at
		FROM workspaces w
		JOIN publishers p ON p.id = w.publisher_id
		WHERE p.slug = $1 AND w.slug = $2`,
		publisherSlug, workspaceSlug).
		Scan(&w.ID, &w.PublisherID, &w.Slug, &w.Name,
			&w.Description, &w.Contact, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("resolving workspace: %w", err)
	}
	return &w, nil
}

// CreateWorkspace inserts a new workspace row. Returns ErrConflict if a
// workspace with the same (publisher_id, slug) already exists.
func (db *DB) CreateWorkspace(ctx context.Context, p CreateWorkspaceParams) (*Workspace, error) {
	ctx, span := startSpan(ctx, "CreateWorkspace")
	defer span.End()

	id := NewULID()
	now := time.Now().UTC()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO workspaces
			(id, publisher_id, slug, name, description, contact, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)`,
		id, p.PublisherID, p.Slug, p.Name, p.Description, p.Contact, now,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				recordErr(span, ErrConflict)
				return nil, ErrConflict
			case "23503":
				// publisher_id FK violation — caller passed a bogus publisher.
				recordErr(span, ErrNotFound)
				return nil, ErrNotFound
			}
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating workspace: %w", err)
	}

	return &Workspace{
		ID:          id,
		PublisherID: p.PublisherID,
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		Contact:     p.Contact,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// UpdateWorkspace updates the mutable metadata fields of a workspace.
// Returns ErrNotFound if the workspace does not exist.
func (db *DB) UpdateWorkspace(ctx context.Context, workspaceID string, p UpdateWorkspaceParams) (*Workspace, error) {
	ctx, span := startSpan(ctx, "UpdateWorkspace")
	defer span.End()

	var w Workspace
	err := db.Pool.QueryRow(ctx, `
		UPDATE workspaces
		SET name=$1, description=$2, contact=$3, updated_at=now()
		WHERE id=$4
		RETURNING id, publisher_id, slug, name,
		          coalesce(description, ''), coalesce(contact, ''),
		          created_at, updated_at`,
		p.Name, p.Description, p.Contact, workspaceID,
	).Scan(&w.ID, &w.PublisherID, &w.Slug, &w.Name,
		&w.Description, &w.Contact, &w.CreatedAt, &w.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("updating workspace: %w", err)
	}
	return &w, nil
}

// DeleteWorkspace hard-deletes a workspace. Returns ErrConflict if any MCP
// server or agent still references the workspace (regardless of status —
// even tombstoned rows hold the FK).
//
// Per ADR 0001, workspace deletion requires the workspace to be empty.
// During the transitional period before the finalising migration, a
// resource may have NULL workspace_id (legacy publisher-keyed rows that
// haven't been backfilled); those do not count against this check.
func (db *DB) DeleteWorkspace(ctx context.Context, workspaceID string) error {
	ctx, span := startSpan(ctx, "DeleteWorkspace")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var mcpCount, agentCount int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM mcp_servers WHERE workspace_id = $1`,
		workspaceID).Scan(&mcpCount); err != nil {
		recordErr(span, err)
		return fmt.Errorf("counting mcp servers: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM agents WHERE workspace_id = $1`,
		workspaceID).Scan(&agentCount); err != nil {
		recordErr(span, err)
		return fmt.Errorf("counting agents: %w", err)
	}
	if mcpCount > 0 || agentCount > 0 {
		recordErr(span, ErrConflict)
		return ErrConflict
	}

	tag, err := tx.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, workspaceID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting workspace: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		recordErr(span, err)
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
