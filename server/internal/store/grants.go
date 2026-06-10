package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/trace"

	"github.com/haibread/ai-registry/internal/domain"
)

// RoleGrant is a single row of the role_grants table: principal P holds role R
// on publisher Q (or on all publishers when PublisherID is empty/global).
// PrincipalLabel is the human-readable principal handle (a user's email or a
// group's slug), denormalised on read for the grants API.
type RoleGrant struct {
	ID             string               `json:"id"`
	PrincipalType  domain.PrincipalType `json:"principal_type"`
	PrincipalID    string               `json:"principal_id"`
	PrincipalLabel string               `json:"principal_label,omitempty"`
	PublisherID    string               `json:"publisher_id,omitempty"` // "" = all publishers
	Role           domain.Role          `json:"role"`
	Source         domain.GrantSource   `json:"source"`
	CreatedAt      time.Time            `json:"created_at"`
}

// CreateGrantParams holds the fields needed to insert a role grant. Leave
// PublisherID empty for a global (all-publishers) grant. Source defaults to
// "api" when empty.
type CreateGrantParams struct {
	PrincipalType domain.PrincipalType
	PrincipalID   string
	PublisherID   string // "" = global
	Role          domain.Role
	Source        domain.GrantSource
}

// EffectiveRolesParams identifies a caller for effective-role resolution on a
// single publisher. ClaimGroupSlugs are the verbatim OIDC claim group names
// (empty for local tokens, which contribute only group_members). PublisherID
// is the publisher being checked; global (NULL) grants always apply.
type EffectiveRolesParams struct {
	UserID          string
	ClaimGroupSlugs []string
	PublisherID     string
}

const grantColumns = `g.id, g.principal_type, g.principal_id,
	coalesce(g.publisher_id, ''), g.role, g.source, g.created_at,
	coalesce(u.email, gr.slug, '')`

const grantJoins = `
	FROM role_grants g
	LEFT JOIN users  u  ON g.principal_type = 'user'  AND u.id  = g.principal_id
	LEFT JOIN groups gr ON g.principal_type = 'group' AND gr.id = g.principal_id`

func scanGrant(row pgx.Row) (*RoleGrant, error) {
	var g RoleGrant
	if err := row.Scan(&g.ID, &g.PrincipalType, &g.PrincipalID, &g.PublisherID,
		&g.Role, &g.Source, &g.CreatedAt, &g.PrincipalLabel); err != nil {
		return nil, err
	}
	return &g, nil
}

