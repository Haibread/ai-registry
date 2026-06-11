package store

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/haibread/ai-registry/internal/domain"
)

// ErrManagedTag is returned when a mutation targets a config-managed tag.
// Managed tags are reconciled from the server configuration at startup; the
// configuration is their source of truth, so the API refuses to edit them.
var ErrManagedTag = errors.New("tag is managed by server configuration")

// ListInstanceTags returns the full instance-wide tag vocabulary — active
// and deactivated tags alike — ordered by name. Deactivated tags stay in the
// listing because versions that carry them still need display resolution;
// callers offering a picker filter on Active.
func (db *DB) ListInstanceTags(ctx context.Context) ([]domain.InstanceTag, error) {
	ctx, span := startSpan(ctx, "ListInstanceTags")
	defer span.End()

	q := `
		SELECT id, slug, name, description, color, active, managed, created_at, updated_at
		FROM instance_tags
		ORDER BY name ASC, slug ASC`

	rows, err := db.Pool.Query(ctx, q)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing instance tags: %w", err)
	}
	defer rows.Close()

	var result []domain.InstanceTag
	for rows.Next() {
		var t domain.InstanceTag
		if err := rows.Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Color,
			&t.Active, &t.Managed, &t.CreatedAt, &t.UpdatedAt); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning instance tag: %w", err)
		}
		result = append(result, t)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// CreateInstanceTagParams holds the fields needed to define a new tag.
type CreateInstanceTagParams struct {
	Slug        string
	Name        string
	Description string
	Color       string
}

// CreateInstanceTag inserts a new tag definition (active by default).
// Returns ErrConflict if the slug is already taken.
func (db *DB) CreateInstanceTag(ctx context.Context, p CreateInstanceTagParams) (*domain.InstanceTag, error) {
	ctx, span := startSpan(ctx, "CreateInstanceTag")
	defer span.End()

	id := NewULID()
	var t domain.InstanceTag
	err := db.Pool.QueryRow(ctx, `
		INSERT INTO instance_tags (id, slug, name, description, color)
		VALUES ($1,$2,$3,$4,$5)
		RETURNING id, slug, name, description, color, active, managed, created_at, updated_at`,
		id, p.Slug, p.Name, p.Description, p.Color,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Color,
		&t.Active, &t.Managed, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating instance tag: %w", err)
	}
	return &t, nil
}

// UpdateInstanceTagParams carries a partial update; nil fields are left
// unchanged. The slug is the tag's identity on frozen version rows and is
// intentionally not updatable.
type UpdateInstanceTagParams struct {
	Name        *string
	Description *string
	Color       *string
	Active      *bool
}

