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

// scanPublisher is a convenience alias used by domain helpers.
var _ = domain.VisibilityPublic // keep domain import used
