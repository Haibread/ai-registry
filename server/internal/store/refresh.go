package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrRefreshReuse is returned by RotateRefreshToken when an already-rotated
// (revoked) refresh token is presented again. This is the theft signal: the
// whole token lineage for the user has been revoked as a side effect, so the
// caller must force a fresh login.
var ErrRefreshReuse = errors.New("refresh token reuse detected")

// RefreshToken is a rotating, single-use refresh token. The raw token is never
// stored; only its SHA-256 hash (TokenHash) is the lookup key, so a DB leak
// yields no usable token. ClaimGroups / ClaimAdmin snapshot the OIDC claim
// inputs at login (empty / false for local logins); effective roles are still
// resolved live from role_grants on every request. IDToken is the OIDC id_token
// kept only as an RP-initiated-logout hint (empty for local).
type RefreshToken struct {
	ID          string
	UserID      string
	AuthMethod  string
	ClaimGroups []string
	ClaimAdmin  bool
	IDToken     string
	RotatedFrom *string
	CreatedAt   time.Time
	ExpiresAt   time.Time
	RevokedAt   *time.Time
}

// CreateRefreshTokenParams holds the fields needed to mint a refresh token.
// TokenHash is the hex SHA-256 of the raw token, computed by the auth layer.
type CreateRefreshTokenParams struct {
	UserID      string
	TokenHash   string
	AuthMethod  string
	ClaimGroups []string
	ClaimAdmin  bool
	IDToken     string
	ExpiresAt   time.Time
}

const refreshColumns = `id, user_id, auth_method, claim_groups, claim_admin,
	coalesce(id_token, ''), rotated_from, created_at, expires_at, revoked_at`

func scanRefreshToken(row pgx.Row) (*RefreshToken, error) {
	var t RefreshToken
	if err := row.Scan(&t.ID, &t.UserID, &t.AuthMethod, &t.ClaimGroups,
		&t.ClaimAdmin, &t.IDToken, &t.RotatedFrom, &t.CreatedAt, &t.ExpiresAt,
		&t.RevokedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateRefreshToken inserts a refresh token row. AuthMethod must be "oidc" or
// "local". Returns ErrConflict on a hash collision (a caller bug — tokens are
// random).
func (db *DB) CreateRefreshToken(ctx context.Context, p CreateRefreshTokenParams) (*RefreshToken, error) {
	ctx, span := startSpan(ctx, "CreateRefreshToken")
	defer span.End()

	groups := p.ClaimGroups
	if groups == nil {
		groups = []string{}
	}
	var idToken *string
	if p.IDToken != "" {
		idToken = &p.IDToken
	}
	t, err := scanRefreshToken(db.Pool.QueryRow(ctx, `
		INSERT INTO refresh_tokens
			(id, token_hash, user_id, auth_method, claim_groups, claim_admin, id_token, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING `+refreshColumns,
		NewULID(), p.TokenHash, p.UserID, p.AuthMethod, groups, p.ClaimAdmin, idToken, p.ExpiresAt))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			recordErr(span, ErrConflict)
			return nil, ErrConflict
		}
		recordErr(span, err)
		return nil, fmt.Errorf("creating refresh token: %w", err)
	}
	return t, nil
}

// RotateRefreshToken atomically consumes the refresh token identified by oldHash
// and issues a successor with newHash. It enforces single use:
//
//   - unknown / expired token        → ErrNotFound (force re-login)
//   - already-revoked token (reuse)  → revoke the whole lineage, ErrRefreshReuse
//   - live token                     → revoke it, insert the successor (linked via
//     rotated_from), return the new row
//
// The carried-over fields (user, auth_method, claim_groups, claim_admin,
// id_token) come from the consumed row so the caller can mint a matching access
// token. Runs in a single transaction with SELECT ... FOR UPDATE to serialise
// concurrent rotations of the same token.
func (db *DB) RotateRefreshToken(ctx context.Context, oldHash, newHash string, newExpiresAt time.Time) (*RefreshToken, error) {
	ctx, span := startSpan(ctx, "RotateRefreshToken")
	defer span.End()

	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("rotating refresh token: begin: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		id         string
		userID     string
		authMethod string
		groups     []string
		claimAdmin bool
		idToken    *string
		expiresAt  time.Time
		revokedAt  *time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, auth_method, claim_groups, claim_admin, id_token, expires_at, revoked_at
		FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE`, oldHash).
		Scan(&id, &userID, &authMethod, &groups, &claimAdmin, &idToken, &expiresAt, &revokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("rotating refresh token: select: %w", err)
	}

	// Reuse of an already-rotated token: revoke the whole lineage for this user.
	if revokedAt != nil {
		if _, rErr := tx.Exec(ctx,
			`UPDATE refresh_tokens SET revoked_at = now()
			 WHERE user_id = $1 AND revoked_at IS NULL`, userID); rErr != nil {
			recordErr(span, rErr)
			return nil, fmt.Errorf("rotating refresh token: revoke lineage: %w", rErr)
		}
		if cErr := tx.Commit(ctx); cErr != nil {
			recordErr(span, cErr)
			return nil, fmt.Errorf("rotating refresh token: commit reuse: %w", cErr)
		}
		recordErr(span, ErrRefreshReuse)
		return nil, ErrRefreshReuse
	}

	// Expired token: treat as invalid (the housekeeping sweep removes it later).
	if !expiresAt.After(time.Now()) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}

	if _, err = tx.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1`, id); err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("rotating refresh token: revoke old: %w", err)
	}

	newRow, err := scanRefreshToken(tx.QueryRow(ctx, `
		INSERT INTO refresh_tokens
			(id, token_hash, user_id, auth_method, claim_groups, claim_admin, id_token, rotated_from, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING `+refreshColumns,
		NewULID(), newHash, userID, authMethod, groups, claimAdmin, idToken, id, newExpiresAt))
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("rotating refresh token: insert successor: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("rotating refresh token: commit: %w", err)
	}
	return newRow, nil
}

// RevokeRefreshToken marks the live refresh token with the given hash revoked
// (logout). Returns the row (for an OIDC id_token logout hint) or ErrNotFound
// when there is no live token to revoke.
func (db *DB) RevokeRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error) {
	ctx, span := startSpan(ctx, "RevokeRefreshToken")
	defer span.End()

	t, err := scanRefreshToken(db.Pool.QueryRow(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE token_hash = $1 AND revoked_at IS NULL
		 RETURNING `+refreshColumns, tokenHash))
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return nil, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return nil, fmt.Errorf("revoking refresh token: %w", err)
	}
	return t, nil
}

// RevokeAllRefreshTokensForUser revokes every live refresh token for a user
// (account disable / global sign-out). Returns the number revoked.
func (db *DB) RevokeAllRefreshTokensForUser(ctx context.Context, userID string) (int64, error) {
	ctx, span := startSpan(ctx, "RevokeAllRefreshTokensForUser")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = now()
		 WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	if err != nil {
		recordErr(span, err)
		return 0, fmt.Errorf("revoking user refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}

// DeleteExpiredRefreshTokens removes refresh tokens past expiry or revoked.
// Housekeeping; returns the number of rows removed.
func (db *DB) DeleteExpiredRefreshTokens(ctx context.Context) (int64, error) {
	ctx, span := startSpan(ctx, "DeleteExpiredRefreshTokens")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`DELETE FROM refresh_tokens WHERE expires_at <= now() OR revoked_at IS NOT NULL`)
	if err != nil {
		recordErr(span, err)
		return 0, fmt.Errorf("deleting expired refresh tokens: %w", err)
	}
	return tag.RowsAffected(), nil
}
