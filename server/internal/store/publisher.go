package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"

	"github.com/haibread/ai-registry/internal/domain"
)

// Publisher is the full publisher row returned by list/get queries.
type Publisher struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Contact   string    `json:"contact,omitempty"`
	Verified  bool      `json:"verified"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ListPublishersParams controls pagination for ListPublishers.
type ListPublishersParams struct {
	Limit  int32
	Cursor string
}

// CreatePublisherParams holds the fields needed to insert a new publisher.
type CreatePublisherParams struct {
	Slug    string
	Name    string
	Contact string
}

// ListPublishers returns a page of publishers ordered by created_at DESC.
func (db *DB) ListPublishers(ctx context.Context, p ListPublishersParams) ([]Publisher, error) {
	ctx, span := startSpan(ctx, "ListPublishers")
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
	rows, err := db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT id, slug, name, coalesce(contact,''), verified, created_at, updated_at
		FROM publishers
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d`, where, argN), args...)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("listing publishers: %w", err)
	}
	defer rows.Close()

	var result []Publisher
	for rows.Next() {
		var pub Publisher
		if err := rows.Scan(&pub.ID, &pub.Slug, &pub.Name, &pub.Contact,
			&pub.Verified, &pub.CreatedAt, &pub.UpdatedAt); err != nil {
			recordErr(span, err)
			return nil, fmt.Errorf("scanning publisher: %w", err)
		}
		result = append(result, pub)
	}
	if err := rows.Err(); err != nil {
		recordErr(span, err)
		return nil, err
	}
	return result, nil
}

// GetPublisher returns a single publisher by slug.
func (db *DB) GetPublisher(ctx context.Context, slug string) (*Publisher, error) {
	ctx, span := startSpan(ctx, "GetPublisher")
	defer span.End()

	var pub Publisher
	err := db.Pool.QueryRow(ctx, `
		SELECT id, slug, name, coalesce(contact,''), verified, created_at, updated_at
		FROM publishers WHERE slug = $1`, slug).
		Scan(&pub.ID, &pub.Slug, &pub.Name, &pub.Contact,
			&pub.Verified, &pub.CreatedAt, &pub.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting publisher: %w", err)
	}
	return &pub, nil
}

// CreatePublisher inserts a new publisher row.
// Returns ErrConflict if the slug already exists.
func (db *DB) CreatePublisher(ctx context.Context, p CreatePublisherParams) (*Publisher, error) {
	ctx, span := startSpan(ctx, "CreatePublisher")
	defer span.End()

	id := NewULID()
	now := time.Now().UTC()

	_, err := db.Pool.Exec(ctx, `
		INSERT INTO publishers (id, slug, name, contact, verified, created_at, updated_at)
		VALUES ($1, $2, $3, $4, false, $5, $5)`,
		id, p.Slug, p.Name, p.Contact, now,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating publisher: %w", err)
	}

	return &Publisher{
		ID:        id,
		Slug:      p.Slug,
		Name:      p.Name,
		Contact:   p.Contact,
		Verified:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// SetPublisherVerified updates the verified flag on a publisher.
func (db *DB) SetPublisherVerified(ctx context.Context, id string, verified bool) error {
	ctx, span := startSpan(ctx, "SetPublisherVerified")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE publishers SET verified=$1, updated_at=NOW() WHERE id=$2`, verified, id)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("updating publisher verified: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// UpdatePublisherParams holds the mutable fields for a PATCH operation.
type UpdatePublisherParams struct {
	Name    string
	Contact string
}

// UpdatePublisher updates the mutable metadata fields of a publisher.
// Returns ErrNotFound if the publisher does not exist.
func (db *DB) UpdatePublisher(ctx context.Context, publisherID string, p UpdatePublisherParams) (*Publisher, error) {
	ctx, span := startSpan(ctx, "UpdatePublisher")
	defer span.End()

	var pub Publisher
	err := db.Pool.QueryRow(ctx, `
		UPDATE publishers
		SET name=$1, contact=$2, updated_at=now()
		WHERE id=$3
		RETURNING id, slug, name, coalesce(contact,''), verified, created_at, updated_at`,
		p.Name, p.Contact, publisherID,
	).Scan(&pub.ID, &pub.Slug, &pub.Name, &pub.Contact, &pub.Verified, &pub.CreatedAt, &pub.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("updating publisher: %w", err)
	}
	return &pub, nil
}

// DeletePublisher hard-deletes a publisher and cascades the deletion to every
// resource it owns: all MCP servers and agents (whatever their status), their
// version rows, and any community reports filed against them. Role grants
// scoped to the publisher are removed automatically by their ON DELETE CASCADE
// foreign key.
//
// The whole cascade runs in one transaction, so the publisher and all of its
// dependents disappear together or not at all. The entry and version tables use
// ON DELETE RESTRICT, so children are deleted bottom-up — versions before their
// parent mcp_server/agent rows, and those before the publisher itself — rather
// than relying on the database to cascade. A delete on a publisher with no
// resources is just the final DELETE with nothing to sweep.
func (db *DB) DeletePublisher(ctx context.Context, publisherID string) error {
	ctx, span := startSpan(ctx, "DeletePublisher")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback is a no-op after commit

	// Reports reference resources by (resource_type, resource_id) with no
	// foreign key, so they neither block the delete nor cascade automatically —
	// purge them explicitly before the resources they point at vanish, leaving
	// no dangling reports behind.
	if _, err := tx.Exec(ctx,
		`DELETE FROM reports
		  WHERE (resource_type='mcp_server'
		         AND resource_id IN (SELECT id FROM mcp_servers WHERE publisher_id=$1))
		     OR (resource_type='agent'
		         AND resource_id IN (SELECT id FROM agents      WHERE publisher_id=$1))`,
		publisherID); err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting resource reports: %w", err)
	}

	// Version tables use ON DELETE RESTRICT, so delete version rows before the
	// parent mcp_server/agent rows.
	if _, err := tx.Exec(ctx,
		`DELETE FROM mcp_server_versions
		  WHERE server_id IN (SELECT id FROM mcp_servers WHERE publisher_id=$1)`,
		publisherID); err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting mcp versions: %w", err)
	}
	mcpTag, err := tx.Exec(ctx,
		`DELETE FROM mcp_servers WHERE publisher_id=$1`, publisherID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting mcp servers: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM agent_versions
		  WHERE agent_id IN (SELECT id FROM agents WHERE publisher_id=$1)`,
		publisherID); err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting agent versions: %w", err)
	}
	agentTag, err := tx.Exec(ctx,
		`DELETE FROM agents WHERE publisher_id=$1`, publisherID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting agents: %w", err)
	}

	tag, err := tx.Exec(ctx, `DELETE FROM publishers WHERE id=$1`, publisherID)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("deleting publisher: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}

	span.SetAttributes(
		attribute.Int64("registry.cascade.mcp_servers_deleted", mcpTag.RowsAffected()),
		attribute.Int64("registry.cascade.agents_deleted", agentTag.RowsAffected()),
	)

	if err := tx.Commit(ctx); err != nil {
		recordErr(span, err)
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// scanPublisher is a convenience alias used by domain helpers.
var _ = domain.VisibilityPublic // keep domain import used
