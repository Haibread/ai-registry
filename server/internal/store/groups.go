package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Group is a team — a named principal that role grants can attach to and that
// users belong to. Its slug doubles as the value matched against OIDC claim
// group names (ADR 0006 §5), so a federated user whose token names the slug
// becomes an effective member without a local group_members row.
type Group struct {
	ID          string    `json:"id"`
	Slug        string    `json:"slug"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateGroupParams holds the fields needed to insert a group.
type CreateGroupParams struct {
	Slug        string
	Name        string
	Description string
}

// UpdateGroupParams holds the mutable fields for a group PATCH.
type UpdateGroupParams struct {
	Name        string
	Description string
}

// ListGroupsParams controls pagination for ListGroups.
type ListGroupsParams struct {
	Limit  int32
	Cursor string
}

const groupColumns = `id, slug, name, coalesce(description, ''), created_at, updated_at`

func scanGroup(row pgx.Row) (*Group, error) {
	var g Group
	if err := row.Scan(&g.ID, &g.Slug, &g.Name, &g.Description, &g.CreatedAt, &g.UpdatedAt); err != nil {
		return nil, err
	}
	return &g, nil
}

// ListGroups returns a page of groups ordered by created_at DESC.
func (db *DB) ListGroups(ctx context.Context, p ListGroupsParams) ([]Group, error) {
	ctx, span := startSpan(ctx, "ListGroups")
	defer span.End()

	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 20
	}

	args := []any{}
	argN := 1
	where := "WHERE 1=1"
	if p.Cursor != "" {
		at, id, err := decodeCursor(p.Cursor)
		if err == nil {
			where += fmt.Sprintf(" AND (created_at, id) < ($%d, $%d)", argN, argN+1)
			args = append(args, at, id)
			argN += 2
		}
	}
	args = append(args, p.Limit)

	rows, err := db.Pool.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM groups %s ORDER BY created_at DESC, id DESC LIMIT $%d`,
		groupColumns, where, argN), args...)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing groups: %w", err)
	}
	defer rows.Close()

	var result []Group
	for rows.Next() {
		g, err := scanGroup(rows)
		if err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning group: %w", err)
		}
		result = append(result, *g)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// GetGroupByID returns a single group by ULID. Returns ErrNotFound if absent.
func (db *DB) GetGroupByID(ctx context.Context, id string) (*Group, error) {
	ctx, span := startSpan(ctx, "GetGroupByID")
	defer span.End()

	g, err := scanGroup(db.Pool.QueryRow(ctx,
		`SELECT `+groupColumns+` FROM groups WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting group by id: %w", err)
	}
	return g, nil
}

// GetGroupBySlug returns a single group by slug. Returns ErrNotFound if absent.
func (db *DB) GetGroupBySlug(ctx context.Context, slug string) (*Group, error) {
	ctx, span := startSpan(ctx, "GetGroupBySlug")
	defer span.End()

	g, err := scanGroup(db.Pool.QueryRow(ctx,
		`SELECT `+groupColumns+` FROM groups WHERE slug = $1`, slug))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting group by slug: %w", err)
	}
	return g, nil
}

// CreateGroup inserts a group row. Returns ErrConflict if the slug exists.
func (db *DB) CreateGroup(ctx context.Context, p CreateGroupParams) (*Group, error) {
	ctx, span := startSpan(ctx, "CreateGroup")
	defer span.End()

	id := NewULID()
	now := time.Now().UTC()
	var description any
	if p.Description != "" {
		description = p.Description
	}

	g, err := scanGroup(db.Pool.QueryRow(ctx, `
		INSERT INTO groups (id, slug, name, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $5)
		RETURNING `+groupColumns,
		id, p.Slug, p.Name, description, now))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating group: %w", err)
	}
	return g, nil
}

// EnsureGroupBySlug returns the group with the given slug, creating it if it
// does not yet exist. Idempotent — used by the boot seed (reviewer group) and
// the workspace-binding conversion. The name is only applied on creation; an
// existing group keeps its current name.
func (db *DB) EnsureGroupBySlug(ctx context.Context, slug, name string) (*Group, error) {
	ctx, span := startSpan(ctx, "EnsureGroupBySlug")
	defer span.End()

	if g, err := db.GetGroupBySlug(ctx, slug); err == nil {
		return g, nil
	} else if !errors.Is(err, ErrNotFound) {
		recordErr(span, err)
		return nil, err
	}

	g, err := db.CreateGroup(ctx, CreateGroupParams{Slug: slug, Name: name})
	if errors.Is(err, ErrConflict) {
		// Raced with a concurrent creator — re-read the winner.
		return db.GetGroupBySlug(ctx, slug)
	}
	if err != nil {
		recordErr(span, err)
		return nil, err
	}
	return g, nil
}

// UpdateGroup patches the mutable fields of a group. Returns ErrNotFound if
// the group does not exist.
func (db *DB) UpdateGroup(ctx context.Context, id string, p UpdateGroupParams) (*Group, error) {
	ctx, span := startSpan(ctx, "UpdateGroup")
	defer span.End()

	var description any
	if p.Description != "" {
		description = p.Description
	}
	g, err := scanGroup(db.Pool.QueryRow(ctx, `
		UPDATE groups SET name = $1, description = $2, updated_at = now()
		WHERE id = $3
		RETURNING `+groupColumns,
		p.Name, description, id))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("updating group: %w", err)
	}
	return g, nil
}

// DeleteGroup removes a group, its memberships (cascade), and all grants
// attached to it (polymorphic, deleted explicitly). Returns ErrNotFound if the
// group does not exist.
func (db *DB) DeleteGroup(ctx context.Context, id string) error {
	ctx, span := startSpan(ctx, "DeleteGroup")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err := tx.Exec(ctx,
		`DELETE FROM role_grants WHERE principal_type = 'group' AND principal_id = $1`,
		id); err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting group grants: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM groups WHERE id = $1`, id)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting group: %w", err)
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

// AddGroupMember adds a user to a group. Idempotent (ON CONFLICT DO NOTHING).
// Returns ErrNotFound when either the group or the user does not exist.
func (db *DB) AddGroupMember(ctx context.Context, groupID, userID string) error {
	ctx, span := startSpan(ctx, "AddGroupMember")
	defer span.End()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)
		ON CONFLICT (group_id, user_id) DO NOTHING`,
		groupID, userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			recordErr(span, ErrNotFound)
			return ErrNotFound
		}
		recordErr(span, err)
		return fmt.Errorf("adding group member: %w", err)
	}
	return nil
}

// RemoveGroupMember removes a user from a group. Returns ErrNotFound if the
// membership did not exist.
func (db *DB) RemoveGroupMember(ctx context.Context, groupID, userID string) error {
	ctx, span := startSpan(ctx, "RemoveGroupMember")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`,
		groupID, userID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("removing group member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// ListGroupMembers returns the users belonging to a group, ordered by email.
func (db *DB) ListGroupMembers(ctx context.Context, groupID string) ([]User, error) {
	ctx, span := startSpan(ctx, "ListGroupMembers")
	defer span.End()

	rows, err := db.Pool.Query(ctx, `
		SELECT `+userColumns+`
		FROM users
		JOIN group_members gm ON gm.user_id = users.id
		WHERE gm.group_id = $1
		ORDER BY users.email`, groupID)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing group members: %w", err)
	}
	defer rows.Close()

	var result []User
	for rows.Next() {
		u, err := db.scanUser(rows)
		if err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning group member: %w", err)
		}
		result = append(result, *u)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}
