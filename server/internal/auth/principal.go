package auth

import (
	"context"
	"errors"

	"github.com/haibread/ai-registry/internal/store"
)

// Principal is the resolved caller used for publisher-scoped authorization. It
// is populated by the Authenticator from the request's session. UserID is the
// registry users.id — the principal key. ClaimGroups
// are the OIDC claim group slugs snapshotted into the session at login (empty
// for local logins, whose group membership comes from group_members in the DB).
// IsServerAdmin is true when the snapshotted claim carried realm admin OR
// users.is_server_admin is set. AuthMethod records which front door minted the
// session ("oidc" | "local").
type Principal struct {
	UserID        string
	Email         string
	ClaimGroups   []string
	IsServerAdmin bool
	AuthMethod    string
}

// PrincipalStore is the narrow slice of the store the Authenticator and the
// OIDC callback need: resolve a session/identity into a users row and
// just-in-time provision federated users. *store.DB satisfies it.
type PrincipalStore interface {
	GetUserByID(ctx context.Context, id string) (*store.User, error)
	GetUserBySubject(ctx context.Context, subject string) (*store.User, error)
	GetUserByEmail(ctx context.Context, email string) (*store.User, error)
	CreateUser(ctx context.Context, p store.CreateUserParams) (*store.User, error)
	BindSubject(ctx context.Context, userID, subject string) error
}

// Internal sentinels for the federated provisioning path. Not exported; the
// OIDC callback maps a failure to an HTTP error.
var (
	errPrincipalUnresolved = errors.New("principal could not be resolved")
	errNoEmail             = errors.New("token carries no email; cannot provision user")
)

const principalKey contextKey = "auth_principal"

// PrincipalFromContext returns the resolved principal, if Authenticate stored one.
func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	p, ok := ctx.Value(principalKey).(*Principal)
	return p, ok
}

// ContextWithPrincipal injects a principal into a context. Used in tests to
// simulate an authenticated, resolved caller without a real session.
func ContextWithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey, p)
}

// IsServerAdminFromContext reports whether the caller is a Server Admin, from
// either source: the snapshotted realm-admin claim or the resolved principal's
// is_server_admin flag (the local bootstrap-admin path).
func IsServerAdminFromContext(ctx context.Context) bool {
	if p, ok := PrincipalFromContext(ctx); ok && p != nil && p.IsServerAdmin {
		return true
	}
	if c, ok := ClaimsFromContext(ctx); ok && c != nil && c.IsAdmin() {
		return true
	}
	return false
}
