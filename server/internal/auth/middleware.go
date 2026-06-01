package auth

import (
	"context"
	"net/http"

	"github.com/haibread/ai-registry/internal/problem"
)

type contextKey string

const (
	claimsKey  contextKey = "auth_claims"
	isAdminKey contextKey = "auth_is_admin"
)

// Authenticator resolves the registry session cookie into a Principal and
// populates request context. It replaces the
// old multi-issuer JWT Validator: the registry is now the single token
// authority — OIDC is brokered server-side and both front doors end in a
// session cookie. It never blocks an unauthenticated request; the guards
// (RequireAdmin, RequirePublisherRole) gate writes.
type Authenticator struct {
	sessions *SessionManager
	store    PrincipalStore
}

// NewAuthenticator builds the session authenticator. A nil sessions or store
// (e.g. route-walk tests that construct the router with no DB) makes
// Authenticate a pass-through.
func NewAuthenticator(sessions *SessionManager, store PrincipalStore) *Authenticator {
	return &Authenticator{sessions: sessions, store: store}
}

// Authenticate is chi middleware that resolves the session cookie when present.
// An absent / stale cookie proceeds unauthenticated (the stale cookie is
// cleared); a disabled account is refused with 403 on every request.
func (a *Authenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.sessions == nil || a.store == nil {
			next.ServeHTTP(w, r)
			return
		}
		token := a.sessions.TokenFromRequest(r)
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		sess, err := a.sessions.Resolve(r.Context(), token)
		if err != nil {
			// Expired / revoked / unknown: clear the stale cookie and proceed
			// unauthenticated so the SPA falls back to the signed-out state.
			http.SetCookie(w, a.sessions.ClearCookie())
			next.ServeHTTP(w, r)
			return
		}
		u, err := a.store.GetUserByID(r.Context(), sess.UserID)
		if err != nil {
			http.SetCookie(w, a.sessions.ClearCookie())
			next.ServeHTTP(w, r)
			return
		}
		if u.Disabled {
			problem.Write(w, http.StatusForbidden, "forbidden",
				"This account is disabled.", r.URL.Path)
			return
		}

		princ := &Principal{
			UserID:        u.ID,
			Email:         u.Email,
			ClaimGroups:   sess.ClaimGroups,
			IsServerAdmin: sess.ClaimAdmin || u.IsServerAdmin,
			AuthMethod:    sess.AuthMethod,
		}
		// Synthesize minimal claims so the context helpers that predate the
		// session model (ClaimsFromContext, IsServerAdminFromContext,
		// IdentityFromContext) keep working unchanged.
		claims := &KeycloakClaims{Email: u.Email, Groups: sess.ClaimGroups}
		if princ.IsServerAdmin {
			claims.RealmAccess.Roles = []string{"admin"}
		}
		ctx := context.WithValue(r.Context(), principalKey, princ)
		ctx = context.WithValue(ctx, claimsKey, claims)
		ctx = context.WithValue(ctx, isAdminKey, princ.IsServerAdmin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is chi middleware that gates a route to Server Admins. It must be
// chained after Authenticate. Server Admin is dual-sourced: the
// snapshotted realm-admin claim OR the resolved principal's is_server_admin
// flag, via IsServerAdminFromContext.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			problem.Write(w, http.StatusUnauthorized, "unauthorized",
				"Missing or invalid session", r.URL.Path)
			return
		}
		if !IsServerAdminFromContext(r.Context()) {
			problem.Write(w, http.StatusForbidden, "forbidden",
				"Insufficient permissions: Server Admin required", r.URL.Path)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClaimsFromContext retrieves the synthesized claims for the authenticated
// caller (nil/false when unauthenticated).
func ClaimsFromContext(ctx context.Context) (*KeycloakClaims, bool) {
	c, ok := ctx.Value(claimsKey).(*KeycloakClaims)
	return c, ok
}

// IsAdminFromContext reports whether the current request is authenticated as admin.
func IsAdminFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(isAdminKey).(bool)
	return v
}

// ContextWithClaims injects claims into a context. Used in tests to simulate an
// authenticated caller without a real session.
func ContextWithClaims(ctx context.Context, claims *KeycloakClaims) context.Context {
	ctx = context.WithValue(ctx, claimsKey, claims)
	ctx = context.WithValue(ctx, isAdminKey, claims.IsAdmin())
	return ctx
}
