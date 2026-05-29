package auth

import (
	"context"
	"errors"

	"github.com/haibread/ai-registry/internal/store"
)

// IssuerKind records which front door authenticated a request: a federated
// OIDC token or a registry-issued local token. The MCP wall (ADR 0006 §3)
// keys on this to reject local tokens on the OAuth-only protocol surface.
type IssuerKind string

const (
	IssuerOIDC  IssuerKind = "oidc"
	IssuerLocal IssuerKind = "local"
)

// Principal is the resolved caller used for publisher-scoped authorization.
// It is populated by Authenticate when a PrincipalStore is configured. UserID
// is the registry users.id — the principal key, auth-method-agnostic.
// ClaimGroups are the verbatim OIDC claim group names (empty for local tokens,
// whose group membership comes from group_members in the DB). IsServerAdmin is
// true when the OIDC claim carries realm admin OR users.is_server_admin is set.
type Principal struct {
	UserID        string
	Email         string
	ClaimGroups   []string
	IsServerAdmin bool
	Issuer        IssuerKind
}

// PrincipalStore is the narrow slice of the store that Authenticate needs to
// resolve a token into a Principal and to just-in-time provision federated
// users. *store.DB satisfies it.
type PrincipalStore interface {
	GetUserByID(ctx context.Context, id string) (*store.User, error)
	GetUserBySubject(ctx context.Context, subject string) (*store.User, error)
	GetUserByEmail(ctx context.Context, email string) (*store.User, error)
	CreateUser(ctx context.Context, p store.CreateUserParams) (*store.User, error)
	BindSubject(ctx context.Context, userID, subject string) error
}

// errAccountDisabled and errPrincipalUnresolved are internal sentinels that map
// to authentication failures. They are not exported — callers see an HTTP 403
// (disabled) or 401 (unresolved) via the middleware.
var (
	errAccountDisabled     = errors.New("account disabled")
	errPrincipalUnresolved = errors.New("principal could not be resolved")
	// errNoEmail is returned when a federated token lacks an email, so no
	// users row (email is NOT NULL) can be provisioned.
	errNoEmail = errors.New("token carries no email; cannot provision user")
)

const principalKey contextKey = "auth_principal"

// PrincipalFromContext returns the resolved principal, if Authenticate stored
// one (only when a PrincipalStore is configured).
func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey).(*Principal)
	return p, ok
}

// ContextWithPrincipal injects a principal into a context. Used in tests to
// simulate an authenticated, resolved caller without a real token.
func ContextWithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// IsServerAdminFromContext reports whether the caller is a Server Admin, from
// either source: the OIDC realm-admin claim or the resolved principal's
// is_server_admin flag (the local bootstrap-admin path). Falls back to the
// claim alone when no principal was resolved, preserving the pre-ADR-0006
// behaviour for callers that only set claims.
func IsServerAdminFromContext(ctx context.Context) bool {
	if p, ok := PrincipalFromContext(ctx); ok && p != nil && p.IsServerAdmin {
		return true
	}
	if c, ok := ClaimsFromContext(ctx); ok && c != nil && c.IsAdmin() {
		return true
	}
	return false
}
