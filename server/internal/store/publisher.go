package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

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

// DeletePublisher hard-deletes a publisher. Returns ErrConflict if the
// publisher still owns any active MCP servers or agents (status != 'deleted').
//
// Soft-deleted (tombstoned) child rows are purged in the same transaction so
// that the ON DELETE RESTRICT foreign key does not block the publisher delete.
// The intent of soft-deletion is to hide an entry from listings without
// silently breaking caches mid-run; once the owning publisher itself is being
// removed, there is nothing left to protect and the tombstones can go.
func (db *DB) DeletePublisher(ctx context.Context, publisherID string) error {
	ctx, span := startSpan(ctx, "DeletePublisher")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback is a no-op after commit

	// Check for dependent active resources. Tombstoned rows do not count —
	// they are purged below before the publisher delete runs.
	var mcpCount, agentCount int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM mcp_servers WHERE publisher_id=$1 AND status != 'deleted'`,
		publisherID).Scan(&mcpCount); err != nil {
		recordErr(span, err)
		return fmt.Errorf("counting mcp servers: %w", err)
	}
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM agents WHERE publisher_id=$1 AND status != 'deleted'`,
		publisherID).Scan(&agentCount); err != nil {
		recordErr(span, err)
		return fmt.Errorf("counting agents: %w", err)
	}
	if mcpCount > 0 || agentCount > 0 {
		recordErr(span, ErrConflict)
		return ErrConflict
	}

	// Purge tombstoned children so the ON DELETE RESTRICT FK will not fire.
	// Version tables also use ON DELETE RESTRICT, so delete version rows
	// first, then the parent mcp_server/agent rows.
	if _, err := tx.Exec(ctx,
		`DELETE FROM mcp_server_versions
		 WHERE server_id IN (
		     SELECT id FROM mcp_servers WHERE publisher_id=$1 AND status='deleted'
		 )`,
		publisherID); err != nil {
		recordErr(span, err)
		return fmt.Errorf("purging tombstoned mcp versions: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM mcp_servers WHERE publisher_id=$1 AND status='deleted'`,
		publisherID); err != nil {
		recordErr(span, err)
		return fmt.Errorf("purging tombstoned mcp servers: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM agent_versions
		 WHERE agent_id IN (
		     SELECT id FROM agents WHERE publisher_id=$1 AND status='deleted'
		 )`,
		publisherID); err != nil {
		recordErr(span, err)
		return fmt.Errorf("purging tombstoned agent versions: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM agents WHERE publisher_id=$1 AND status='deleted'`,
		publisherID); err != nil {
		recordErr(span, err)
		return fmt.Errorf("purging tombstoned agents: %w", err)
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

	if err := tx.Commit(ctx); err != nil {
		recordErr(span, err)
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// scanPublisher is a convenience alias used by domain helpers.
var _ = domain.VisibilityPublic // keep domain import used
