package auth

import (
	"context"
	"time"

	"github.com/haibread/ai-registry/internal/store"
)

// RefreshStore is the narrow store slice the RefreshManager needs.
// *store.DB satisfies it.
type RefreshStore interface {
	CreateRefreshToken(ctx context.Context, p store.CreateRefreshTokenParams) (*store.RefreshToken, error)
	RotateRefreshToken(ctx context.Context, oldHash, newHash string, newExpiresAt time.Time) (*store.RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) (*store.RefreshToken, error)
}

// RefreshManager issues, rotates, and revokes refresh tokens. The raw token is
// returned only at mint/rotate time; only its hash is persisted.
type RefreshManager struct {
	store RefreshStore
	ttl   time.Duration
}

// NewRefreshManager builds a RefreshManager, defaulting a non-positive TTL to
// 30 days.
func NewRefreshManager(s RefreshStore, ttl time.Duration) *RefreshManager {
	if ttl <= 0 {
		ttl = 720 * time.Hour
	}
	return &RefreshManager{store: s, ttl: ttl}
}

// RefreshIssueParams describes a refresh token to mint after a successful login.
type RefreshIssueParams struct {
	UserID      string
	AuthMethod  string // "oidc" | "local"
	ClaimGroups []string
	ClaimAdmin  bool
	// IDToken is the raw OIDC id_token, kept as an RP-initiated-logout hint
	// (empty for local logins).
	IDToken string
}

// Issue mints a refresh token and returns the raw value (the only point at which
// it exists in plaintext).
func (m *RefreshManager) Issue(ctx context.Context, p RefreshIssueParams) (string, error) {
	raw, err := randomToken()
	if err != nil {
		return "", err
	}
	if _, err := m.store.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		UserID:      p.UserID,
		TokenHash:   hashToken(raw),
		AuthMethod:  p.AuthMethod,
		ClaimGroups: p.ClaimGroups,
		ClaimAdmin:  p.ClaimAdmin,
		IDToken:     p.IDToken,
		ExpiresAt:   time.Now().Add(m.ttl),
	}); err != nil {
		return "", err
	}
	return raw, nil
}

// Rotate consumes raw and returns a successor raw token plus the carried-over
// row (whose user/groups/admin the caller uses to mint a matching access token).
// Maps store sentinels: store.ErrNotFound (unknown/expired) and
// store.ErrRefreshReuse (theft — the whole lineage was revoked).
func (m *RefreshManager) Rotate(ctx context.Context, raw string) (newRaw string, row *store.RefreshToken, err error) {
	newRaw, err = randomToken()
	if err != nil {
		return "", nil, err
	}
	row, err = m.store.RotateRefreshToken(ctx, hashToken(raw), hashToken(newRaw), time.Now().Add(m.ttl))
	if err != nil {
		return "", nil, err
	}
	return newRaw, row, nil
}

// Revoke ends the refresh token for raw (logout). Returns the revoked row (for
// an OIDC id_token logout hint) or store.ErrNotFound when none is live.
func (m *RefreshManager) Revoke(ctx context.Context, raw string) (*store.RefreshToken, error) {
	return m.store.RevokeRefreshToken(ctx, hashToken(raw))
}