// EffectiveRoles returns the set of roles the caller holds on the given
// publisher, unioned across grants to their user and to any effective group
// (claim-matched groups ∪ local group_members). Grants match when their
// publisher_id equals the target publisher OR is NULL (all-publishers).
//
// This is the authoritative authorization query. It does NOT account for
// Server Admin — that is sourced from the OIDC claim or the users.is_server_admin
// flag and short-circuits in the middleware before this is consulted.
func (db *DB) EffectiveRoles(ctx context.Context, p EffectiveRolesParams) (map[domain.Role]bool, error) {
	ctx, span := startSpan(ctx, "EffectiveRoles")
	defer span.End()

	slugs := p.ClaimGroupSlugs
	if slugs == nil {
		slugs = []string{}
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT role FROM role_grants
		WHERE (publisher_id = $1 OR publisher_id IS NULL)
		  AND (
		        (principal_type = 'user'  AND principal_id = $2)
		     OR (principal_type = 'group' AND principal_id IN (
		            SELECT id FROM groups WHERE slug = ANY($3::text[])
		            UNION
		            SELECT group_id FROM group_members WHERE user_id = $2
		         ))
		      )`,
		p.PublisherID, p.UserID, slugs)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("resolving effective roles: %w", err)
	}
	defer rows.Close()

	held := make(map[domain.Role]bool, 4)
	for rows.Next() {
		var role domain.Role
		if err := rows.Scan(&role); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning effective role: %w", err)
		}
		held[role] = true
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return held, nil
}

// EffectivePublisherIDs returns the distinct publisher ids on which the caller
// holds any role grant — direct user grants ∪ claim-matched groups ∪ local
// group_members — together with whether the caller holds a global
// (all-publishers) grant. A global grant means "every publisher", so callers
// must NOT apply the returned ids as a filter when hasGlobal is true; they
// should treat the caller as scoped to all publishers instead.
//
// Backs the mine-scoped admin listing: an author sees only the
// resources of publishers they hold a role on.
func (db *DB) EffectivePublisherIDs(ctx context.Context, userID string, claimGroupSlugs []string) (ids []string, hasGlobal bool, err error) {
	ctx, span := startSpan(ctx, "EffectivePublisherIDs")
	defer span.End()

	slugs := claimGroupSlugs
	if slugs == nil {
		slugs = []string{}
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT publisher_id FROM role_grants
		WHERE (
		        (principal_type = 'user'  AND principal_id = $1)
		     OR (principal_type = 'group' AND principal_id IN (
		            SELECT id FROM groups WHERE slug = ANY($2::text[])
		            UNION
		            SELECT group_id FROM group_members WHERE user_id = $1
		         ))
		      )`,
		userID, slugs)
	if err != nil {
		recordErr(span, err)
		return nil, false, fmt.Errorf("resolving effective publishers: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var pid *string
		if err := rows.Scan(&pid); err != nil {
			recordErr(span, err)
			return nil, false, fmt.Errorf("scanning effective publisher: %w", err)
		}
		if pid == nil {
			hasGlobal = true
			continue
		}
		ids = append(ids, *pid)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, false, err
	}
	return ids, hasGlobal, nil
}

// PrincipalGrant is a single role grant held by the caller, enriched with the
// target publisher's slug and name for display. PublisherID/Slug/Name are empty
// for a global (all-publishers) grant. Powers GET /api/v1/me.
type PrincipalGrant struct {
	Role          domain.Role `json:"role"`
	PublisherID   string      `json:"publisher_id,omitempty"`
	PublisherSlug string      `json:"publisher_slug,omitempty"`
	PublisherName string      `json:"publisher_name,omitempty"`
}

// ListGrantsForPrincipal returns every role grant the caller holds — direct
// user grants ∪ claim-matched groups ∪ local group_members — enriched with the
// target publisher's slug/name (empty for global grants). Powers GET
// /api/v1/me so the SPA can gate the admin UI by role without trusting any
// client-side claim.
func (db *DB) ListGrantsForPrincipal(ctx context.Context, userID string, claimGroupSlugs []string) ([]PrincipalGrant, error) {
	ctx, span := startSpan(ctx, "ListGrantsForPrincipal")
	defer span.End()

	slugs := claimGroupSlugs
	if slugs == nil {
		slugs = []string{}
	}

	rows, err := db.Pool.Query(ctx, `
		SELECT DISTINCT g.role, coalesce(g.publisher_id, ''),
		       coalesce(p.slug, ''), coalesce(p.name, '')
		FROM role_grants g
		LEFT JOIN publishers p ON p.id = g.publisher_id
		WHERE (
		        (g.principal_type = 'user'  AND g.principal_id = $1)
		     OR (g.principal_type = 'group' AND g.principal_id IN (
		            SELECT id FROM groups WHERE slug = ANY($2::text[])
		            UNION
		            SELECT group_id FROM group_members WHERE user_id = $1
		         ))
		      )
		ORDER BY 2, 1`,
		userID, slugs)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing principal grants: %w", err)
	}
	defer rows.Close()

	var out []PrincipalGrant
	for rows.Next() {
		var pg PrincipalGrant
		if err := rows.Scan(&pg.Role, &pg.PublisherID, &pg.PublisherSlug, &pg.PublisherName); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning principal grant: %w", err)
		}
		out = append(out, pg)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return out, nil
}