// UpdateInstanceTag applies a partial update to the tag named by slug.
// Returns ErrNotFound if no such tag exists and ErrManagedTag when the tag
// is reconciled from server configuration (read-only via the API).
func (db *DB) UpdateInstanceTag(ctx context.Context, slug string, p UpdateInstanceTagParams) (*domain.InstanceTag, error) {
	ctx, span := startSpan(ctx, "UpdateInstanceTag")
	defer span.End()

	sets := []string{}
	args := []any{slug}
	addSet := func(col string, v any) {
		sets = append(sets, fmt.Sprintf("%s = $%d", col, len(args)+1))
		args = append(args, v)
	}
	if p.Name != nil {
		addSet("name", *p.Name)
	}
	if p.Description != nil {
		addSet("description", *p.Description)
	}
	if p.Color != nil {
		addSet("color", *p.Color)
	}
	if p.Active != nil {
		addSet("active", *p.Active)
	}
	if len(sets) == 0 {
		// Nothing to change — return the current row so PATCH {} is a no-op read.
		sets = append(sets, "slug = slug")
	}

	var t domain.InstanceTag
	err := db.Pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE instance_tags
		SET %s
		WHERE slug = $1 AND NOT managed
		RETURNING id, slug, name, description, color, active, managed, created_at, updated_at`,
		strings.Join(sets, ", ")), args...,
	).Scan(&t.ID, &t.Slug, &t.Name, &t.Description, &t.Color,
		&t.Active, &t.Managed, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		reason := db.tagMissingOrManaged(ctx, slug)
		recordErr(span, reason)
		return nil, reason
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("updating instance tag: %w", err)
	}
	return &t, nil
}

// tagMissingOrManaged disambiguates a zero-row mutation on instance_tags:
// the tag either does not exist (ErrNotFound) or is config-managed and the
// guard refused to touch it (ErrManagedTag).
func (db *DB) tagMissingOrManaged(ctx context.Context, slug string) error {
	var managed bool
	err := db.Pool.QueryRow(ctx,
		`SELECT managed FROM instance_tags WHERE slug = $1`, slug,
	).Scan(&managed)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("checking instance tag: %w", err)
	}
	if managed {
		return ErrManagedTag
	}
	return ErrNotFound
}

// DeleteInstanceTag hard-deletes a tag that no version references. Returns
// ErrNotFound if the tag does not exist, ErrManagedTag when it is reconciled
// from server configuration, and ErrConflict if any MCP server or agent
// version carries it (published versions are immutable, so an in-use tag can
// only be deactivated).
func (db *DB) DeleteInstanceTag(ctx context.Context, slug string) error {
	ctx, span := startSpan(ctx, "DeleteInstanceTag")
	defer span.End()

	// The guards and the delete are a single statement, so a concurrent
	// version create that ticks this tag cannot race the existence checks.
	asArray := []string{slug}
	tag, err := db.Pool.Exec(ctx, `
		DELETE FROM instance_tags
		WHERE slug = $1
		  AND NOT managed
		  AND NOT EXISTS (SELECT 1 FROM mcp_server_versions v WHERE v.tags @> $2)
		  AND NOT EXISTS (SELECT 1 FROM agent_versions av WHERE av.tags @> $2)`,
		slug, asArray)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting instance tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		// Diagnose which guard refused: missing row → 404, managed → the
		// configuration owns it, otherwise it must be carried by a version.
		var managed bool
		err := db.Pool.QueryRow(ctx,
			`SELECT managed FROM instance_tags WHERE slug = $1`, slug,
		).Scan(&managed)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			recordErr(span, ErrNotFound)
			return ErrNotFound
		case err != nil:
			recordErr(span, err)
			return fmt.Errorf("checking instance tag: %w", err)
		case managed:
			recordErr(span, ErrManagedTag)
			return ErrManagedTag
		default:
			recordErr(span, ErrConflict)
			return ErrConflict
		}
	}
	return nil
}

// ManagedTagSpec is one config-defined tag to reconcile at startup. The
// caller (cmd/server) maps the config package's spec onto it so the store
// does not depend on config.
type ManagedTagSpec struct {
	Slug        string
	Name        string
	Description string
	Color       string
	Active      bool
}

// ReconcileManagedInstanceTags makes the database agree with the
// config-defined portion of the tag vocabulary:
//
//   - every spec is created or updated by slug and flagged managed (the
//     configuration wins over any admin edit, including one made while the
//     tag was briefly unmanaged);
//   - previously managed tags that are no longer in the configuration have
//     the flag cleared, releasing them to admin-UI ownership (they are NOT
//     deleted or deactivated — frozen versions may carry them).
//
// Runs in one transaction so a restart can never observe a half-applied
// vocabulary.
func (db *DB) ReconcileManagedInstanceTags(ctx context.Context, specs []ManagedTagSpec) error {
	ctx, span := startSpan(ctx, "ReconcileManagedInstanceTags")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback is a no-op after commit

	slugs := make([]string, 0, len(specs))
	for _, s := range specs {
		slugs = append(slugs, s.Slug)
		if _, err := tx.Exec(ctx, `
			INSERT INTO instance_tags (id, slug, name, description, color, active, managed)
			VALUES ($1,$2,$3,$4,$5,$6,true)
			ON CONFLICT (slug) DO UPDATE
			SET name = EXCLUDED.name,
			    description = EXCLUDED.description,
			    color = EXCLUDED.color,
			    active = EXCLUDED.active,
			    managed = true`,
			NewULID(), s.Slug, s.Name, s.Description, s.Color, s.Active,
		); err != nil {
			recordErr(span, err)
			return fmt.Errorf("reconciling instance tag %q: %w", s.Slug, err)
		}
	}

	if _, err := tx.Exec(ctx,
		`UPDATE instance_tags SET managed = false WHERE managed AND NOT (slug = ANY($1))`,
		slugs,
	); err != nil {
		recordErr(span, err)
		return fmt.Errorf("releasing unlisted managed tags: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		recordErr(span, err)
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// MissingActiveTagSlugs returns the subset of slugs that are not active
// instance tags (unknown or deactivated), preserving input order. Used to
// validate the tags ticked on a new version against the vocabulary.
func (db *DB) MissingActiveTagSlugs(ctx context.Context, slugs []string) ([]string, error) {
	ctx, span := startSpan(ctx, "MissingActiveTagSlugs")
	defer span.End()

	if len(slugs) == 0 {
		return nil, nil
	}
	rows, err := db.Pool.Query(ctx,
		`SELECT slug FROM instance_tags WHERE slug = ANY($1) AND active`, slugs)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("resolving instance tags: %w", err)
	}
	defer rows.Close()

	known := make(map[string]struct{}, len(slugs))
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning instance tag slug: %w", err)
		}
		known[s] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}

	var missing []string
	for _, s := range slugs {
		if _, ok := known[s]; !ok {
			missing = append(missing, s)
		}
	}
	return missing, nil
}
