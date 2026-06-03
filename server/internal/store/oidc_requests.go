package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// ── OIDC login-transaction state ─────────────────────────────────────────────
// Replaces the former HttpOnly transaction cookie. The browser only ever sees
// the opaque `state`; its hash is the lookup key, and the row is consumed
// (deleted) by the callback.

// CreateOIDCAuthRequestParams holds the login-transaction state. StateHash is
// the hex SHA-256 of the `state` value, computed by the auth layer.
type CreateOIDCAuthRequestParams struct {
	StateHash    string
	Nonce        string
	CodeVerifier string
	ExpiresAt    time.Time
}

// CreateOIDCAuthRequest persists a login transaction.
func (db *DB) CreateOIDCAuthRequest(ctx context.Context, p CreateOIDCAuthRequestParams) error {
	ctx, span := startSpan(ctx, "CreateOIDCAuthRequest")
	defer span.End()

	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO oidc_auth_requests (state_hash, nonce, code_verifier, expires_at)
		VALUES ($1, $2, $3, $4)`,
		p.StateHash, p.Nonce, p.CodeVerifier, p.ExpiresAt); err != nil {
		recordErr(span, err)
		return fmt.Errorf("creating oidc auth request: %w", err)
	}
	return nil
}

// ConsumeOIDCAuthRequest deletes and returns the login transaction for a state
// hash, atomically (single use). Returns ErrNotFound when no live (unexpired)
// transaction matches.
func (db *DB) ConsumeOIDCAuthRequest(ctx context.Context, stateHash string) (nonce, codeVerifier string, err error) {
	ctx, span := startSpan(ctx, "ConsumeOIDCAuthRequest")
	defer span.End()

	err = db.Pool.QueryRow(ctx, `
		DELETE FROM oidc_auth_requests
		WHERE state_hash = $1 AND expires_at > now()
		RETURNING nonce, code_verifier`, stateHash).Scan(&nonce, &codeVerifier)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return "", "", ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return "", "", fmt.Errorf("consuming oidc auth request: %w", err)
	}
	return nonce, codeVerifier, nil
}

// DeleteExpiredOIDCAuthRequests removes abandoned login transactions.
func (db *DB) DeleteExpiredOIDCAuthRequests(ctx context.Context) (int64, error) {
	ctx, span := startSpan(ctx, "DeleteExpiredOIDCAuthRequests")
	defer span.End()

	tag, err := db.Pool.Exec(ctx, `DELETE FROM oidc_auth_requests WHERE expires_at <= now()`)
	if err != nil {
		recordErr(span, err)
		return 0, fmt.Errorf("deleting expired oidc auth requests: %w", err)
	}
	return tag.RowsAffected(), nil
}

// ── One-time token handoff codes ─────────────────────────────────────────────
// Deliver freshly minted tokens to the SPA after the brokered OIDC redirect
// without ever putting them in a URL. Single-use, very short TTL.

// CreateHandoffCodeParams holds a one-time code and the tokens it hands off.
// CodeHash is the hex SHA-256 of the raw code, computed by the auth layer.
type CreateHandoffCodeParams struct {
	CodeHash     string
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	ExpiresAt    time.Time
}

// CreateHandoffCode persists a one-time handoff code.
func (db *DB) CreateHandoffCode(ctx context.Context, p CreateHandoffCodeParams) error {
	ctx, span := startSpan(ctx, "CreateHandoffCode")
	defer span.End()

	if _, err := db.Pool.Exec(ctx, `
		INSERT INTO auth_handoff_codes (code_hash, access_token, refresh_token, expires_in, expires_at)
		VALUES ($1, $2, $3, $4, $5)`,
		p.CodeHash, p.AccessToken, p.RefreshToken, p.ExpiresIn, p.ExpiresAt); err != nil {
		recordErr(span, err)
		return fmt.Errorf("creating handoff code: %w", err)
	}
	return nil
}

// ConsumeHandoffCode atomically marks a handoff code consumed and returns its
// tokens (single use). Returns ErrNotFound when no live, unconsumed code
// matches.
func (db *DB) ConsumeHandoffCode(ctx context.Context, codeHash string) (accessToken, refreshToken string, expiresIn int, err error) {
	ctx, span := startSpan(ctx, "ConsumeHandoffCode")
	defer span.End()

	err = db.Pool.QueryRow(ctx, `
		UPDATE auth_handoff_codes SET consumed_at = now()
		WHERE code_hash = $1 AND consumed_at IS NULL AND expires_at > now()
		RETURNING access_token, refresh_token, expires_in`, codeHash).
		Scan(&accessToken, &refreshToken, &expiresIn)
	if errors.Is(err, pgx.ErrNoRows) {
		recordErr(span, ErrNotFound)
		return "", "", 0, ErrNotFound
	}
	if err != nil {
		recordErr(span, err)
		return "", "", 0, fmt.Errorf("consuming handoff code: %w", err)
	}
	return accessToken, refreshToken, expiresIn, nil
}

// DeleteExpiredHandoffCodes removes expired or consumed handoff codes.
func (db *DB) DeleteExpiredHandoffCodes(ctx context.Context) (int64, error) {
	ctx, span := startSpan(ctx, "DeleteExpiredHandoffCodes")
	defer span.End()

	tag, err := db.Pool.Exec(ctx,
		`DELETE FROM auth_handoff_codes WHERE expires_at <= now() OR consumed_at IS NOT NULL`)
	if err != nil {
		recordErr(span, err)
		return 0, fmt.Errorf("deleting expired handoff codes: %w", err)
	}
	return tag.RowsAffected(), nil
}
