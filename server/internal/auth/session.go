package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/haibread/ai-registry/internal/store"
)

// SessionStore is the narrow store slice the SessionManager needs (ADR 0006
// amendment, 2026-06-01). *store.DB satisfies it.
type SessionStore interface {
	CreateSession(ctx context.Context, p store.CreateSessionParams) (*store.Session, error)
	ActiveSessionByTokenHash(ctx context.Context, tokenHash string) (*store.Session, error)
	RevokeSession(ctx context.Context, tokenHash string) error
}

// SessionConfig holds the cookie + lifetime settings for registry sessions.
// Built from config.AuthConfig in main so the auth package stays decoupled from
// the config loader.
type SessionConfig struct {
	CookieName string
	TTL        time.Duration
	Secure     bool
	SameSite   http.SameSite
}

// ParseSameSite maps a config string ("lax" / "strict" / "none") to
// http.SameSite, defaulting to Lax for empty / unrecognised values.
func ParseSameSite(s string) http.SameSite {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// SessionManager issues, resolves, and revokes registry sessions, and builds
// the HttpOnly cookie that carries the opaque session token. The token is a
// 256-bit random string; only its SHA-256 hash is persisted, so a DB leak
// yields no usable session.
type SessionManager struct {
	store SessionStore
	cfg   SessionConfig
}

// NewSessionManager builds a SessionManager, filling in safe defaults for an
// empty cookie name or non-positive TTL.
func NewSessionManager(s SessionStore, cfg SessionConfig) *SessionManager {
	if cfg.CookieName == "" {
		cfg.CookieName = "ai_registry_session"
	}
	if cfg.TTL <= 0 {
		cfg.TTL = 24 * time.Hour
	}
	return &SessionManager{store: s, cfg: cfg}
}

// CookieName is the name of the session cookie this manager reads and writes.
func (m *SessionManager) CookieName() string { return m.cfg.CookieName }

// CookieSecure reports whether issued cookies carry the Secure flag. The OIDC
// handlers reuse it for the short-lived login-transaction cookie.
func (m *SessionManager) CookieSecure() bool { return m.cfg.Secure }

// hashToken returns the hex SHA-256 of a session token — the value persisted
// and looked up, never the raw token.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// newToken returns a 256-bit URL-safe random session token.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating session token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// IssueParams describes a session to open after a successful login. ClaimGroups
// and ClaimAdmin snapshot the OIDC claim inputs (empty / false for local
// logins).
type IssueParams struct {
	UserID      string
	AuthMethod  string // "oidc" | "local"
	ClaimGroups []string
	ClaimAdmin  bool
	// IDToken is the raw OIDC id_token, stored as a logout hint for RP-initiated
	// logout (empty for local logins). See store.Session.IDToken.
	IDToken string
}

// Issue creates a session row and returns the raw cookie token — the only point
// at which it exists in plaintext. Callers set it on the response via Cookie().
func (m *SessionManager) Issue(ctx context.Context, p IssueParams) (string, error) {
	token, err := newToken()
	if err != nil {
		return "", err
	}
	if _, err := m.store.CreateSession(ctx, store.CreateSessionParams{
		UserID:      p.UserID,
		TokenHash:   hashToken(token),
		AuthMethod:  p.AuthMethod,
		ClaimGroups: p.ClaimGroups,
		ClaimAdmin:  p.ClaimAdmin,
		IDToken:     p.IDToken,
		ExpiresAt:   time.Now().Add(m.cfg.TTL),
	}); err != nil {
		return "", err
	}
	return token, nil
}

// Resolve returns the live session for a raw cookie token, or store.ErrNotFound
// when none is active (expired, revoked, or unknown).
func (m *SessionManager) Resolve(ctx context.Context, token string) (*store.Session, error) {
	return m.store.ActiveSessionByTokenHash(ctx, hashToken(token))
}

// Revoke ends the session for a raw cookie token (logout). Returns
// store.ErrNotFound when there is no live session.
func (m *SessionManager) Revoke(ctx context.Context, token string) error {
	return m.store.RevokeSession(ctx, hashToken(token))
}

// Cookie builds the Set-Cookie carrying the session token. Path=/ so it rides
// every request; HttpOnly so JS cannot read it; Secure / SameSite per config.
func (m *SessionManager) Cookie(token string) *http.Cookie {
	return &http.Cookie{
		Name:     m.cfg.CookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.cfg.Secure,
		SameSite: m.cfg.SameSite,
		Expires:  time.Now().Add(m.cfg.TTL),
		MaxAge:   int(m.cfg.TTL.Seconds()),
	}
}

// ClearCookie builds a Set-Cookie that immediately expires the session cookie
// (logout / 401).
func (m *SessionManager) ClearCookie() *http.Cookie {
	return &http.Cookie{
		Name:     m.cfg.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.cfg.Secure,
		SameSite: m.cfg.SameSite,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	}
}

// TokenFromRequest returns the raw session token from the request's session
// cookie, or "" when absent.
func (m *SessionManager) TokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(m.cfg.CookieName)
	if err != nil {
		return ""
	}
	return c.Value
}
