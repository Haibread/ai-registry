package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/haibread/ai-registry/internal/problem"
)

type contextKey string

const (
	claimsKey  contextKey = "auth_claims"
	isAdminKey contextKey = "auth_is_admin"
)

// Authenticator validates the registry-issued access token (a bearer JWT) and
// populates the request context with the resolved Principal. The registry is the
// single token authority: there is no cookie and no multi-issuer validation, and
// verification is pure crypto (no DB) so it sits on the hot read path. It never
// blocks an unauthenticated or stale-token request; the guards (RequireAdmin,
// RequirePublisherRole) gate writes.
type Authenticator struct {
	tokens *TokenAuthority
}

// NewAuthenticator builds the bearer authenticator. A nil TokenAuthority (e.g.
// route-walk tests that construct the router with no signing key) makes
// Authenticate a pass-through.
func NewAuthenticator(tokens *TokenAuthority) *Authenticator {
	return &Authenticator{tokens: tokens}
}

// Authenticate is chi middleware that resolves a valid bearer token into a
// Principal. A missing, malformed, expired, or otherwise invalid token proceeds
// unauthenticated (public reads still work; the SPA refreshes on the resulting
// 401 from a guarded route).
func (a *Authenticator) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.tokens == nil {
			next.ServeHTTP(w, r)
			return
		}
		raw := bearerToken(r)
		if raw == "" {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := a.tokens.Verify(raw)
		if err != nil {
			// Invalid / expired token: proceed unauthenticated rather than 401,
			// so the contract ("Authenticate never blocks") holds and the guards
			// produce the 401 that drives the SPA's silent refresh.
			next.ServeHTTP(w, r)
			return
		}

		princ := &Principal{
			UserID:        claims.Subject,
			Email:         claims.Email,
			ClaimGroups:   claims.Groups,
			IsServerAdmin: claims.SrvAdmin,
			AuthMethod:    claims.AuthMethod,
		}
		// Synthesize minimal claims so the context helpers that predate the token
		// model (ClaimsFromContext, IsServerAdminFromContext, IdentityFromContext)
		// keep working unchanged.
		kc := &KeycloakClaims{Email: claims.Email, Groups: claims.Groups}
		if princ.IsServerAdmin {
			kc.RealmAccess.Roles = []string{"admin"}
		}
		ctx := context.WithValue(r.Context(), principalKey, princ)
		ctx = context.WithValue(ctx, claimsKey, kc)
		ctx = context.WithValue(ctx, isAdminKey, princ.IsServerAdmin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// bearerToken extracts the token from an `Authorization: Bearer <token>` header,
// or "" when absent / malformed.
func bearerToken(r *http.Request) string {
	const prefix = "Bearer "
	h := r.Header.Get("Authorization")
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

// RequireAdmin is chi middleware that gates a route to Server Admins. It must be
// chained after Authenticate. Server Admin is dual-sourced: the snapshotted
// claim flag OR the local is_server_admin flag (baked into the token at login),
// via IsServerAdminFromContext.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			problem.Write(w, http.StatusUnauthorized, "unauthorized",
				"Missing or invalid bearer token", r.URL.Path)
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
// authenticated caller without a real token.
func ContextWithClaims(ctx context.Context, claims *KeycloakClaims) context.Context {
	ctx = context.WithValue(ctx, claimsKey, claims)
	ctx = context.WithValue(ctx, isAdminKey, claims.IsAdmin())
	return ctx
}
