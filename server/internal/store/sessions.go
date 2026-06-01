package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Session is a server-side login session.
// Both front doors — brokered OIDC and local password — create one. The opaque
// cookie token is never stored; only its SHA-256 hash (TokenHash) is, so a DB
// leak yields no usable session. ClaimGroups and ClaimAdmin snapshot the OIDC
// claim inputs at login (empty / false for local logins); effective roles are
// still resolved live from role_grants on every request.
type Session struct {
	ID          string
	UserID      string
	AuthMethod  string // "oidc" | "local"
	ClaimGroups []string
	ClaimAdmin  bool
	// IDToken is the raw OIDC id_token, kept only for OIDC sessions so logout can
	// pass it to the IdP as id_token_hint (RP-initiated logout). Empty for local
	// sessions. Never sent to the browser.
	IDToken   string
	CreatedAt time.Time
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// CreateSessionParams holds the fields needed to open a session. TokenHash is
// the hex SHA-256 of the cookie token, computed by the auth layer — the raw
// token never reaches the store. ClaimGroups / ClaimAdmin are the login-time
// snapshot (nil ClaimGroups is stored as an empty array).
type CreateSessionParams struct {
	UserID      string
	TokenHash   string
	AuthMethod  string
	ClaimGroups []string
	ClaimAdmin  bool
	// IDToken is the raw OIDC id_token (empty for local logins); stored as a
	// logout hint. See Session.IDToken.
	IDToken   string
	ExpiresAt time.Time
}

const sessionColumns = `id, user_id, auth_method, claim_groups, claim_admin,
	coalesce(id_token, ''), created_at, expires_at, revoked_at`

func scanSession(row pgx.Row) (*Session, error) {
	var s Session
	if err := row.Scan(&s.ID, &s.UserID, &s.AuthMethod, &s.ClaimGroups,
		&s.ClaimAdmin, &s.IDToken, &s.CreatedAt, &s.ExpiresAt, &s.RevokedAt); err != nil {
		return nil, err
	}
	return &s, nil
}

// CreateSession inserts a session row. AuthMethod must be "oidc" or "local".
// Returns ErrConflict if the token hash already exists (a caller bug — tokens
// are random).
func (db *DB) CreateSession(ctx context.Context, p CreateSessionParams) (*Session, error) {
	ctx, span := startSpan(ctx, "CreateSession")
	defer span.End()

	groups := p.ClaimGroups
	if groups == nil {
		groups = []string{}
	}
	var idToken *string
	if p.IDToken != "" {
		idToken = &p.IDToken
	}
	s, err := scanSession(db.Pool.QueryRow(ctx, `
		INSERT INTO sessions
			(id, token_hash, user_id, auth_method, claim_groups, claim_admin, id_token, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+sessionColumns,
		NewULID(), p.TokenHash, p.UserID, p.AuthMethod, groups, p.ClaimAdmin, idToken, p.ExpiresAt))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating session: %w", err)
	}
	return s, nil
}

// ActiveSessionByTokenHash returns the live (not revoked, not expired) session
// for a cookie token hash. Returns ErrNotFound when no active session matches —
// the middleware treats that as "unauthenticated".
func (db *DB) ActiveSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	ctx, span := startSpan(ctx, "ActiveSessionByTokenHash")
	defer span.End()

	s, err := scanSession(db.Pool.QueryRow(ctx,
		`SELECT `+sessionColumns+` FROM sessions
		 WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()`,
		tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("getting session by token hash: %w", err)
	}
	return s, nil
}

// RevokeSession marks the session with the given token hash revoked (logout).
// Returns ErrNotFound when there is no live session to revoke.
func (db *DB) RevokeSession(ctx context.Context, tokenHash string) error {
	ctx, span := startSpan(ctx, "RevokeSession")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE sessions SET revoked_at = now()
		 WHERE token_hash = $1 AND revoked_at IS NULL`, tokenHash)
	if err != nil {
		recordErr(span, err)
		return fmt.Errorf("revoking session: %w", err)
	}
	if tag.RowsAffected() == 0 {
		recordErr(span, ErrNotFound)
		return ErrNotFound
	}
	return nil
}

// DeleteExpiredSessions removes sessions that are past expiry or were revoked.
// Housekeeping; returns the number of rows removed.
func (db *DB) DeleteExpiredSessions(ctx context.Context) (int64, error) {
	ctx, span := startSpan(ctx, "DeleteExpiredSessions")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`DELETE FROM sessions WHERE expires_at <= now() OR revoked_at IS NOT NULL`)
	if err != nil {
		recordErr(span, err)
		return 0, fmt.Errorf("deleting expired sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}