// UserAccessGrant is one role grant contributing to a user's access, enriched
// for display: the target publisher's slug/name (empty for a global grant)
// and, when inherited through a local group membership, the group's identity.
// Powers GET /api/v1/users/{id}/grants.
type UserAccessGrant struct {
	ID            string             `json:"id"`
	Role          domain.Role        `json:"role"`
	PublisherID   string             `json:"publisher_id,omitempty"`
	PublisherSlug string             `json:"publisher_slug,omitempty"`
	PublisherName string             `json:"publisher_name,omitempty"`
	Source        domain.GrantSource `json:"source"`
	Via           string             `json:"via"` // "direct" | "group"
	GroupID       string             `json:"group_id,omitempty"`
	GroupSlug     string             `json:"group_slug,omitempty"`
	GroupName     string             `json:"group_name,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}

// ListUserAccess returns every role grant contributing to a user's access:
// grants attached directly to the user plus grants attached to groups the
// user is a local member of. Grants bound to IdP claim groups are matched
// per-request from the sign-in token and are intentionally absent — local
// state cannot know a federated user's claims.
func (db *DB) ListUserAccess(ctx context.Context, userID string) ([]UserAccessGrant, error) {
	ctx, span := startSpan(ctx, "ListUserAccess")
	defer span.End()

	rows, err := db.Pool.Query(ctx, `
		SELECT g.id, g.role, coalesce(g.publisher_id, ''),
		       coalesce(p.slug, ''), coalesce(p.name, ''), g.source,
		       coalesce(gr.id, ''), coalesce(gr.slug, ''), coalesce(gr.name, ''),
		       g.created_at
		FROM role_grants g
		LEFT JOIN publishers p ON p.id = g.publisher_id
		LEFT JOIN groups gr ON g.principal_type = 'group' AND gr.id = g.principal_id
		WHERE (g.principal_type = 'user'  AND g.principal_id = $1)
		   OR (g.principal_type = 'group' AND g.principal_id IN (
		          SELECT group_id FROM group_members WHERE user_id = $1))
		ORDER BY coalesce(p.slug, ''), g.role, g.created_at, g.id`,
		userID)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing user access: %w", err)
	}
	defer rows.Close()

	var out []UserAccessGrant
	for rows.Next() {
		var g UserAccessGrant
		if err := rows.Scan(&g.ID, &g.Role, &g.PublisherID, &g.PublisherSlug,
			&g.PublisherName, &g.Source, &g.GroupID, &g.GroupSlug, &g.GroupName,
			&g.CreatedAt); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning user access grant: %w", err)
		}
		g.Via = "direct"
		if g.GroupID != "" {
			g.Via = "group"
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return out, nil
}

// ListGrantsByPublisher returns the grants scoped to a single publisher
// (publisher_id = X). Global grants are excluded — list those via
// ListGlobalGrants.
func (db *DB) ListGrantsByPublisher(ctx context.Context, publisherID string) ([]RoleGrant, error) {
	ctx, span := startSpan(ctx, "ListGrantsByPublisher")
	defer span.End()

	rows, err := db.Pool.Query(ctx,
		`SELECT `+grantColumns+grantJoins+
			` WHERE g.publisher_id = $1 ORDER BY g.created_at DESC, g.id DESC`,
		publisherID)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing publisher grants: %w", err)
	}
	return collectGrants(span, rows)
}

// ListGlobalGrants returns the all-publishers grants (publisher_id IS NULL).
func (db *DB) ListGlobalGrants(ctx context.Context) ([]RoleGrant, error) {
	ctx, span := startSpan(ctx, "ListGlobalGrants")
	defer span.End()

	rows, err := db.Pool.Query(ctx,
		`SELECT `+grantColumns+grantJoins+
			` WHERE g.publisher_id IS NULL ORDER BY g.created_at DESC, g.id DESC`)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing global grants: %w", err)
	}
	return collectGrants(span, rows)
}

func collectGrants(span trace.Span, rows pgx.Rows) ([]RoleGrant, error) {
	defer rows.Close()
	var result []RoleGrant
	for rows.Next() {
		g, err := scanGrant(rows)
		if err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning grant: %w", err)
		}
		result = append(result, *g)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// CreateGrant inserts a role grant. Returns ErrConflict when an identical
// grant (same principal, publisher scope, and role) already exists — the
// UNIQUE NULLS NOT DISTINCT constraint treats two global grants as equal.
// Returns ErrNotFound when the publisher_id FK is violated.
func (db *DB) CreateGrant(ctx context.Context, p CreateGrantParams) (*RoleGrant, error) {
	ctx, span := startSpan(ctx, "CreateGrant")
	defer span.End()

	id := NewULID()
	source := p.Source
	if source == "" {
		source = domain.GrantSourceAPI
	}
	var publisherID any
	if p.PublisherID != "" {
		publisherID = p.PublisherID
	}

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO role_grants
			(id, principal_type, principal_id, publisher_id, role, source)
		VALUES ($1, $2, $3, $4, $5, $6)`,
		id, p.PrincipalType, p.PrincipalID, publisherID, p.Role, source)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505":
				recordErr(span, ErrConflict)
				return nil, ErrConflict
			case "23503":
				recordErr(span, ErrNotFound)
				return nil, ErrNotFound
			}
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating grant: %w", err)
	}

	return &RoleGrant{
		ID:            id,
		PrincipalType: p.PrincipalType,
		PrincipalID:   p.PrincipalID,
		PublisherID:   p.PublisherID,
		Role:          p.Role,
		Source:        source,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// EnsureGrant idempotently creates a grant, treating a duplicate as success.
// Used by the boot seed (the config reviewer-group grant) and the
// workspace-binding conversion. Returns the existing or newly-created grant.
func (db *DB) EnsureGrant(ctx context.Context, p CreateGrantParams) error {
	ctx, span := startSpan(ctx, "EnsureGrant")
	defer span.End()

	source := p.Source
	if source == "" {
		source = domain.GrantSourceAPI
	}
	var publisherID any
	if p.PublisherID != "" {
		publisherID = p.PublisherID
	}

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO role_grants
			(id, principal_type, principal_id, publisher_id, role, source)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (principal_type, principal_id, publisher_id, role) DO NOTHING`,
		NewULID(), p.PrincipalType, p.PrincipalID, publisherID, p.Role, source)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("ensuring grant: %w", err)
	}
	return nil
}

// DeleteGrant removes a grant by id. Returns ErrNotFound if absent.
func (db *DB) DeleteGrant(ctx context.Context, id string) error {
	ctx, span := startSpan(ctx, "DeleteGrant")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `DELETE FROM role_grants WHERE id = $1`, id)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting grant: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// GetGrant returns a single grant by id. Returns ErrNotFound if absent.
func (db *DB) GetGrant(ctx context.Context, id string) (*RoleGrant, error) {
	ctx, span := startSpan(ctx, "GetGrant")
	defer span.End()

	g, err := scanGrant(db.Pool.QueryRow(ctx,
		`SELECT `+grantColumns+grantJoins+` WHERE g.id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting grant: %w", err)
	}
	return g, nil
}
